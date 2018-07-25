// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package paillier

import (
	"errors"
	"io"
	"math/big"
)

// smallPrimes is a list of small, prime numbers that allows us to rapidly
// exclude some fraction of composite candidates when searching for a random
// prime. This list is truncated at the point where smallPrimesProduct exceeds
// a uint64. It does not include two because we ensure that the candidates are
// odd by construction.
var smallPrimes = []uint8{
	3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53,
}

// smallPrimesProduct is the product of the values in smallPrimes and allows us
// to reduce a candidate prime by this number and then determine whether it's
// coprime to all the elements of smallPrimes without further big.Int
// operations.
var smallPrimesProduct = new(big.Int).SetUint64(16294579238595022365)

// Prime returns a number, p, of the given size, such that p is prime
// with high probability.
// Prime will return error for any error returned by rand.Read or if bits < 2.
func SafePrimes(rand io.Reader, bits int) (p *big.Int, q *big.Int, err error) {
	qbits := bits - 1

	if qbits < 2 {
		err = errors.New("crypto/rand: prime size must be at least 2-bit")
		return
	}

	b := uint(qbits % 8)
	if b == 0 {
		b = 8
	}

	bytes := make([]byte, (qbits+7)/8)
	p = new(big.Int)
	q = new(big.Int)

	bigMod := new(big.Int)

	//NextRand:
	for {
		_, err = io.ReadFull(rand, bytes)
		if err != nil {
			return nil, nil, err
		}

		// Clear bits in the first byte to make sure the candidate has a size <= bits.
		bytes[0] &= uint8(int(1<<b) - 1)
		// Don't let the value be too small, i.e, set the most significant two bits.
		// Setting the top two bits, rather than just the top bit,
		// means that when two of these values are multiplied together,
		// the result isn't ever one bit short.
		if b >= 2 {
			bytes[0] |= 3 << (b - 2)
		} else {
			// Here b==1, because b cannot be zero.
			bytes[0] |= 1
			if len(bytes) > 1 {
				bytes[1] |= 0x80
			}
		}
		// Make the value odd since an even number this large certainly isn't prime.
		bytes[len(bytes)-1] |= 1

		q.SetBytes(bytes)

		p.Mul(q, big.NewInt(2))
		p.Add(p, big.NewInt(1))

		// Calculate the value mod the product of smallPrimes. If it's
		// a multiple of any of these primes we add two until it isn't.
		// The probability of overflowing is minimal and can be ignored
		// because we still perform Miller-Rabin tests on the result.
		bigMod.Mod(q, smallPrimesProduct)
		mod := bigMod.Uint64()

	NextDelta:
		for delta := uint64(0); delta < 1<<20; delta += 2 {
			m := mod + delta
			for _, prime := range smallPrimes {
				if m%uint64(prime) == 0 && (qbits > 6 || m != uint64(prime)) {
					continue NextDelta
				}
			}

			if delta > 0 {
				bigMod.SetUint64(delta)
				q.Add(q, bigMod)
			}

			qMod3 := new(big.Int).Mod(q, big.NewInt(3))
			if qMod3.Cmp(big.NewInt(1)) == 0 {
				continue NextDelta
			}

			p.Mul(q, big.NewInt(2))
			p.Add(p, big.NewInt(1))
			if !isPrimeCandidate(p) {
				continue NextDelta
			}

			break
		}

		// There is a tiny possibility that, by adding delta, we caused
		// the number to be one bit too long. Thus we check BitLen
		// here.
		if q.ProbablyPrime(20) &&
			isPocklingtonCriterionSatisfied(p) &&
			q.BitLen() == qbits {
			return
		}
	}
}

func isPocklingtonCriterionSatisfied(p *big.Int) bool {
	return new(big.Int).Exp(
		big.NewInt(2),
		new(big.Int).Sub(p, big.NewInt(1)),
		p,
	).Cmp(big.NewInt(1)) == 0
}

func isPrimeCandidate(number *big.Int) bool {
	bigMod := new(big.Int)
	bigMod.Mod(number, smallPrimesProduct)
	m := bigMod.Uint64()

	for _, prime := range smallPrimes {
		if m%uint64(prime) == 0 && m != uint64(prime) {
			return false
		}
	}

	return true
}
