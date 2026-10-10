// Package base64dq implements a base64 encoding variant that is inspired by the Revival Password of Dragon Quest.
//
// The Revival Password (ふっかつのじゅもん) is a string of 20 characters that is used to revive a player's party in the [Dragon Quest] series.
// It is encoded in a custom base64 variant that uses 64 characters from the Japanese hiragana syllabary.
//
// [Dragon Quest]: https://www.dragonquest.jp/
package base64dq

import (
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	rootNode    = -1
	midNode     = -2
	paddingNode = 64
)

// node is a node in a DFA (Deterministic Finite State Machine).
type node struct {
	v        int
	children []*node
}

func buildDFA(entries [64]string, padding rune) *node {
	root := &node{
		v:        rootNode,
		children: make([]*node, 256),
	}
	for i, entry := range entries {
		n := root
		for _, b := range []byte(entry[:len(entry)-1]) {
			if n.children[b] == nil {
				n.children[b] = &node{
					v:        midNode,
					children: make([]*node, 256),
				}
			}
			n = n.children[b]
		}
		n.children[entry[len(entry)-1]] = &node{
			v:        i,
			children: root.children,
		}
	}

	if padding != NoPadding {
		pad := &node{
			v:        paddingNode,
			children: make([]*node, 256),
		}
		pad.children['\n'] = &node{
			v:        rootNode,
			children: pad.children,
		}
		pad.children['\r'] = &node{
			v:        rootNode,
			children: pad.children,
		}

		var buf [4]byte
		l := utf8.EncodeRune(buf[:], padding)
		n, m := root, pad
		for _, b := range buf[:l-1] {
			if n.children[b] == nil {
				n.children[b] = &node{
					v:        -1,
					children: make([]*node, 256),
				}
			}
			if m.children[b] == nil {
				m.children[b] = &node{
					v:        -1,
					children: make([]*node, 256),
				}
			}
			n = n.children[b]
			m = m.children[b]
		}
		n.children[buf[l-1]] = pad
		m.children[buf[l-1]] = pad
	}

	root.children['\n'] = root
	root.children['\r'] = root
	return root
}

type Encoding struct {
	once sync.Once // guards root and fast
	root *node
	fast *fastDecoder

	encode    [64]string
	encodeBuf [64]uint32 // encode[i] packed in little endian
	encodeLen [64]uint8  // len(encode[i])
	maxSize   int        // maximum number of bytes per rune
	padChar   rune
	strict    bool
}

// Strict creates a new encoding identical to enc except with
// strict decoding enabled. In this mode, the decoder requires that
// trailing padding bits are zero.
//
// Note that the input is still malleable, as new line characters
// (CR and LF) are still ignored.
func (enc *Encoding) Strict() *Encoding {
	return &Encoding{
		encode:    enc.encode,
		encodeBuf: enc.encodeBuf,
		encodeLen: enc.encodeLen,
		maxSize:   enc.maxSize,
		padChar:   enc.padChar,
		strict:    true,
	}
}

const encodeStd = "あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれろわがぎぐげござじずぜぞだぢづでどばびぶべぼ"
const encodeName = "０１２３４５６７８９あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれろわをんっゃゅょ゛゜ー　"

const (
	StdPadding rune = '・' // Standard padding character
	NoPadding  rune = -1  // No padding
)

// NewEncoding returns a new padded Encoding defined by the given alphabet.
func NewEncoding(encoder string) *Encoding {
	e := &Encoding{
		padChar: StdPadding,
		maxSize: 1,
	}

	var pos [65]int
	j := 0
	for i, ch := range encoder {
		if j >= 64 {
			panic("encoding alphabet is not 64-runes long")
		}
		if ch == utf8.RuneError {
			panic("encoding alphabet contains invalid UTF-8 sequence")
		}
		pos[j] = i
		j++
	}
	pos[64] = len(encoder)

	for i := 0; i < 64; i++ {
		e.encode[i] = encoder[pos[i]:pos[i+1]]
		var buf [4]byte
		copy(buf[:], e.encode[i])
		e.encodeBuf[i] = binary.LittleEndian.Uint32(buf[:])
		e.encodeLen[i] = uint8(len(e.encode[i]))
		if size := pos[i+1] - pos[i]; size > e.maxSize {
			e.maxSize = size
		}
	}
	if size := utf8.RuneLen(e.padChar); size > e.maxSize {
		e.maxSize = size
	}

	return e
}

