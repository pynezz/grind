// Package credit implements signing and verification for work credits: the
// non-transferable, key-bound capability minted by the work leg (see
// PLAN.md, "Two credit classes"). A WorkCredit is signed by the mint and
// bound to a client-generated pubkey; every request carries the credit plus
// a fresh signature over (credit_id, request_nonce), so stealing the credit
// without the private key buys nothing.
//
// The paid leg's bearer credits (blind-signed, transferable) belong to M5
// and are out of scope here.
package credit

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"time"

	"github.com/pynezz/grind/internal/crockford"
)

// IDSize is the length in bytes of a credit's identifier.
const IDSize = 16

const marshaledCreditSize = IDSize + ed25519.PublicKeySize + 4 + 8

var (
	ErrMalformedPubKey   = errors.New("credit: client pubkey has wrong length")
	ErrBadMintSig        = errors.New("credit: invalid mint signature")
	ErrExpired           = errors.New("credit: credit expired")
	ErrBadProof          = errors.New("credit: invalid client proof")
	ErrMalformedEncoding = errors.New("credit: malformed encoding")
)

// WorkCredit is the capability object described in PLAN.md as
// {credit_id, client_pubkey, uses, exp}.
type WorkCredit struct {
	ID        [IDSize]byte
	ClientPub ed25519.PublicKey
	Uses      uint32
	Exp       int64 // unix seconds
}

// Marshal produces the canonical byte encoding that the mint signature and
// verification are computed over.
func (c WorkCredit) Marshal() ([]byte, error) {
	if len(c.ClientPub) != ed25519.PublicKeySize {
		return nil, ErrMalformedPubKey
	}
	buf := make([]byte, marshaledCreditSize)
	off := 0
	off += copy(buf[off:], c.ID[:])
	off += copy(buf[off:], c.ClientPub)
	binary.BigEndian.PutUint32(buf[off:], c.Uses)
	off += 4
	binary.BigEndian.PutUint64(buf[off:], uint64(c.Exp))
	return buf, nil
}

func unmarshalCredit(buf []byte) (WorkCredit, error) {
	if len(buf) != marshaledCreditSize {
		return WorkCredit{}, ErrMalformedEncoding
	}
	var c WorkCredit
	off := 0
	copy(c.ID[:], buf[off:off+IDSize])
	off += IDSize
	c.ClientPub = append(ed25519.PublicKey{}, buf[off:off+ed25519.PublicKeySize]...)
	off += ed25519.PublicKeySize
	c.Uses = binary.BigEndian.Uint32(buf[off:])
	off += 4
	c.Exp = int64(binary.BigEndian.Uint64(buf[off:]))
	return c, nil
}

// SignedCredit is a WorkCredit plus the mint's signature over it.
type SignedCredit struct {
	Credit  WorkCredit
	MintSig []byte
}

// Mint signs c with the mint's private key, producing the SignedCredit a
// client presents at the gate.
func Mint(mintPriv ed25519.PrivateKey, c WorkCredit) (SignedCredit, error) {
	msg, err := c.Marshal()
	if err != nil {
		return SignedCredit{}, err
	}
	return SignedCredit{Credit: c, MintSig: ed25519.Sign(mintPriv, msg)}, nil
}

// VerifyMint checks that sc was signed by the holder of mintPub and has not
// expired at now. It does not check remaining uses; callers track that
// separately (a per-key counter, per PLAN.md's "Two credit classes").
func VerifyMint(mintPub ed25519.PublicKey, sc SignedCredit, now time.Time) error {
	msg, err := sc.Credit.Marshal()
	if err != nil {
		return err
	}
	if !ed25519.Verify(mintPub, msg, sc.MintSig) {
		return ErrBadMintSig
	}
	if now.After(time.Unix(sc.Credit.Exp, 0)) {
		return ErrExpired
	}
	return nil
}

// Encode renders sc as a Crockford base32 string for the X-Grind-Credit
// header (see PLAN.md, "Wire format").
func (sc SignedCredit) Encode() (string, error) {
	msg, err := sc.Credit.Marshal()
	if err != nil {
		return "", err
	}
	buf := append(msg, sc.MintSig...)
	return crockford.Encode(buf), nil
}

// Decode parses a SignedCredit from the Crockford base32 encoding Encode
// produces. It does not verify the mint signature; call VerifyMint on the
// result.
func Decode(s string) (SignedCredit, error) {
	buf, err := crockford.Decode(s)
	if err != nil {
		return SignedCredit{}, err
	}
	if len(buf) != marshaledCreditSize+ed25519.SignatureSize {
		return SignedCredit{}, ErrMalformedEncoding
	}
	credit, err := unmarshalCredit(buf[:marshaledCreditSize])
	if err != nil {
		return SignedCredit{}, err
	}
	sig := append([]byte{}, buf[marshaledCreditSize:]...)
	return SignedCredit{Credit: credit, MintSig: sig}, nil
}

// SignProof produces the per-request proof a client attaches to a request:
// a fresh signature over (credit_id, request_nonce), proving possession of
// the private key the credit is bound to.
func SignProof(clientPriv ed25519.PrivateKey, creditID [IDSize]byte, requestNonce []byte) []byte {
	return ed25519.Sign(clientPriv, proofMessage(creditID, requestNonce))
}

// VerifyProof checks a per-request proof against the pubkey a credit is
// bound to. It does not check uses or expiry; combine with VerifyMint and a
// caller-tracked use counter.
func VerifyProof(clientPub ed25519.PublicKey, creditID [IDSize]byte, requestNonce, sig []byte) error {
	if len(clientPub) != ed25519.PublicKeySize {
		return ErrMalformedPubKey
	}
	if !ed25519.Verify(clientPub, proofMessage(creditID, requestNonce), sig) {
		return ErrBadProof
	}
	return nil
}

func proofMessage(creditID [IDSize]byte, requestNonce []byte) []byte {
	msg := make([]byte, 0, IDSize+len(requestNonce))
	msg = append(msg, creditID[:]...)
	msg = append(msg, requestNonce...)
	return msg
}
