// Package puzzle implements the argon2id-over-hashcash proof of work
// described in PLAN.md, "Choosing the puzzle" (M1: Argon2id-over-hashcash).
// A client looks for a nonce such that argon2id(challenge, nonce) has at
// least Difficulty leading zero bits. Argon2id's memory requirement caps
// how many attempts run in parallel per core, unlike SHA-256 hashcash.
//
// This package is pure Go with no network code, matching PLAN.md's M0 scope.
package puzzle

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"time"

	"golang.org/x/crypto/argon2"
)

// ChallengeSize is the length in bytes of a server-issued challenge nonce.
const ChallengeSize = 16

const hashSize = 32

// Params configures an Argon2id evaluation and the difficulty threshold a
// solution must meet. Field names match the argon2id parameters in the
// WWW-Authenticate descriptor in PLAN.md, "Wire format".
type Params struct {
	Memory     uint32 // m, in KiB
	Time       uint32 // t, number of passes
	Threads    uint8  // p
	Difficulty uint8  // d, required leading zero bits
}

// DefaultParams matches the calibration in PLAN.md, "Calibration, and the
// honest ceiling": 64 MiB, t=1, p=1, d=5, around 2 seconds on one core.
var DefaultParams = Params{Memory: 64 * 1024, Time: 1, Threads: 1, Difficulty: 5}

var (
	// ErrExpired is returned by Verify when the challenge's expiry has passed.
	ErrExpired = errors.New("puzzle: challenge expired")
	// ErrInvalidSolution is returned by Verify when the solution does not
	// meet the required difficulty.
	ErrInvalidSolution = errors.New("puzzle: solution does not meet difficulty")
)

// Challenge is a server-issued puzzle instance. The zero value is not
// valid; use NewChallenge.
type Challenge struct {
	Nonce  [ChallengeSize]byte
	Params Params
	Exp    time.Time
}

// NewChallenge generates a fresh, random challenge with the given params,
// expiring after ttl.
func NewChallenge(params Params, ttl time.Duration) (Challenge, error) {
	var nonce [ChallengeSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Challenge{}, err
	}
	return Challenge{Nonce: nonce, Params: params, Exp: time.Now().Add(ttl)}, nil
}

// Solution is the client-found value that satisfies a Challenge.
type Solution struct {
	Nonce uint64
}

// Solve brute-forces Solution.Nonce until argon2id(challenge, nonce) has
// Params.Difficulty leading zero bits, or ctx is cancelled. It does not
// check the challenge's expiry; callers should bound solve time with ctx.
func Solve(ctx context.Context, c Challenge) (Solution, error) {
	for n := uint64(0); ; n++ {
		select {
		case <-ctx.Done():
			return Solution{}, ctx.Err()
		default:
		}
		if hashMeetsDifficulty(c, n) {
			return Solution{Nonce: n}, nil
		}
	}
}

// Verify checks that sol solves c and that c had not expired at now.
func Verify(c Challenge, sol Solution, now time.Time) error {
	if now.After(c.Exp) {
		return ErrExpired
	}
	if !hashMeetsDifficulty(c, sol.Nonce) {
		return ErrInvalidSolution
	}
	return nil
}

func hashMeetsDifficulty(c Challenge, n uint64) bool {
	var password [8]byte
	binary.BigEndian.PutUint64(password[:], n)
	h := argon2.IDKey(password[:], c.Nonce[:], c.Params.Time, c.Params.Memory, c.Params.Threads, hashSize)
	return leadingZeroBits(h) >= int(c.Params.Difficulty)
}

func leadingZeroBits(b []byte) int {
	n := 0
	for _, by := range b {
		if by == 0 {
			n += 8
			continue
		}
		for i := 7; i >= 0; i-- {
			if by&(1<<uint(i)) != 0 {
				return n
			}
			n++
		}
	}
	return n
}
