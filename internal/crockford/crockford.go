// Package crockford implements Crockford's base32 variant: no padding,
// case-insensitive decoding, and O/I/L normalized to 0/1. This is the token
// encoding already used by pynezz-hdauth and uptime; grind credits reuse it
// so the token vocabulary across pynezz.dev stays uniform (see PLAN.md,
// "Wire format").
package crockford

import (
	"encoding/base32"
	"strings"
)

const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var encoding = base32.NewEncoding(alphabet).WithPadding(base32.NoPadding)

var normalizer = strings.NewReplacer(
	"O", "0",
	"I", "1",
	"L", "1",
	"-", "",
	" ", "",
)

// Encode returns the Crockford base32 encoding of b.
func Encode(b []byte) string {
	return encoding.EncodeToString(b)
}

// Decode parses a Crockford base32 string, tolerating the case-insensitivity
// and O/I/L ambiguity the encoding is designed to survive.
func Decode(s string) ([]byte, error) {
	s = normalizer.Replace(strings.ToUpper(s))
	return encoding.DecodeString(s)
}
