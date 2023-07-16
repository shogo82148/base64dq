package base64dq

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"testing"
)

type testpair struct {
	decoded, encoded string
}

var pairs = []testpair{
	// https://github.com/yoshi389111/dq1pswd/blob/b1a5d107eefd7b263d312d1a255a7eb5f0120a42/dq1ana.md?plain=1#L16-L23
	{"\x10\xaf\x91\x55\x97\x6b\xbe\xfd\xba\xf8\x21\x8a\x38\xa5\x59", "おさべつにはほわげげだどべうきさそさには"},
	{"\x14\x9f\x50\x51\x87\x2a\xba\xef\x41\x14\x93\x51\x55\x17\x20", "かこぶちなのへろぐぐぶいかこせつにつへむ"},
	{"\x20\xd0\x54\x61\xcc\x3e\x08\x24\x55\x65\xd8\x65\xa6\x5c\x34", "けせいなのへごべううつにはほめよれよごぜ"},
	{"\x25\x01\x17\x6d\xfc\xc1\x14\x53\x10\x51\x87\x20\x92\x0a\xef", "こちおねふみずいかかすちなのへむゆむわげ"},
	{"\x92\x87\x0f\x4d\x76\xe9\xb6\xdd\x38\xf0\x01\x08\x30\x84\xd7", "ゆるへたとねふれぎぎぜづびあおけすけとね"},
	{"\x96\xb7\xd2\x59\xa7\xac\xc3\x0f\xc3\x1c\xb3\xd3\x5d\x37\xa2", "よわみてぬひまがごごぼえくしたとねとまも"},
	{"\xa2\xf8\xd6\x69\xec\x80\x10\x44\xd7\x6d\xf8\xe7\xae\x7c\xb6", "るげやぬひまじあおおとねふみやりわりじだ"},
	{"\xa6\xe8\x95\x65\xdc\x7f\x0c\x32\x8e\x49\x66\x9e\x89\xea\x6d", "れぐもにはほざぼええさそてぬひまもまれぎ"},

	// RFC 3548 examples
	{"\x14\xfb\x9c\x03\xd9\x7e", "かたぐへあぶよべ"},
	{"\x14\xfb\x9c\x03\xd9", "かたぐへあぶゆ・"},
	{"\x14\xfb\x9c\x03", "かたぐへあご・・"},

	// RFC 4648 examples
	{"", ""},
	{"f", "はむ・・"},
	{"fo", "はらび・"},
	{"foo", "はらぶげ"},
	{"foob", "はらぶげのむ・・"},
	{"fooba", "はらぶげのらお・"},
	{"foobar", "はらぶげのらかじ"},

	// Wikipedia examples
	{"sure.", "へぢにじはてづ・"},
	{"sure", "へぢにじはち・・"},
	{"sur", "へぢにじ"},
	{"su", "へぢな・"},
	{"leasure.", "ふきにめへぢにじはてづ・"},
	{"easure.", "はぬかずほねこよしむ・・"},
	{"asure.", "のねせぞへらなぐ"},
	{"sure.", "へぢにじはてづ・"},
}

var std2dq = strings.NewReplacer(
	"A", "あ",
	"B", "い",
	"C", "う",
	"D", "え",
	"E", "お",
	"F", "か",
	"G", "き",
	"H", "く",
	"I", "け",
	"J", "こ",
	"K", "さ",
	"L", "し",
	"M", "す",
	"N", "せ",
	"O", "そ",
	"P", "た",
	"Q", "ち",
	"R", "つ",
	"S", "て",
	"T", "と",
	"U", "な",
	"V", "に",
	"W", "ぬ",
	"X", "ね",
	"Y", "の",
	"Z", "は",
	"a", "ひ",
	"b", "ふ",
	"c", "へ",
	"d", "ほ",
	"e", "ま",
	"f", "み",
	"g", "む",
	"h", "め",
	"i", "も",
	"j", "や",
	"k", "ゆ",
	"l", "よ",
	"m", "ら",
	"n", "り",
	"o", "る",
	"p", "れ",
	"q", "ろ",
	"r", "わ",
	"s", "が",
	"t", "ぎ",
	"u", "ぐ",
	"v", "げ",
	"w", "ご",
	"x", "ざ",
	"y", "じ",
	"z", "ず",
	"0", "ぜ",
	"1", "ぞ",
	"2", "だ",
	"3", "ぢ",
	"4", "づ",
	"5", "で",
	"6", "ど",
	"7", "ば",
	"8", "び",
	"9", "ぶ",
	"+", "べ",
	"/", "ぼ",
	"=", "・",
)