func (enc *Encoding) buildOnce() {
	enc.once.Do(enc.build)
}

func (enc *Encoding) build() {
	enc.root = buildDFA(enc.encode, enc.padChar)
	enc.fast = newFastDecoder(enc.encode)
}

// fastDecoder decodes complete quanta that contain no new lines and no padding.
// It is available only if all runes in the alphabet have the same length of 1 or 3 bytes.
type fastDecoder struct {
	size int // the length of each rune in bytes

	// for size == 1: the value of the rune, 0xFF if invalid.
	decode1 [256]uint8

	// for size == 3: the rune b0 b1 b2 is decoded as
	// pages[index[(b0&0x0F)<<6|(b1&0x3F)]<<6|(b2&0x3F)].
	// page 0 is filled with 0xFF (invalid).
	index [1024]uint8
	pages []uint8
}

func newFastDecoder(entries [64]string) *fastDecoder {
	size := len(entries[0])
	for _, entry := range entries {
		if len(entry) != size {
			return nil
		}
	}

	switch size {
	case 1:
		f := &fastDecoder{size: 1}
		for i := range f.decode1 {
			f.decode1[i] = 0xFF
		}
		for i, entry := range entries {
			if entry[0] == '\n' || entry[0] == '\r' {
				// new lines are ignored by the DFA, leave them to the slow path.
				continue
			}
			f.decode1[entry[0]] = uint8(i)
		}
		return f
	case 3:
		f := &fastDecoder{size: 3}
		f.pages = make([]uint8, 64, 2*64)
		for i := range f.pages {
			f.pages[i] = 0xFF
		}
		for i, entry := range entries {
			// all runes are 3-byte UTF-8 sequences, so b0 is 1110xxxx and b1, b2 are 10xxxxxx.
			key := uint(entry[0]&0x0F)<<6 | uint(entry[1]&0x3F)
			if f.index[key] == 0 {
				f.index[key] = uint8(len(f.pages) >> 6)
				f.pages = append(f.pages, f.pages[:64]...)
			}
			f.pages[uint(f.index[key])<<6|uint(entry[2]&0x3F)] = uint8(i)
		}
		return f
	}
	return nil
}

// decode decodes as many complete quanta from src as possible.
// It stops at the first quantum that contains anything other than the alphabet
// (e.g. new lines, padding, and invalid bytes), and leaves it to the slow path.
// The last quantum of src is always left to the slow path, because it is likely to contain padding.
// It returns the number of bytes consumed from src and written to dst.
func (f *fastDecoder) decode(dst, src []byte) (si, di int) {
	if f == nil {
		return 0, 0
	}

	switch f.size {
	case 1:
		for len(src)-si > 4 && len(dst)-di >= 3 {
			s := src[si : si+4]
			v0 := uint(f.decode1[s[0]])
			v1 := uint(f.decode1[s[1]])
			v2 := uint(f.decode1[s[2]])
			v3 := uint(f.decode1[s[3]])
			if (v0|v1|v2|v3)&0xC0 != 0 {
				break
			}
			val := v0<<18 | v1<<12 | v2<<6 | v3
			d := dst[di : di+3]
			d[0] = byte(val >> 16)
			d[1] = byte(val >> 8)
			d[2] = byte(val)
			si += 4
			di += 3
		}
	case 3:
		pages := f.pages
		for len(src)-si > 12 && len(dst)-di >= 3 {
			s := src[si : si+12]

			// check that all runes are 3-byte UTF-8 sequences.
			var bad byte
			for i := 0; i < 12; i += 3 {
				bad |= (s[i] & 0xF0) ^ 0xE0
				bad |= (s[i+1] & 0xC0) ^ 0x80
				bad |= (s[i+2] & 0xC0) ^ 0x80
			}
			if bad != 0 {
				break
			}

			v0 := uint(pages[uint(f.index[uint(s[0]&0x0F)<<6|uint(s[1]&0x3F)])<<6|uint(s[2]&0x3F)])
			v1 := uint(pages[uint(f.index[uint(s[3]&0x0F)<<6|uint(s[4]&0x3F)])<<6|uint(s[5]&0x3F)])
			v2 := uint(pages[uint(f.index[uint(s[6]&0x0F)<<6|uint(s[7]&0x3F)])<<6|uint(s[8]&0x3F)])
			v3 := uint(pages[uint(f.index[uint(s[9]&0x0F)<<6|uint(s[10]&0x3F)])<<6|uint(s[11]&0x3F)])
			if (v0|v1|v2|v3)&0xC0 != 0 {
				break
			}
			val := v0<<18 | v1<<12 | v2<<6 | v3
			d := dst[di : di+3]
			d[0] = byte(val >> 16)
			d[1] = byte(val >> 8)
			d[2] = byte(val)
			si += 12
			di += 3
		}
	}
	return si, di
}

