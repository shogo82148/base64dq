package base64dq

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

// validate alphabets
func validAlphabets(alphabets string) bool {
	if !utf8.ValidString(alphabets) {
		return false
	}
	if utf8.RuneCountInString(alphabets) != 64 {
		return false
	}
	if strings.ContainsRune(alphabets, utf8.RuneError) {
		return false
	}
	if strings.ContainsAny(alphabets, "・\n\r") {
		return false
	}
	seen := map[rune]bool{}
	for _, r := range alphabets {
		if seen[r] {
			return false
		}
		seen[r] = true
	}
	return true
}

func FuzzEncode(f *testing.F) {
	for _, p := range pairs {
		f.Add(encodeStd, []byte(p.encoded))
	}
	f.Fuzz(func(t *testing.T, alphabets string, data []byte) {
		if !validAlphabets(alphabets) {
			return
		}
		enc := NewEncoding(alphabets)
		encoded := enc.EncodeToString(data)
		decoded, err := enc.DecodeString(encoded)
		if err != nil {
			t.Error(err)
		}
		if string(decoded) != string(data) {
			t.Errorf("%q: decoded %q, want %q", encoded, decoded, data)
		}
	})
}

func FuzzEncoder(f *testing.F) {
	for _, p := range pairs {
		f.Add(encodeStd, []byte(p.encoded))
	}
	f.Fuzz(func(t *testing.T, alphabets string, data []byte) {
		if !validAlphabets(alphabets) {
			return
		}
		enc := NewEncoding(alphabets)
		buf := new(bytes.Buffer)
		e := NewEncoder(enc, buf)
		if _, err := e.Write(data); err != nil {
			t.Error(err)
		}
		if err := e.Close(); err != nil {
			t.Error(err)
		}
		encoded := buf.String()
		decoded, err := enc.DecodeString(encoded)
		if err != nil {
			t.Error(err)
		}
		if string(decoded) != string(data) {
			t.Errorf("%q: decoded %q, want %q", encoded, decoded, data)
		}
	})
}

func FuzzDecode(f *testing.F) {
	for _, p := range pairs {
		f.Add(encodeStd, p.decoded)
	}
	for _, t := range decodeCorruptTestCases {
		f.Add(encodeStd, t.input)
	}
	f.Fuzz(func(t *testing.T, alphabets, data string) {
		if !validAlphabets(alphabets) {
			return
		}
		enc := NewEncoding(alphabets)
		decoded, err := enc.DecodeString(data)
		if err != nil {
			return
		}
		encoded := enc.EncodeToString(decoded)
		decoded2, err := enc.DecodeString(encoded)
		if err != nil {
			t.Error(err)
		}
		if string(decoded2) != string(decoded) {
			t.Errorf("%q: decoded %q, want %q", encoded, decoded2, decoded)
		}
	})
}

func FuzzDecoder(f *testing.F) {
	for _, p := range pairs {
		f.Add(encodeStd, p.decoded)
	}
	for _, t := range decodeCorruptTestCases {
		f.Add(encodeStd, t.input)
	}
	f.Fuzz(func(t *testing.T, alphabets, data string) {
		if !validAlphabets(alphabets) {
			return
		}
		enc := NewEncoding(alphabets)
		d := NewDecoder(enc, strings.NewReader(data))
		decoded, err := io.ReadAll(d)
		if err != nil {
			return
		}
		encoded := enc.EncodeToString(decoded)
		decoded2, err := enc.DecodeString(encoded)
		if err != nil {
			t.Error(err)
		}
		if string(decoded2) != string(decoded) {
			t.Errorf("%q: decoded %q, want %q", encoded, decoded2, decoded)
		}
	})
}
