package base64dq

import (
	"log"
	"testing"
)

func TestEncode(t *testing.T) {
	dst := make([]byte, 12)
	n := StdEncoding.Encode(dst, []byte{0, 0, 0})
	log.Print(string(dst[:n]))
}
