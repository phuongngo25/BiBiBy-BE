package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const BidxVersion = "v1"

// BlindIndex creates a non-reversible, search-friendly versioned hash of the input.
// It normalizes the input by removing Vietnamese accents and converting to lowercase.
func BlindIndex(input, hmacKey string) string {
	if input == "" {
		return ""
	}

	normalized := normalizeForSearch(input)
	
	h := hmac.New(sha256.New, []byte(hmacKey))
	h.Write([]byte(normalized))
	hash := hex.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("bidx:%s:%s", BidxVersion, hash)
}

// normalizeForSearch handles lowercasing and diacritic removal (accents).
func normalizeForSearch(s string) string {
	// 1. Convert to lowercase
	s = strings.ToLower(strings.TrimSpace(s))

	// 2. Handle specific Vietnamese characters like 'đ' which isn't handled by standard Mn removal
	s = strings.ReplaceAll(s, "đ", "d")

	// 3. Remove other diacritics (Accents)
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	
	return result
}