var dq2std = strings.NewReplacer(
	"あ", "A",
	"い", "B",
	"う", "C",
	"え", "D",
	"お", "E",
	"か", "F",
	"き", "G",
	"く", "H",
	"け", "I",
	"こ", "J",
	"さ", "K",
	"し", "L",
	"す", "M",
	"せ", "N",
	"そ", "O",
	"た", "P",
	"ち", "Q",
	"つ", "R",
	"て", "S",
	"と", "T",
	"な", "U",
	"に", "V",
	"ぬ", "W",
	"ね", "X",
	"の", "Y",
	"は", "Z",
	"ひ", "a",
	"ふ", "b",
	"へ", "c",
	"ほ", "d",
	"ま", "e",
	"み", "f",
	"む", "g",
	"め", "h",
	"も", "i",
	"や", "j",
	"ゆ", "k",
	"よ", "l",
	"ら", "m",
	"り", "n",
	"る", "o",
	"れ", "p",
	"ろ", "q",
	"わ", "r",
	"が", "s",
	"ぎ", "t",
	"ぐ", "u",
	"げ", "v",
	"ご", "w",
	"ざ", "x",
	"じ", "y",
	"ず", "z",
	"ぜ", "0",
	"ぞ", "1",
	"だ", "2",
	"ぢ", "3",
	"づ", "4",
	"で", "5",
	"ど", "6",
	"ば", "7",
	"び", "8",
	"ぶ", "9",
	"べ", "+",
	"ぼ", "/",
	"・", "=",
)

// Do nothing to a reference base64 string (leave in standard format)
func stdRef(ref string) string {
	return ref
}

// Convert a reference string to raw, unpadded format
func rawRef(ref string) string {
	return strings.TrimRight(ref, "・")
}

type encodingTest struct {
	enc  *Encoding           // Encoding to test
	conv func(string) string // Reference string converter
}

var encodingTests = []encodingTest{
	{StdEncoding, stdRef},
	{RawStdEncoding, rawRef},
	{StdEncoding.Strict(), stdRef},
	{RawStdEncoding.Strict(), rawRef},
}

var bigtest = testpair{
	"Twas brillig, and the slithy toves",
	"にくほめへじいもへらよがふきよりしういめふらちむほきめよけくせがひねつるまていぜふぢはよへご・・",
}

func TestEncode(t *testing.T) {
	for _, p := range pairs {
		for _, tt := range encodingTests {
			encoded := tt.enc.EncodeToString([]byte(p.decoded))
			if encoded != tt.conv(p.encoded) {
				t.Errorf("Encode(%q) = %q, want %q", p.decoded, encoded, tt.conv(p.encoded))
			}
		}
	}

	// test compatibility with standard base64
	for _, p := range pairs {
		encoded := StdEncoding.EncodeToString([]byte(p.decoded))
		encoded2 := std2dq.Replace(base64.StdEncoding.EncodeToString([]byte(p.decoded)))
		if encoded != encoded2 {
			t.Errorf("Encode(%q) = %q, want %q", p.decoded, encoded, encoded2)
		}
	}
}

func TestEncoder(t *testing.T) {
	for _, p := range pairs {
		bb := &strings.Builder{}
		encoder := NewEncoder(StdEncoding, bb)
		if _, err := encoder.Write([]byte(p.decoded)); err != nil {
			t.Errorf("Encoder.Write(%q) error: %v", p.decoded, err)
		}
		if err := encoder.Close(); err != nil {
			t.Error("Encoder.Close() error:", err)
		}
		if bb.String() != p.encoded {
			t.Errorf("Encode(%q) = %q, want %q", p.decoded, bb.String(), p.encoded)
		}
	}
}

