package accessgateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func newID(prefix string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
func newAccessKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "gw_sk_" + hex.EncodeToString(b), nil
}
func hashSecret(v string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(v)))
	return hex.EncodeToString(sum[:])
}
func secretPrefix(v string) string {
	if len(v) <= 12 {
		return v
	}
	return v[:12]
}