// WithPadding creates a new encoding identical to enc except
// with a specified padding character, or NoPadding to disable padding.
// The padding character must not be '\r' or '\n', must not
// be contained in the encoding's alphabet.
func (enc *Encoding) WithPadding(padding rune) *Encoding {
	if padding == '\r' || padding == '\n' {
		panic("invalid padding")
	}

	for _, s := range enc.encode {
		r, _ := utf8.DecodeRuneInString(s)
		if r == padding {
			panic("padding contained in alphabet")
		}
	}

	maxSize := enc.maxSize
	size := utf8.RuneLen(padding)
	if size > maxSize {
		maxSize = size
	}

	return &Encoding{
		encode:    enc.encode,
		encodeBuf: enc.encodeBuf,
		encodeLen: enc.encodeLen,
		maxSize:   maxSize,
		padChar:   padding,
		strict:    enc.strict,
	}
}

// StdEncoding is a base64 encoding used in Revival Password.
var StdEncoding = NewEncoding(encodeStd)

// NameEncoding is a base64 encoding used in encoding a user name.
var NameEncoding = NewEncoding(encodeName)

// RawStdEncoding is the standard raw, unpadded base64 encoding.
var RawStdEncoding = StdEncoding.WithPadding(NoPadding)

// RawNameEncoding is the name raw, unpadded base64 encoding.
var RawNameEncoding = NameEncoding.WithPadding(NoPadding)

func (enc *Encoding) Encode(dst, src []byte) int {
	if len(src) == 0 {
		return 0
	}

	di, si := 0, 0
	n := (len(src) / 3) * 3

	// Fast path: write each rune as a 4-byte store while dst has enough room.
	// A store may write up to 3 bytes past the rune, but they are overwritten by the next rune.
	for si+3 < n && len(dst)-di >= 16 {
		val := uint(src[si+0])<<16 | uint(src[si+1])<<8 | uint(src[si+2])
		c0, c1, c2, c3 := val>>18&0x3F, val>>12&0x3F, val>>6&0x3F, val&0x3F
		binary.LittleEndian.PutUint32(dst[di:], enc.encodeBuf[c0])
		di += int(enc.encodeLen[c0])
		binary.LittleEndian.PutUint32(dst[di:], enc.encodeBuf[c1])
		di += int(enc.encodeLen[c1])
		binary.LittleEndian.PutUint32(dst[di:], enc.encodeBuf[c2])
		di += int(enc.encodeLen[c2])
		binary.LittleEndian.PutUint32(dst[di:], enc.encodeBuf[c3])
		di += int(enc.encodeLen[c3])
		si += 3
	}
	if si+3 == n {
		// The last quantum: encode it into a temporary buffer,
		// and copy exactly the output so that nothing is written past it.
		var buf [16]byte
		val := uint(src[si+0])<<16 | uint(src[si+1])<<8 | uint(src[si+2])
		c0, c1, c2, c3 := val>>18&0x3F, val>>12&0x3F, val>>6&0x3F, val&0x3F
		m := 0
		binary.LittleEndian.PutUint32(buf[m:], enc.encodeBuf[c0])
		m += int(enc.encodeLen[c0])
		binary.LittleEndian.PutUint32(buf[m:], enc.encodeBuf[c1])
		m += int(enc.encodeLen[c1])
		binary.LittleEndian.PutUint32(buf[m:], enc.encodeBuf[c2])
		m += int(enc.encodeLen[c2])
		binary.LittleEndian.PutUint32(buf[m:], enc.encodeBuf[c3])
		m += int(enc.encodeLen[c3])
		di += copy(dst[di:], buf[:m])
		si += 3
	}

	for si < n {
		val := uint(src[si+0])<<16 | uint(src[si+1])<<8 | uint(src[si+2])
		di += copy(dst[di:], enc.encode[val>>18&0x3F])
		di += copy(dst[di:], enc.encode[val>>12&0x3F])
		di += copy(dst[di:], enc.encode[val>>6&0x3F])
		di += copy(dst[di:], enc.encode[val&0x3F])
		si += 3
	}

	remain := len(src) - si
	if remain == 0 {
		return di
	}

	// Add the remaining small block
	val := uint(src[si+0]) << 16
	if remain == 2 {
		val |= uint(src[si+1]) << 8
	}
	di += copy(dst[di:], enc.encode[val>>18&0x3F])
	di += copy(dst[di:], enc.encode[val>>12&0x3F])

	switch remain {
	case 2:
		di += copy(dst[di:], enc.encode[val>>6&0x3F])
		if enc.padChar != NoPadding {
			di += utf8.EncodeRune(dst[di:], enc.padChar)
		}
	case 1:
		if enc.padChar != NoPadding {
			di += utf8.EncodeRune(dst[di:], enc.padChar)
			di += utf8.EncodeRune(dst[di:], enc.padChar)
		}
	}
	return di
}

