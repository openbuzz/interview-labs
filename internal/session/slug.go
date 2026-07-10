package session

import (
	"crypto/rand"
	"math/big"

	petname "github.com/dustinkirkland/golang-petname"
)

const suffixAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// newSlug mints "<petname>-<4 base36 chars>", retrying while exists reports true.
func newSlug(exists func(string) bool) (string, error) {
	for {
		suffix := make([]byte, 4)
		for i := range suffix {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(suffixAlphabet))))
			if err != nil {
				return "", err
			}
			suffix[i] = suffixAlphabet[n.Int64()]
		}
		slug := petname.Generate(2, "-") + "-" + string(suffix)
		if !exists(slug) {
			return slug, nil
		}
	}
}
