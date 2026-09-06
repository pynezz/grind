package credit

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

func newTestCredit(t *testing.T) (SignedCredit, ed25519.PrivateKey, ed25519.PrivateKey) {
	t.Helper()
	mintPub, mintPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate mint key: %v", err)
	}
	clientPub, clientPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}

	c := WorkCredit{
		ClientPub: clientPub,
		Uses:      100,
		Exp:       time.Now().Add(time.Hour).Unix(),
	}
	copy(c.ID[:], []byte("0123456789abcdef"))

	sc, err := Mint(mintPriv, c)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if err := VerifyMint(mintPub, sc, time.Now()); err != nil {
		t.Fatalf("VerifyMint on freshly minted credit: %v", err)
	}
	return sc, mintPriv, clientPriv
}

func TestMintAndVerify(t *testing.T) {
	newTestCredit(t) // VerifyMint is exercised inline; failure fails the test.
}

func TestVerifyMintRejectsWrongKey(t *testing.T) {
	sc, _, _ := newTestCredit(t)
	otherPub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if err := VerifyMint(otherPub, sc, time.Now()); err != ErrBadMintSig {
		t.Fatalf("VerifyMint: got %v, want ErrBadMintSig", err)
	}
}

func TestVerifyMintRejectsTampering(t *testing.T) {
	sc, _, _ := newTestCredit(t)

	tampered := sc
	tampered.Credit.Uses = sc.Credit.Uses + 1000

	mintPub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if err := VerifyMint(mintPub, tampered, time.Now()); err == nil {
		t.Fatal("VerifyMint: expected error for tampered credit, got nil")
	}
}

func TestVerifyMintRejectsExpired(t *testing.T) {
	mintPub, mintPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate mint key: %v", err)
	}
	clientPub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	c := WorkCredit{ClientPub: clientPub, Uses: 1, Exp: time.Now().Add(-time.Minute).Unix()}
	sc, err := Mint(mintPriv, c)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if err := VerifyMint(mintPub, sc, time.Now()); err != ErrExpired {
		t.Fatalf("VerifyMint: got %v, want ErrExpired", err)
	}
}

func TestSignAndVerifyProof(t *testing.T) {
	sc, _, clientPriv := newTestCredit(t)
	nonce := []byte("request-nonce-1")

	sig := SignProof(clientPriv, sc.Credit.ID, nonce)
	if err := VerifyProof(sc.Credit.ClientPub, sc.Credit.ID, nonce, sig); err != nil {
		t.Fatalf("VerifyProof: %v", err)
	}
}

func TestVerifyProofRejectsWrongNonce(t *testing.T) {
	sc, _, clientPriv := newTestCredit(t)
	sig := SignProof(clientPriv, sc.Credit.ID, []byte("nonce-a"))
	if err := VerifyProof(sc.Credit.ClientPub, sc.Credit.ID, []byte("nonce-b"), sig); err != ErrBadProof {
		t.Fatalf("VerifyProof: got %v, want ErrBadProof", err)
	}
}

func TestVerifyProofRejectsWrongKey(t *testing.T) {
	sc, _, _ := newTestCredit(t)
	_, otherPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	nonce := []byte("request-nonce-1")
	// A signature from a key other than the one the credit is bound to must
	// not verify against the credit's pubkey: stealing the credit without
	// the private key buys nothing.
	sig := SignProof(otherPriv, sc.Credit.ID, nonce)
	if err := VerifyProof(sc.Credit.ClientPub, sc.Credit.ID, nonce, sig); err != ErrBadProof {
		t.Fatalf("VerifyProof: got %v, want ErrBadProof", err)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	sc, _, _ := newTestCredit(t)

	s, err := sc.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(s)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Credit.ID != sc.Credit.ID {
		t.Fatalf("ID mismatch: got %x want %x", got.Credit.ID, sc.Credit.ID)
	}
	if !bytes.Equal(got.Credit.ClientPub, sc.Credit.ClientPub) {
		t.Fatalf("ClientPub mismatch: got %x want %x", got.Credit.ClientPub, sc.Credit.ClientPub)
	}
	if got.Credit.Uses != sc.Credit.Uses {
		t.Fatalf("Uses mismatch: got %d want %d", got.Credit.Uses, sc.Credit.Uses)
	}
	if got.Credit.Exp != sc.Credit.Exp {
		t.Fatalf("Exp mismatch: got %d want %d", got.Credit.Exp, sc.Credit.Exp)
	}
	if !bytes.Equal(got.MintSig, sc.MintSig) {
		t.Fatalf("MintSig mismatch: got %x want %x", got.MintSig, sc.MintSig)
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	if _, err := Decode("not-a-valid-credit"); err == nil {
		t.Fatal("Decode: expected error for malformed input, got nil")
	}
}
