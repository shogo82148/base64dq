// Package base64dq implements a base64 encoding variant that is inspired by the Revival Password of Dragon Quest.
//
// The Revival Password (ふっかつのじゅもん) is a string of 20 characters that is used to revive a player's party in the [Dragon Quest] series.
// It is encoded in a custom base64 variant that uses 64 characters from the Japanese hiragana syllabary.
//
// [Dragon Quest]: https://www.dragonquest.jp/
package base64dq

import (
	"errors"
	"io"
	"sort"
	"strconv"
	"unicode/utf8"
)

type decodeMap struct {
	runes [64]rune
	bytes [64]byte
}

func (dm *decodeMap) Len() int {
	return len(dm.runes)
}

func (dm *decodeMap) Less(i, j int) bool {
	return dm.runes[i] < dm.runes[j]
}

func (dm *decodeMap) Swap(i, j int) {
	dm.runes[i], dm.runes[j] = dm.runes[j], dm.runes[i]
	dm.bytes[i], dm.bytes[j] = dm.bytes[j], dm.bytes[i]
}

func (dm *decodeMap) sort() {
	sort.Sort(dm)
}

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

func buildTrie(entries [64]string, padding rune) *node {
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
	encode  [64]string
	decode  decodeMap
	maxSize int
	padChar rune
	strict  bool
}

// Strict creates a new encoding identical to enc except with
// strict decoding enabled. In this mode, the decoder requires that
// trailing padding bits are zero.
//
// Note that the input is still malleable, as new line characters
// (CR and LF) are still ignored.
func (enc Encoding) Strict() *Encoding {
	enc.strict = true
	return &enc
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
		e.decode.runes[j] = ch
		e.decode.bytes[j] = byte(j)
		j++
	}
	pos[64] = len(encoder)

	for i := 0; i < 64; i++ {
		e.encode[i] = encoder[pos[i]:pos[i+1]]
		if size := pos[i+1] - pos[i]; size > e.maxSize {
			e.maxSize = size
		}
	}
	if size := utf8.RuneLen(e.padChar); size > e.maxSize {
		e.maxSize = size
	}
	e.decode.sort()

	return e
}

// WithPadding creates a new encoding identical to enc except
// with a specified padding character, or NoPadding to disable padding.
// The padding character must not be '\r' or '\n', must not
// be contained in the encoding's alphabet.
func (enc Encoding) WithPadding(padding rune) *Encoding {
	if padding == '\r' || padding == '\n' {
		panic("invalid padding")
	}

	if enc.decode.search(padding) != 0xff {
		panic("padding contained in alphabet")
	}

	enc.padChar = padding
	return &enc
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
	buf := make([]byte, enc.EncodedLen(len(src)))
	n := enc.Encode(buf, src)
	return string(buf[:n])
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
	root := buildTrie(enc.encode, enc.padChar)

	// Decode quantum using the base64 alphabet
	var dbuf [4]byte

	n := root
	padCount := 0
	lastBlock := 0 // position of last block boundary
	lastRune := 0  // position of last rune that contributed to the output
	i := 0
	j := 0
	k := 0

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
	enc *Encoding
	r   *runeReader

	// buffer for input
	n    int64   // total bytes consumed
	si   int     // index of first source byte
	buf  [4]byte // source bytes waiting to be decoded
	nbuf int     // number of bytes in buf

	// buffer for output
	out  [3]byte // leftover decoded bytes from last Read
	nout int     // number of bytes in out
}

