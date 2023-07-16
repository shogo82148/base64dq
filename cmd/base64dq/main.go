package main

import (
	"base64dq"
	"flag"
	"io"
	"log"
	"os"
)

func main() {
	os.Exit(run())
}

func run() int {
	var decode bool
	flag.BoolVar(&decode, "d", false, "decode data")
	flag.BoolVar(&decode, "decode", false, "decode data")
	flag.Parse()
	if decode {
		return runDecode(os.Stdout, os.Stdin)
	} else {
		return runEncode(os.Stdout, os.Stdin)
	}
}

func runEncode(w io.Writer, r io.Reader) int {
	enc := base64dq.NewEncoder(base64dq.StdEncoding, w)
	if _, err := io.Copy(enc, r); err != nil {
		log.Println(err)
		return 1
	}
	if err := enc.Close(); err != nil {
		log.Println(err)
		return 1
	}
	return 0
}

func runDecode(w io.Writer, r io.Reader) int {
	dec := base64dq.NewDecoder(base64dq.StdEncoding, r)
	if _, err := io.Copy(w, dec); err != nil {
		log.Println(err)
		return 1
	}
	return 0
}
