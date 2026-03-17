package vercel

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// SolveChallenge takes a Vercel v2 challenge token and returns the solution.
// Token format: 2.{timestamp}.{ttl}.{base64_payload}.{signature}
// Payload (semicolon-separated): {project_id};{suffix};{start_hash};{iterations};{binary}
// Solution: {key1};{key2};...{keyN} where each key is 16 hex chars
func SolveChallenge(challengeToken string) (string, error) {
	tokenParts := strings.SplitN(challengeToken, ".", 5)
	if len(tokenParts) < 5 {
		return "", fmt.Errorf("invalid challenge token: expected 5 dot-separated parts, got %d", len(tokenParts))
	}

	b64Payload := tokenParts[3]
	decoded, err := base64.StdEncoding.DecodeString(b64Payload)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(b64Payload)
		if err != nil {
			return "", fmt.Errorf("failed to decode challenge payload: %w", err)
		}
	}

	// Split on semicolons - first 4 fields are text, 5th is binary
	fields := strings.SplitN(string(decoded), ";", 5)
	if len(fields) < 4 {
		return "", fmt.Errorf("invalid decoded payload: expected at least 4 fields, got %d", len(fields))
	}

	suffix := fields[1]
	startHash := fields[2]
	iterations := 0
	fmt.Sscanf(fields[3], "%d", &iterations)

	if iterations <= 0 || iterations > 100 {
		return "", fmt.Errorf("invalid iterations count: %d", iterations)
	}

	// The prefix length to match -- use the suffix length as a hint
	// The suffix is typically 8 hex chars, and the prefix to match is also 8 chars
	prefixLen := len(suffix)
	if prefixLen > len(startHash) {
		prefixLen = len(startHash)
	}

	currentPrefix := startHash[:prefixLen]

	keys := make([]string, 0, iterations)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < iterations; i++ {
		key, hash, err := findMatchingKey(suffix, currentPrefix, r)
		if err != nil {
			return "", fmt.Errorf("failed at iteration %d: %w", i, err)
		}
		keys = append(keys, key)
		// Next prefix is from the tail of the hash
		currentPrefix = hash[len(hash)-prefixLen:]
	}

	return strings.Join(keys, ";"), nil
}

// findMatchingKey brute-forces a random 8-byte key (16 hex chars) such that
// sha1(suffix + key_hex) starts with requiredPrefix.
func findMatchingKey(suffix, requiredPrefix string, r *rand.Rand) (string, string, error) {
	const maxAttempts = 50_000_000
	for attempt := 0; attempt < maxAttempts; attempt++ {
		key := randomHex(r, 16)
		hash := sha1Hex(suffix + key)
		if strings.HasPrefix(hash, requiredPrefix) {
			return key, hash, nil
		}
	}
	return "", "", fmt.Errorf("failed to find matching key after %d attempts (prefix len: %d)", maxAttempts, len(requiredPrefix))
}

func sha1Hex(input string) string {
	h := sha1.Sum([]byte(input))
	return hex.EncodeToString(h[:])
}

func randomHex(r *rand.Rand, length int) string {
	const hexChars = "0123456789abcdef"
	b := make([]byte, length)
	for i := range b {
		b[i] = hexChars[r.Intn(16)]
	}
	return string(b)
}

// ExtractChallengeToken extracts the challenge token from the 429 HTML response.
// It looks for the token in various formats used by Vercel's challenge pages.
func ExtractChallengeToken(body string) (string, error) {
	// Try v2: look for data attribute or JS variable containing the token
	markers := []string{
		`window._vcrct="`,
		`data-challenge="`,
		`"challengeToken":"`,
		`challenge-token" content="`,
	}

	for _, marker := range markers {
		idx := strings.Index(body, marker)
		if idx != -1 {
			start := idx + len(marker)
			end := strings.Index(body[start:], `"`)
			if end != -1 {
				return body[start : start+end], nil
			}
		}
	}

	return "", fmt.Errorf("challenge token not found in response body")
}
