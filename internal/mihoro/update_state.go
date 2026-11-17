package mihoro

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"mihoro-go/internal/utils"
)

// updateState records what was installed so updates can skip unchanged items.
// It never contains secrets.
type updateState struct {
	UI map[string]string `toml:"ui,omitempty"`
}

func newUpdateState() *updateState {
	return &updateState{UI: map[string]string{}}
}

// updateStatePath returns the path of the local update state file.
func updateStatePath(mihoroDir string) string {
	return filepath.Join(mihoroDir, ".update-state.toml")
}

// loadUpdateState loads the state file. A missing file yields an empty state.
func loadUpdateState(mihoroDir string) (*updateState, error) {
	st := newUpdateState()
	data, err := os.ReadFile(updateStatePath(mihoroDir))
	if os.IsNotExist(err) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("read update state: %w", err)
	}
	if err := toml.Unmarshal(data, st); err != nil {
		return st, fmt.Errorf("parse update state: %w", err)
	}
	if st.UI == nil {
		st.UI = map[string]string{}
	}
	return st, nil
}

// saveUpdateState writes the state file atomically.
func saveUpdateState(mihoroDir string, st *updateState) error {
	var b strings.Builder
	if err := toml.NewEncoder(&b).Encode(st); err != nil {
		return fmt.Errorf("encode update state: %w", err)
	}
	if err := utils.WriteFileAtomic(updateStatePath(mihoroDir), []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("write update state: %w", err)
	}
	return nil
}

// fileSHA256 returns the lowercase hex SHA-256 of path.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// digestHex normalizes a GitHub asset digest ("sha256:<hex>") to plain hex.
func digestHex(digest string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(digest), "sha256:"))
}
