package chshare

// Deterministic crypto.Reader
// overview: half the result is used as the output
// [a|...] -> sha512(a) -> [b|output] -> sha512(b)

import (
	"crypto/sha512"
	"io"
)

// DetermRandIter is the number of times a seed is hashed with SHA-512 to produce
// starting state of a pseudo-random stream
const DetermRandIter = 2048

// NewDetermRand creates an io.Reader that produces pseudo random bytes that are deterministic
// from a seed
func NewDetermRand(seed []byte) io.Reader {
	var out []byte
	//strengthen seed
	var next = seed
	for i := 0; i < DetermRandIter; i++ {
		next, out = hash(next)
	}
	return &DetermRand{
		next: next,
		out:  out,
	}
}

// DetermRand keeps running state for a pseudorandom byte stream
type DetermRand struct {
	next, out []byte
}

func (d *DetermRand) Read(b []byte) (int, error) {
	n := 0
	l := len(b)
	for n < l {
		next, out := hash(d.next)
		n += copy(b[n:], out)
		d.next = next
	}
	return n, nil
}

// hash computes an SHA-512 hash
func hash(input []byte) (next []byte, output []byte) {
	nextout := sha512.Sum512(input)
	return nextout[:sha512.Size/2], nextout[sha512.Size/2:]
}