func (enc *Encoding) EncodeToString(src []byte) string {
	var sb strings.Builder
	sb.Grow(enc.EncodedLen(len(src)))

	// Encode src in chunks into a small buffer on the stack,
	// and append them to sb. sb.String() doesn't copy the result.
	var buf [256]byte
	chunk := len(buf) / enc.maxSize / 4 * 3
	for len(src) > chunk {
		n := enc.Encode(buf[:], src[:chunk])
		sb.Write(buf[:n])
		src = src[chunk:]
	}
	n := enc.Encode(buf[:], src)
	sb.Write(buf[:n])
	return sb.String()
}

// EncodedLen returns the length in bytes of the base64 encoding
// of an input buffer of length n.
func (enc *Encoding) EncodedLen(n int) int {
	var ret int
	if enc.padChar == NoPadding {
		ret = (n*8 + 5) / 6 // minimum # chars at 6 bits per char
	} else {
		ret = (n + 2) / 3 * 4 // minimum # 4-char quanta, 3 bytes each
	}
	return ret * enc.maxSize // maximum # bytes: utf8.UTFMax bytes per char
}

type encoder struct {
	err  error
	enc  *Encoding
	w    io.Writer
	buf  [3]byte    // buffered data waiting to be encoded
	nbuf int        // number of bytes in buf
	out  [1024]byte // output buffer
}

func (e *encoder) Write(p []byte) (n int, err error) {
	if e.err != nil {
		return 0, e.err
	}

	// Leading fringe.
	if e.nbuf > 0 {
		var i int
		for i = 0; i < len(p) && e.nbuf < 3; i++ {
			e.buf[e.nbuf] = p[i]
			e.nbuf++
		}
		n += i
		p = p[i:]
		if e.nbuf < 3 {
			return
		}
		size := e.enc.Encode(e.out[:], e.buf[:])
		if _, e.err = e.w.Write(e.out[:size]); e.err != nil {
			return n, e.err
		}
		e.nbuf = 0
	}

	// Large interior chunks.
	for len(p) >= 3 {
		nn := len(e.out) / e.enc.maxSize / 4 * 3
		if nn > len(p) {
			nn = len(p)
			nn -= nn % 3
		}
		size := e.enc.Encode(e.out[:], p[:nn])
		if _, e.err = e.w.Write(e.out[:size]); e.err != nil {
			return n, e.err
		}
		n += nn
		p = p[nn:]
	}

	// Trailing fringe.
	copy(e.buf[:], p)
	e.nbuf = len(p)
	n += len(p)
	return n, nil
}

