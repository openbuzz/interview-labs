// Package stack owns the container-stack machinery: staging the embedded
// compose file and session secrets.
package stack

import (
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"

	interviewlabs "github.com/openbuzz/interview-labs"
)

// Stage materializes the compose file into dst. The VM pulls prebuilt
// images, so the compose file is the entire payload.
func Stage(dst string) error {
	data, err := fs.ReadFile(interviewlabs.DockerFS, "docker/compose.yaml")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dst, "compose.yaml"), data, 0o644)
}

// GeneratePassword mints the per-session gateway password: 16 hex chars,
// crypto/rand. Short-lived interview-gate material, not a durable secret.
func GeneratePassword() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
