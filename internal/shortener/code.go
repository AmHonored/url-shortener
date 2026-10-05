package shortener

import (
	"crypto/rand"
	"fmt"
)

const (
	CodePrefix    = "sg"
	codeRandomLen = 6
	CodeLen       = len(CodePrefix) + codeRandomLen

	base62        = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	unbiasedLimit = 256 / len(base62) * len(base62) // 248; skip bytes >= this for uniform distribution
)

// NewCode returns a random short code like "sgaB3dE9".
func NewCode() (string, error) {
	code := make([]byte, 0, CodeLen)
	code = append(code, CodePrefix...)

	var buf [codeRandomLen * 2]byte
	for len(code) < CodeLen {
		if _, err := rand.Read(buf[:]); err != nil {
			return "", fmt.Errorf("generate code: %w", err)
		}
		for _, b := range buf {
			if int(b) >= unbiasedLimit {
				continue
			}
			code = append(code, base62[int(b)%len(base62)])
			if len(code) == CodeLen {
				break
			}
		}
	}
	return string(code), nil
}
