package vercel

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// SolveChallenge takes a Vercel challenge token (_vcrct) and returns the solution.
func SolveChallenge(challengeToken string) (string, error) {
	parts := strings.Split(challengeToken, ".")
	if len(parts) < 4 {
		return "", fmt.Errorf("invalid challenge token: expected at least 4 parts, got %d", len(parts))
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(parts[3])
		if err != nil {
			// Try raw (no padding)
			decoded, err = base64.RawStdEncoding.DecodeString(parts[3])
			if err != nil {
				return "", fmt.Errorf("failed to decode challenge token part: %w", err)
			}
		}
	}

	decodedStr := string(decoded)
	fields := strings.Split(decodedStr, ";")
	if len(fields) < 4 {
		return "", fmt.Errorf("invalid decoded token: expected 4 fields (prefix;suffix;startHash;iterations), got %d in '%s'", len(fields), decodedStr)
	}

	suffix := fields[1]
	currentHash := fields[2]
	iterations := 0
	fmt.Sscanf(fields[3], "%d", &iterations)

	if iterations <= 0 || iterations > 100 {
		return "", fmt.Errorf("invalid iterations count: %d", iterations)
	}

	keys := make([]string, 0, iterations)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < iterations; i++ {
		key, hash, err := findMatchingKey(suffix, currentHash, r)
		if err != nil {
			return "", fmt.Errorf("failed at iteration %d: %w", i, err)
		}
		keys = append(keys, key)
		// Next required prefix is the tail of the found hash, same length as currentHash
		currentHash = hash[len(hash)-len(currentHash):]
	}

	return strings.Join(keys, ";"), nil
}

// findMatchingKey brute-forces a key such that sha256(suffix + key) starts with requiredPrefix.
func findMatchingKey(suffix, requiredPrefix string, r *rand.Rand) (string, string, error) {
	const maxAttempts = 10_000_000
	for attempt := 0; attempt < maxAttempts; attempt++ {
		key := randomString(r, 12)
		hash := sha256Hex(suffix + key)
		if strings.HasPrefix(hash, requiredPrefix) {
			return key, hash, nil
		}
	}
	return "", "", fmt.Errorf("failed to find matching key after %d attempts (prefix: %s)", maxAttempts, requiredPrefix)
}

func sha256Hex(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

func randomString(r *rand.Rand, length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}

// ExtractChallengeToken extracts the _vcrct value from the 429 HTML response body.
func ExtractChallengeToken(body string) (string, error) {
	// Look for window._vcrct="..."
	marker := `window._vcrct="`
	idx := strings.Index(body, marker)
	if idx == -1 {
		return "", fmt.Errorf("challenge token not found in response body")
	}

	start := idx + len(marker)
	end := strings.Index(body[start:], `"`)
	if end == -1 {
		return "", fmt.Errorf("malformed challenge token in response body")
	}

	return body[start : start+end], nil
}
