package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunEncode(t *testing.T) {
	r := strings.NewReader("Hello, 世界")
	w := new(bytes.Buffer)
	code := runEncode(w, r)
	if code != 0 {
		t.Error("code != 0")
	}
	if w.String() != "てきにがふきびがけそてづよぐまにやあ・・" {
		t.Error("w.String() != `てきにがふきびがけそてづよぐまにやあ・・`")
	}
}

func TestRunDecode(t *testing.T) {
	r := strings.NewReader("てきにがふきびがけそてづよぐまにやあ・・")
	w := new(bytes.Buffer)
	code := runDecode(w, r)
	if code != 0 {
		t.Error("code != 0")
	}
	if w.String() != "Hello, 世界" {
		t.Error("w.String() != `Hello, 世界`")
	}
}
