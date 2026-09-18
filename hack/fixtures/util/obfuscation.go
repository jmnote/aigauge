package util

import (
	"fmt"
	"strings"
)

var obfuscatedKeys = map[string]struct{}{
	"accessToken":      {},
	"refreshToken":     {},
	"id_token":         {},
	"access_token":     {},
	"refresh_token":    {},
	"account_id":       {},
	"user_id":          {},
	"email":            {},
	"organizationUuid": {},
}

type seq struct {
	v, start, size int
	n              int
}

func (s *seq) next(v int) int {
	if s.n == 0 {
		s.v, s.start, s.n = v, v, 1
		return v
	}

	next := (s.v + 2) % s.size
	if next == s.start {
		next = (next + 1) % s.size
		s.start = next
	}

	s.v = next
	s.n++
	return next
}

func isHex(s string) bool {
	for _, r := range s {
		if !('0' <= r && r <= '9') &&
			!('a' <= r && r <= 'f') && r != '-' {
			return false
		}
	}
	return s != ""
}

func Obfuscate(x any) string {
	s := fmt.Sprintf("%v", x)

	digit := seq{size: 10}
	lower := seq{size: 26}
	upper := seq{size: 26}

	if isHex(s) {
		lower.size = 6
	}

	res := strings.Map(func(r rune) rune {
		switch {
		case '0' <= r && r <= '9':
			return '0' + rune(digit.next(int(r-'0')))
		case 'a' <= r && r <= 'z':
			return 'a' + rune(lower.next(int(r-'a')))
		case 'A' <= r && r <= 'Z':
			return 'A' + rune(upper.next(int(r-'A')))
		default:
			return r
		}
	}, s)

	runes := []rune(res)
	if len(runes) >= 20 {
		return string(runes[:len(runes)-7]) + "EXAMPLE"
	}
	return res
}

// ObfuscateFields recursively obfuscates sensitive values in JSON-like maps
// and arrays. It mutates maps and slices in place and returns the same value.
func ObfuscateFields(v any) any {
	switch value := v.(type) {
	case map[string]any:
		for key, child := range value {
			if _, ok := obfuscatedKeys[key]; ok && child != nil {
				value[key] = Obfuscate(child)
				continue
			}
			value[key] = ObfuscateFields(child)
		}
	case []any:
		for index, child := range value {
			value[index] = ObfuscateFields(child)
		}
	}
	return v
}