func TestEncoderBuffering(t *testing.T) {
	input := []byte(bigtest.decoded)
	for bs := 1; bs <= 12; bs++ {
		bb := &strings.Builder{}
		encoder := NewEncoder(StdEncoding, bb)
		for pos := 0; pos < len(input); pos += bs {
			end := pos + bs
			if end > len(input) {
				end = len(input)
			}
			n, err := encoder.Write(input[pos:end])
			if err != nil {
				t.Errorf("Write(%q) error: %v", input[pos:end], err)
			}
			if n != end-pos {
				t.Errorf("Write(%q) gave length %v, want %v", input[pos:end], n, end-pos)
			}
		}
		if err := encoder.Close(); err != nil {
			t.Error("Close gave error:", err)
		}
		if bb.String() != bigtest.encoded {
			t.Errorf("Encoding/%d of %q = %q, want %q", bs, bigtest.decoded, bb.String(), bigtest.encoded)
		}
	}
}

const emoji = "😀😃😄😁😆😅😂🙂🙃😉😊😇😍😘😗☺️😚😙😋😛😜😝🤑🤗🤔🤐😐😑😶😏😒🙄😬😌😔😪😴😷🤒🤕😵😎🤓😕😟🙁☹️😮😯😲😳😦😧😨😰😥😢😭😱😖😣😞"

var emojiEncode = NewEncoding(emoji)

func TestEncode_Emoji(t *testing.T) {
	for _, p := range pairs {
		encoded := emojiEncode.EncodeToString([]byte(p.decoded))
		t.Log(encoded)
	}
}