// Close flushes any pending output from the encoder.
// It is an error to call Write after calling Close.
func (e *encoder) Close() error {
	// If there's anything left in the buffer, flush it out
	if e.err == nil && e.nbuf > 0 {
		size := e.enc.Encode(e.out[:], e.buf[:e.nbuf])
		_, e.err = e.w.Write(e.out[:size])
		e.nbuf = 0
	}
	return e.err
}

// NewEncoder returns a new base64 stream encoder.
func NewEncoder(enc *Encoding, w io.Writer) io.WriteCloser {
	return &encoder{enc: enc, w: w}
}

// CorruptInputError is returned when the input is not a valid base64dq.
type CorruptInputError int64

// Error implements the error interface.
func (e CorruptInputError) Error() string {
	return "illegal base64dq data at input byte " + strconv.FormatInt(int64(e), 10)
}

func (enc *Encoding) Decode(dst, src []byte) (int, error) {
	// Decode quantum using the base64 alphabet
	var dbuf [4]byte

	enc.buildOnce()
	n := enc.root
	padCount := 0
	lastBlock := 0 // position of last block boundary
	lastRune := 0  // position of last rune that contributed to the output
	i, k := enc.fast.decode(dst, src)
	j := 0
	lastBlock, lastRune = i, i

LOOP:
	for ; i < len(src); i++ {
		b := src[i]
		n = n.children[b]
		if n == nil {
			return 0, CorruptInputError(lastRune)
		}

		v := n.v
		if v < 0 {
			continue
		}
		if v == 64 {
			switch j % 4 {
			case 0, 1:
				// incorrect padding
				return 0, CorruptInputError(lastRune)
			}
			padCount++
			v = 0
		}

		dbuf[j%4] = byte(v)
		j++
		if j%4 == 0 {
			lastBlock = i + 1
			// Convert 4x 6bit source bytes into 3 bytes
			val := uint(dbuf[0])<<18 | uint(dbuf[1])<<12 | uint(dbuf[2])<<6 | uint(dbuf[3])
			switch padCount {
			case 0:
				dst[k+0] = byte(val >> 16)
				dst[k+1] = byte(val >> 8)
				dst[k+2] = byte(val >> 0)
				k += 3

				// back to the fast path
				si, di := enc.fast.decode(dst[k:], src[i+1:])
				i += si
				k += di
				lastBlock = i + 1
			case 1:
				dst[k+0] = byte(val >> 16)
				dst[k+1] = byte(val >> 8)
				if enc.strict && (val&0xFF) != 0 {
					return 0, CorruptInputError(lastRune)
				}
				k += 2
				i += 1
				break LOOP
			case 2:
				dst[k+0] = byte(val >> 16)
				if enc.strict && (val&0xFFFF) != 0 {
					return 0, CorruptInputError(lastRune)
				}
				k += 1
				i += 1
				break LOOP
			case 3, 4:
				return 0, CorruptInputError(lastRune)
			}
		}
		if n.v < 64 {
			lastRune = i + 1
		}
	}
	if n.v < 0 && n.v != rootNode {
		// invalid rune
		return 0, CorruptInputError(i)
	}

	// handle remaining bytes and padding
	if j%4 != 0 {
		if enc.padChar != NoPadding {
			if padCount == 0 {
				return 0, CorruptInputError(lastBlock)
			}
			return 0, CorruptInputError(i)
		}

		// Convert 4x 6bit source bytes into 3 bytes
		for i := j % 4; i < 4; i++ {
			dbuf[i] = 0
		}
		val := uint(dbuf[0])<<18 | uint(dbuf[1])<<12 | uint(dbuf[2])<<6 | uint(dbuf[3])
		switch j % 4 {
		case 0, 1:
			return 0, CorruptInputError(i)
		case 2:
			dst[k+0] = byte(val >> 16)
			if enc.strict && (val&0xFFFF) != 0 {
				return 0, CorruptInputError(lastRune)
			}
			k += 1
		case 3:
			dst[k+0] = byte(val >> 16)
			dst[k+1] = byte(val >> 8)
			if enc.strict && (val&0xFF) != 0 {
				return 0, CorruptInputError(lastRune)
			}
			k += 2
		}
	}
	for ; i < len(src); i++ {
		if src[i] != '\n' && src[i] != '\r' {
			// trailing garbage
			return 0, CorruptInputError(i)
		}
	}

	return k, nil
}

