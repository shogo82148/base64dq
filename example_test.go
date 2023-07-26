// Keep in sync with base32/example_test.go and base64/example_test.go of the standard library.

package base64dq_test

import (
	"fmt"
	"os"

	"github.com/shogo82148/base64dq"
)

func Example() {
	msg := "Hello, 世界"
	encoded := base64dq.StdEncoding.EncodeToString([]byte(msg))
	fmt.Println(encoded)
	decoded, err := base64dq.StdEncoding.DecodeString(encoded)
	if err != nil {
		fmt.Println("decode error:", err)
		return
	}
	fmt.Println(string(decoded))
	// Output:
	// てきにがふきびがけそてづよぐまにやあ・・
	// Hello, 世界
}

func ExampleEncoding_EncodeToString() {
	data := []byte("any + old & data")
	str := base64dq.StdEncoding.EncodeToString(data)
	fmt.Println(str)
	// Output:
	// のぬででけうがむふだざゆけうのむはきかぜのち・・
}

func ExampleEncoding_Encode() {
	data := []byte("Hello, world!")
	dst := make([]byte, base64dq.StdEncoding.EncodedLen(len(data)))
	base64dq.StdEncoding.Encode(dst, data)
	fmt.Println(string(dst))
	// Output:
	// てきにがふきびがけくほげへらざゆけち・・
}

func ExampleEncoding_DecodeString() {
	str := "へだぶぎはていゆのねつめけくほれほきむむあういめふらちむばばぐぼ"
	data, err := base64dq.StdEncoding.DecodeString(str)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%q\n", data)
	// Output:
	// "some data with \x00 and \ufeff"
}

func ExampleEncoding_Decode() {
	str := "てきにがふきびがけくほげへらざゆけち・・"
	dst := make([]byte, base64dq.StdEncoding.DecodedLen(len(str)))
	n, err := base64dq.StdEncoding.Decode(dst, []byte(str))
	if err != nil {
		fmt.Println("decode error:", err)
		return
	}
	dst = dst[:n]
	fmt.Printf("%q\n", dst)
	// Output:
	// "Hello, world!"
}

func ExampleNewEncoder() {
	input := []byte("foo\x00bar")
	encoder := base64dq.NewEncoder(base64dq.StdEncoding, os.Stdout)
	encoder.Write(input)
	// Must close the encoder when finished to flush any partial blocks.
	// If you comment out the following line, the last partial block "r"
	// won't be encoded.
	encoder.Close()
	// Output:
	// はらぶげあきこめへむ・・
}