func TestEncodedLen(t *testing.T) {
	for _, tt := range []struct {
		enc  *Encoding
		n    int
		want int
	}{
		// Japanese hiragana has 3 bytes per character in utf-8.
		// So we need 3 times larger buffer than the standard base64.
		{RawStdEncoding, 0, 0},
		{RawStdEncoding, 1, 2 * 3},
		{RawStdEncoding, 2, 3 * 3},
		{RawStdEncoding, 3, 4 * 3},
		{RawStdEncoding, 7, 10 * 3},

		// We need 3 times larger buffer than the standard base64.
		{StdEncoding, 0, 0},
		{StdEncoding, 1, 4 * 3},
		{StdEncoding, 2, 4 * 3},
		{StdEncoding, 3, 4 * 3},
		{StdEncoding, 4, 8 * 3},
		{StdEncoding, 7, 12 * 3},

		// Emoji has 4 bytes per character in utf-8.
		// We need larger buffer than Japanese hiragana.
		{emojiEncode, 0, 0},
		{emojiEncode, 1, 4 * 4},
		{emojiEncode, 2, 4 * 4},
		{emojiEncode, 3, 4 * 4},
		{emojiEncode, 4, 8 * 4},
		{emojiEncode, 7, 12 * 4},
	} {
		if got := tt.enc.EncodedLen(tt.n); got != tt.want {
			t.Errorf("EncodedLen(%d): got %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestDecode(t *testing.T) {
	for _, p := range pairs {
		for _, tt := range encodingTests {
			encoded := tt.conv(p.encoded)
			decoded, err := tt.enc.DecodeString(encoded)
			if err != nil {
				t.Errorf("Decode(%q) = %v", p.encoded, err)
			}
			if string(decoded) != string(p.decoded) {
				t.Errorf("Decode(%q) = %q, want %q", p.encoded, decoded, p.decoded)
			}
		}
	}

	// test compatibility with standard base64
	for _, p := range pairs {
		decoded, err := StdEncoding.DecodeString(p.encoded)
		if err != nil {
			t.Errorf("Decode(%q) = %v", p.encoded, err)
		}
		decoded2, err := base64.StdEncoding.DecodeString(dq2std.Replace(p.encoded))
		if err != nil {
			t.Errorf("Decode(%q) = %v", p.encoded, err)
		}
		if string(decoded) != string(decoded2) {
			t.Errorf("Decode(%q) = %q, want %q", p.encoded, decoded, decoded2)
		}
	}
}

func TestDecodeCorrupt(t *testing.T) {
	testCases := []struct {
		input  string
		offset int // -1 means no corruption.
	}{
		{"", -1},
		{"\n", -1},
		{"あああ・\n", -1},
		{"ああああ\n", -1},
		{"！！！！", 0},
		{"・・・・", 0},
		{"が・・・", len("が")},
		{"・あああ", 0},
		{"あ・ああ", len("あ")},
		{"ああ・あ", len("ああ")},
		{"ああ・・あ", len("ああ・・")},
		{"あああ・ああああ", len("あああ・")},
		{"あああああ", len("ああああ")},
		{"ああああああ", len("ああああ")},
		{"あ・", len("あ")},
		{"あ・・", len("あ")},
		{"ああ・", len("ああ・")},
		{"ああ・・", -1},
		{"あああ・", -1},
		{"ああああ", -1},
		{"ああああああ・", len("ああああああ・")},
		{"ふるいけやか・・・・・", len("ふるいけやか・・")},
		{"あ！\n", len("あ")},
		{"あ・\n", len("あ")},
	}
	for _, tc := range testCases {
		dbuf := make([]byte, StdEncoding.DecodedLen(len(tc.input)))
		_, err := StdEncoding.Decode(dbuf, []byte(tc.input))
		if tc.offset == -1 {
			if err != nil {
				t.Error("Decoder wrongly detected corruption in", tc.input)
			}
			continue
		}
		switch err := err.(type) {
		case CorruptInputError:
			if int(err) != tc.offset {
				t.Errorf("Decoder wrongly detected corruption in %q at offset %d, want %d", tc.input, err, tc.offset)
			}
		default:
			t.Error("Decoder failed to detect corruption in", tc)
		}
	}
}

func TestDecoder(t *testing.T) {
	for _, p := range pairs {
		decoder := NewDecoder(StdEncoding, strings.NewReader(p.encoded))
		dbuf := make([]byte, StdEncoding.DecodedLen(len(p.encoded)))
		count, err := decoder.Read(dbuf)
		if err != nil && err != io.EOF {
			t.Fatal("Read failed", err)
		}
		if count != len(p.decoded) {
			t.Errorf("Read from %q = length %v, want %v", p.encoded, count, len(p.decoded))
		}
		if string(dbuf[0:count]) != p.decoded {
			t.Errorf("Decoding of %q = %q, want %q", p.encoded, string(dbuf[0:count]), p.decoded)
		}
		if err != io.EOF {
			count, err = decoder.Read(dbuf)
			if count != 0 {
				t.Errorf("Read after EOF from %q = %d, want 0", p.encoded, count)
			}
		}
		if err != io.EOF {
			t.Errorf("Read from %q = %v, want %v", p.encoded, err, io.EOF)
		}
	}
}

func BenchmarkEncodeToString(b *testing.B) {
	data := make([]byte, 8192)
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		StdEncoding.EncodeToString(data)
	}
}

func BenchmarkEncodeToString_Std(b *testing.B) {
	enc := NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/").WithPadding('=')
	data := make([]byte, 8192)
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		enc.EncodeToString(data)
	}
}

func BenchmarkEncodeToString_StdBase64(b *testing.B) {
	data := make([]byte, 8192)
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		base64.StdEncoding.EncodeToString(data)
	}
}

func BenchmarkDecodeString(b *testing.B) {
	sizes := []int{2, 4, 8, 64, 8192}
	benchFunc := func(b *testing.B, benchSize int) {
		data := StdEncoding.EncodeToString(make([]byte, benchSize))
		b.SetBytes(int64(len(data)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			StdEncoding.DecodeString(data)
		}
	}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			benchFunc(b, size)
		})
	}
}
