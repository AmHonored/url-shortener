package shortener

import (
	"crypto/rand"
	"fmt"
)

const (
	// CodePrefix tags every generated code ("sg" = sysgrp).
	CodePrefix = "sg"
	// codeRandomLen is the number of random characters after the prefix.
	// 62^6 ≈ 56.8 billion possible codes.
	codeRandomLen = 6
	// CodeLen is the total length of a generated code (8, within the 6–8 rule).
	CodeLen = len(CodePrefix) + codeRandomLen

	base62 = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// unbiasedLimit is the largest multiple of 62 that fits in a byte (248).
	// Random bytes >= it are skipped so that byte % 62 is uniform.
	unbiasedLimit = 256 / len(base62) * len(base62)
)

// NewCode returns a new random short code: CodePrefix followed by
// codeRandomLen base62 characters from crypto/rand, e.g. "sgaB3dE9".
// Codes are unpredictable, so links cannot be enumerated.
func NewCode() (string, error) {
	code := make([]byte, 0, CodeLen)
	code = append(code, CodePrefix...)

	var buf [codeRandomLen * 2]byte // extra bytes so one read usually suffices
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
