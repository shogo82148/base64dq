// Package base64dq implements a base64 encoding variant that is inspired by the Revival Password of Dragon Quest.
//
// The Revival Password (ふっかつのじゅもん) is a string of 20 characters that is used to revive a player's party in the [Dragon Quest] series.
// It is encoded in a custom base64 variant that uses 64 characters from the Japanese hiragana syllabary.
//
// [Dragon Quest]: https://www.dragonquest.jp/
package base64dq

import (
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

type Encoding struct {
	encode  [64]string
	decode  decodeMap
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
	return ret * utf8.UTFMax // maximum # bytes: utf8.UTFMax bytes per char
}

type CorruptInputError int64

func (e CorruptInputError) Error() string {
	return "illegal base64dq data at input byte " + strconv.FormatInt(int64(e), 10)
}

func (enc *Encoding) Decode(dst, src []byte) (int, error) {
	var err error
	n := 0
	si, sj, sk := 0, 0, 0
	dlen := 4

	for si < len(src) {
		// Decode quantum using the base64 alphabet
		var dbuf [4]byte

		for j := 0; j < len(dbuf); j++ {
			if len(src) == si {
				switch {
				case j == 0:
					return n, nil
				case j == 1, enc.padChar != NoPadding:
					return n, CorruptInputError(sj)
				}
				dlen = j
				break
			}
			r, size := utf8.DecodeRune(src[si:])
			if r == utf8.RuneError {
				return n, CorruptInputError(si)
			}
			si, sj, sk = si+size, si, sj

			out := enc.decode.search(r)
			if out != 0xFF {
				dbuf[j] = out
				continue
			}

			if r == '\n' || r == '\r' {
				j--
				continue
			}

			if r != enc.padChar {
				return n, CorruptInputError(sj)
			}

			// We've reached the end and there's padding
			switch j {
			case 0, 1:
				// incorrect padding
				return n, CorruptInputError(sj)
			case 2:
				// "・・" is expected, the first "・" is already consumed.
				// skip over newlines
				for si < len(src) && (src[si] == '\n' || src[si] == '\r') {
					si, sj, sk = si+1, si, sj
				}
				if si == len(src) {
					// not enough padding
					return n, CorruptInputError(len(src))
				}
				pad, size := utf8.DecodeRune(src[si:])
				if pad != enc.padChar {
					// incorrect padding
					return n, CorruptInputError(sj)
				}
				si += size
			}

			// skip over newlines
			for si < len(src) && (src[si] == '\n' || src[si] == '\r') {
				si, sj, sk = si+1, si, sj
			}
			if si < len(src) {
				// trailing garbage
				err = CorruptInputError(si)
			}
			dlen = j
			break
		}

		// Convert 4x 6bit source bytes into 3 bytes
		val := uint(dbuf[0])<<18 | uint(dbuf[1])<<12 | uint(dbuf[2])<<6 | uint(dbuf[3])
		dbuf[2], dbuf[1], dbuf[0] = byte(val>>0), byte(val>>8), byte(val>>16)

		switch dlen {
		case 4:
			dst[n+2] = dbuf[2]
			dbuf[2] = 0
			fallthrough
		case 3:
			dst[n+1] = dbuf[1]
			if enc.strict && dbuf[2] != 0 {
				return n, CorruptInputError(sj)
			}
			dbuf[1] = 0
			fallthrough
		case 2:
			dst[n+0] = dbuf[0]
			if enc.strict && (dbuf[1] != 0 || dbuf[2] != 0) {
				return n, CorruptInputError(sk)
			}
		}
		n += dlen - 1
		if err != nil {
			return n, err
		}
	}

	return n, nil
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
