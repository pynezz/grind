package crockford

import (
	"bytes"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	cases := [][]byte{
		{},
		{0x00},
		{0xff},
		[]byte("grind"),
		bytes.Repeat([]byte{0xAB, 0xCD, 0xEF}, 7),
	}
	for _, in := range cases {
		enc := Encode(in)
		out, err := Decode(enc)
		if err != nil {
			t.Fatalf("Decode(%q): %v", enc, err)
		}
		if !bytes.Equal(in, out) {
			t.Fatalf("round trip mismatch: in=%x out=%x", in, out)
		}
	}
}

func TestDecodeNormalizesAmbiguousChars(t *testing.T) {
	in := []byte("grind credit")
	enc := Encode(in)

	mangled := strings.ToLower(enc)
	mangled = strings.ReplaceAll(mangled, "0", "o")
	mangled = strings.ReplaceAll(mangled, "1", "i")

	out, err := Decode(mangled)
	if err != nil {
		t.Fatalf("Decode(%q): %v", mangled, err)
	}
	if !bytes.Equal(in, out) {
		t.Fatalf("normalized decode mismatch: in=%x out=%x", in, out)
	}
}