type decoder struct {
	enc     *Encoding
	r       io.Reader
	state   *node
	err     error
	readErr error // error from r.Read

	// buffer for input
	n         int64      // total bytes consumed
	padCount  int        // number of padding characters seen
	lastBlock int64      // position of last block boundary
	lastRune  int64      // position of last rune that contributed to the output
	buf       [4096]byte // source bytes waiting to be decoded
	pos       int        // current position in buf
	nbuf      int        // number of bytes in buf
	expectEOF bool       // whether a base64dq stream expects to end soon

	// buffer for output
	dbuf  [4]byte // Decode quantum using the base64 alphabet
	ndbuf int     // number of bytes in dbuf
	out   [3]byte // leftover decoded bytes from last Read
	nout  int     // number of bytes in out
}

func (d *decoder) Read(p []byte) (n int, err error) {
	// Use leftover decoded output from last read.
	if d.nout > 0 {
		n = copy(p, d.out[:d.nout])
		d.nout -= n
		copy(d.out[:], d.out[n:])
		return n, nil
	}

	if d.err != nil {
		return 0, d.err
	}

	// Refill buffer.
	if d.pos >= d.nbuf {
		d.pos = 0
		d.nbuf = 0
		nn := len(p) / 3 * 4 * d.enc.maxSize
		if nn < 4*d.enc.maxSize {
			nn = 4 * d.enc.maxSize
		}
		if nn > len(d.buf) {
			nn = len(d.buf)
		}
		for d.nbuf < 4*d.enc.maxSize && d.readErr == nil {
			var nr int
			nr, d.readErr = d.r.Read(d.buf[d.nbuf:nn])
			d.nbuf += nr
		}
	}

	if d.expectEOF {
		for ; d.pos < d.nbuf; d.pos, d.n = d.pos+1, d.n+1 {
			if d.buf[d.pos] != '\n' && d.buf[d.pos] != '\r' {
				// trailing garbage
				d.err = CorruptInputError(d.n)
				return 0, d.err
			}
		}
		d.err = d.readErr
		return 0, d.err
	}

	if d.ndbuf == 0 && d.padCount == 0 && (d.state == d.enc.root || d.state.v >= 0 && d.state.v < 64) {
		// at the boundary of quanta; try the fast path
		si, di := d.enc.fast.decode(p, d.buf[d.pos:d.nbuf])
		if si > 0 {
			d.pos += si
			d.n += int64(si)
			d.lastBlock = d.n
			d.lastRune = d.n
			p = p[di:]
			n += di
		}
	}

	for ; d.pos < d.nbuf && len(p) > 0; d.pos, d.n = d.pos+1, d.n+1 {
		b := d.buf[d.pos]
		d.state = d.state.children[b]
		if d.state == nil {
			d.err = CorruptInputError(d.lastRune)
			return n, d.err
		}

		v := d.state.v
		if v < 0 {
			continue
		}
		if v == 64 {
			switch d.ndbuf {
			case 0, 1:
				// incorrect padding
				d.err = CorruptInputError(d.lastRune)
				return n, d.err
			}
			d.padCount++
			v = 0
		}

		d.dbuf[d.ndbuf] = byte(v)
		d.ndbuf++
		if d.ndbuf == 4 {
			d.ndbuf = 0
			d.lastBlock = d.n + 1
			// Convert 4x 6bit source bytes into 3 bytes
			val := uint(d.dbuf[0])<<18 | uint(d.dbuf[1])<<12 | uint(d.dbuf[2])<<6 | uint(d.dbuf[3])
			if d.padCount == 0 && len(p) >= 3 {
				p[0] = byte(val >> 16)
				p[1] = byte(val >> 8)
				p[2] = byte(val >> 0)
				p = p[3:]
				n += 3

				// back to the fast path
				si, di := d.enc.fast.decode(p, d.buf[d.pos+1:d.nbuf])
				d.pos += si
				d.n += int64(si)
				d.lastBlock = d.n + 1
				p = p[di:]
				n += di
			} else {
				switch d.padCount {
				case 0:
					d.out[0] = byte(val >> 16)
					d.out[1] = byte(val >> 8)
					d.out[2] = byte(val >> 0)
					d.nout = 3
				case 1:
					d.out[0] = byte(val >> 16)
					d.out[1] = byte(val >> 8)
					if d.enc.strict && (val&0xFF) != 0 {
						d.err = CorruptInputError(d.lastRune)
						return n, d.err
					}
					d.nout = 2
					d.expectEOF = true
				case 2:
					d.out[0] = byte(val >> 16)
					if d.enc.strict && (val&0xFFFF) != 0 {
						d.err = CorruptInputError(d.lastRune)
						return n, d.err
					}
					d.nout = 1
					d.expectEOF = true
				case 3, 4:
					d.err = CorruptInputError(d.lastRune)
					return n, d.err
				}
				nn := copy(p, d.out[:d.nout])
				p = p[nn:]
				d.nout -= nn
				copy(d.out[:], d.out[nn:])
				n += nn
				if d.expectEOF {
					d.pos++
					d.n++
					break
				}
			}
		}
		if d.state.v < 64 {
			d.lastRune = d.n + 1
		}
	}
	if d.pos < d.nbuf {
		// p is full, but there are still bytes to decode in the buffer.
		// The error from r.Read will be reported after consuming them.
		return n, nil
	}
	d.err = d.readErr
	if errors.Is(d.err, io.EOF) {
		if d.state.v < 0 && d.state.v != rootNode {
			// invalid rune
			d.err = CorruptInputError(d.n)
		}

		// handle remaining bytes and padding
		if d.ndbuf > 0 {
			if d.enc.padChar != NoPadding {
				if d.padCount == 0 {
					d.err = CorruptInputError(d.lastBlock)
				} else {
					d.err = CorruptInputError(d.n)
				}
				return n, d.err
			}

			// Convert 4x 6bit source bytes into 3 bytes
			for i := d.ndbuf; i < 4; i++ {
				d.dbuf[i] = 0
			}
			val := uint(d.dbuf[0])<<18 | uint(d.dbuf[1])<<12 | uint(d.dbuf[2])<<6 | uint(d.dbuf[3])
			switch d.ndbuf {
			case 0, 1:
				d.err = CorruptInputError(d.n)
				return n, d.err
			case 2:
				d.out[0] = byte(val >> 16)
				if d.enc.strict && (val&0xFFFF) != 0 {
					d.err = CorruptInputError(d.lastRune)
					return n, d.err
				}
				d.nout = 1
			case 3:
				d.out[0] = byte(val >> 16)
				d.out[1] = byte(val >> 8)
				if d.enc.strict && (val&0xFF) != 0 {
					d.err = CorruptInputError(d.lastRune)
					return n, d.err
				}
				d.nout = 2
			}
			d.ndbuf = 0

			// p may not have enough room; keep the rest in d.out for the next Read.
			nn := copy(p, d.out[:d.nout])
			d.nout -= nn
			copy(d.out[:], d.out[nn:])
			n += nn
			d.expectEOF = true
		}
	}
	if d.nout > 0 {
		// The error will be reported after the leftover is consumed.
		return n, nil
	}
	return n, d.err
}

// NewDecoder constructs a new base64 stream decoder.
func NewDecoder(enc *Encoding, r io.Reader) io.Reader {
	enc.buildOnce()
	return &decoder{enc: enc, r: r, state: enc.root}
}

// DecodeString returns the bytes represented by the base64 string s.
func (enc *Encoding) DecodeString(s string) ([]byte, error) {
	dbuf := make([]byte, enc.DecodedLen(len(s)))
	n, err := enc.Decode(dbuf, []byte(s))
	return dbuf[:n], err
}

// DecodedLen returns the maximum length in bytes of the decoded data
// corresponding to n bytes of base64-encoded data.
func (enc *Encoding) DecodedLen(n int) int {
	if enc.padChar == NoPadding {
		// Unpadded data may end with partial block of 2-3 characters.
		return n * 6 / 8
	}
	// Padded base64 should always be a multiple of 4 characters in length.
	return n / 4 * 3
}
