package puzzle

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// fastParams keeps Argon2id memory low enough for tests to run quickly.
// DefaultParams (64 MiB) is calibrated for production difficulty, not test
// iteration speed.
var fastParams = Params{Memory: 8 * 1024, Time: 1, Threads: 1, Difficulty: 8}

func TestSolveAndVerify(t *testing.T) {
	tests := []struct {
		name       string
		difficulty uint8
	}{
		{"difficulty 1", 1},
		{"difficulty 4", 4},
		{"difficulty 8", 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := fastParams
			params.Difficulty = tt.difficulty
			c, err := NewChallenge(params, time.Minute)
			if err != nil {
				t.Fatalf("NewChallenge: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			sol, err := Solve(ctx, c)
			if err != nil {
				t.Fatalf("Solve: %v", err)
			}

			if err := Verify(c, sol, time.Now()); err != nil {
				t.Fatalf("Verify: %v", err)
			}
		})
	}
}

func TestVerifyRejectsWrongSolution(t *testing.T) {
	c, err := NewChallenge(fastParams, time.Minute)
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	// An arbitrary nonce is astronomically unlikely to meet the difficulty.
	if err := Verify(c, Solution{Nonce: 12345}, time.Now()); err == nil {
		t.Fatal("Verify: expected error for an unsolved nonce, got nil")
	}
}

func TestVerifyRejectsExpiredChallenge(t *testing.T) {
	params := fastParams
	params.Difficulty = 0 // any nonce satisfies this, isolating the expiry check
	c, err := NewChallenge(params, -time.Second)
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	if err := Verify(c, Solution{Nonce: 0}, time.Now()); err != ErrExpired {
		t.Fatalf("Verify: got %v, want ErrExpired", err)
	}
}

func TestSolveRespectsContextCancellation(t *testing.T) {
	params := fastParams
	params.Difficulty = 255 // effectively unsolvable
	c, err := NewChallenge(params, time.Minute)
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := Solve(ctx, c); err != context.DeadlineExceeded {
		t.Fatalf("Solve: got %v, want context.DeadlineExceeded", err)
	}
}

// BenchmarkSolve demonstrates cost scaling with difficulty: each additional
// bit should roughly double the average number of Argon2id evaluations, per
// PLAN.md's calibration table. Memory is kept small (relative to
// DefaultParams) so the benchmark completes in a reasonable time; only the
// *relative* scaling across difficulties is meaningful here, not the
// absolute times.
func BenchmarkSolve(b *testing.B) {
	base := Params{Memory: 4 * 1024, Time: 1, Threads: 1}
	for _, d := range []uint8{4, 6, 8, 10} {
		params := base
		params.Difficulty = d
		b.Run(fmt.Sprintf("d=%d", d), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c, err := NewChallenge(params, time.Hour)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := Solve(context.Background(), c); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