func (d *decoder) decodeQuantum() error {
	for d.nbuf < len(d.buf) {
		r, size, err := d.r.ReadRune()
		if errors.Is(err, io.EOF) {
			switch {
			case d.nbuf == 0:
				return err
			case d.nbuf == 1:
				return CorruptInputError(d.n)
			case d.enc.padChar != NoPadding:
				return CorruptInputError(d.n)
			}
			d.si += size
			break
		}
		if err != nil {
			return err
		}
		if r == utf8.RuneError {
			return CorruptInputError(d.n)
		}
		if r == '\n' || r == '\r' {
			d.si += size
			continue
		}

		out := d.enc.decode.search(r)
		if out != 0xFF {
			d.buf[d.nbuf] = out
			d.nbuf++
			d.si += size
			continue
		}

		if r != d.enc.padChar {
			return CorruptInputError(d.n + int64(d.si))
		}

		// We've reached the end and there's padding
		switch d.nbuf {
		case 0, 1:
			// incorrect padding
			return CorruptInputError(d.n + int64(d.si))
		case 2:
			// "・・" is expected, the first "・" is already consumed.
			pad := d.si
			d.si += size
			for {
				r, size, err = d.r.ReadRune()
				if errors.Is(err, io.EOF) {
					// not enough padding
					return CorruptInputError(d.n + int64(d.si))
				}
				if err != nil {
					return err
				}
				if r != '\n' && r != '\r' {
					break
				}
				d.si += size
			}
			if r != d.enc.padChar {
				// incorrect padding
				return CorruptInputError(d.n + int64(pad))
			}
			d.si += size
		default:
			d.si += size
		}

		// skip over newlines
		for {
			r, size, err = d.r.ReadRune()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if r != '\n' && r != '\r' {
				// trailing garbage
				return CorruptInputError(d.n + int64(d.si))
			}
			d.si += size
		}
		break
	}

	// Convert 4x 6bit source bytes into 3 bytes
	val := uint(d.buf[0])<<18 | uint(d.buf[1])<<12 | uint(d.buf[2])<<6 | uint(d.buf[3])
	d.out = [3]byte{byte(val >> 16), byte(val >> 8), byte(val >> 0)}
	switch d.nbuf {
	case 4:
		d.nout = 3
	case 3:
		d.nout = 2
		if d.enc.strict && d.out[2] != 0 {
			return CorruptInputError(d.n)
		}
	case 2:
		d.nout = 1
		if d.enc.strict && (d.out[1] != 0 || d.out[2] != 0) {
			return CorruptInputError(d.n)
		}
	}
	d.n += int64(d.si)
	d.si = 0
	d.nbuf = 0
	return nil
}

func (d *decoder) Read(p []byte) (n int, err error) {
	if d.nout > 0 {
		n = copy(p, d.out[:d.nout])
		d.nout -= n
		copy(d.out[:], d.out[n:])
		return n, nil
	}

	for {
		if err := d.decodeQuantum(); err != nil {
			return n, err
		}
		nn := copy(p, d.out[:d.nout])
		p = p[nn:]
		n += nn
		d.nout -= nn
		copy(d.out[:], d.out[nn:])
		if len(p) <= 0 || d.r.Buffered() < 4*d.enc.maxSize {
			break
		}
	}
	return
}

// runeReader is a simplified version of bufio.Reader that only supports.
type runeReader struct {
	wrapped io.Reader
	buf     [4096]byte
	r, w    int
	err     error
}

// fill reads a new chunk into the buffer.
func (r *runeReader) fill() {
	// Slide existing data to beginning.
	if r.r > 0 {
		copy(r.buf[:], r.buf[r.r:r.w])
		r.w -= r.r
		r.r = 0
	}

	if r.w >= len(r.buf) {
		panic("base64dq: tried to fill full buffer")
	}

	n, err := r.wrapped.Read(r.buf[r.w:])
	r.w += n
	r.err = err
}

func (r *runeReader) ReadRune() (ch rune, size int, err error) {
	for r.r+utf8.UTFMax > r.w && !utf8.FullRune(r.buf[r.r:r.w]) && r.err == nil && r.w-r.r < len(r.buf) {
		r.fill()
	}
	if r.r == r.w {
		return 0, 0, r.readErr()
	}
	ch, size = utf8.DecodeRune(r.buf[r.r:r.w])
	r.r += size
	return ch, size, nil
}

// Buffered returns the number of bytes that can be read from the current buffer.
func (r *runeReader) Buffered() int { return r.w - r.r }

func (b *runeReader) readErr() error {
	err := b.err
	b.err = nil
	return err
}

// NewDecoder constructs a new base64 stream decoder.
func NewDecoder(enc *Encoding, r io.Reader) io.Reader {
	return &decoder{enc: enc, r: &runeReader{wrapped: r}}
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
