package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"
)

// GetToday returns today's date in YYYY-MM-DD format.
func GetToday() string {
	return time.Now().Format("2006-01-02")
}

// GenerateID creates a unique clarification ID in format: clr-YYYYMMDD-xxxxxx
// where xxxxxx is a 6-character hex hash derived from the question content.
func GenerateID(question string) string {
	// Get today's date
	today := time.Now().Format("20060102")

	// Generate MD5 hash of question
	hash := md5.Sum([]byte(question))
	hashHex := hex.EncodeToString(hash[:])

	// Take first 6 characters of hash
	shortHash := hashHex[:6]

	return fmt.Sprintf("clr-%s-%s", today, shortHash)
}
