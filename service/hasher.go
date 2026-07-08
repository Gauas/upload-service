package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func hashFile(f *os.File) (string, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return "", fmt.Errorf("service: seek for hash: %w", err)
	}

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("service: hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
