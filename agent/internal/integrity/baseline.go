// Package integrity provides workspace snapshot + diff for tampering
// detection. The composite GitHub Action calls `citadel-agent snapshot`
// immediately after `actions/checkout`, then `citadel-agent diff` at the
// end of the job — any changes are emitted as `file_tamper` events.
package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Snapshot walks rootDir and returns a map of relative-path → hex SHA256
// for every regular file. Symlinks, sockets, devices, and dirs are skipped.
// Hidden top-level dirs commonly used by tooling (.git, .github, node_modules)
// are NOT skipped — tampering can hide anywhere, and the cost is acceptable
// for the workspace sizes typical in CI.
func Snapshot(rootDir string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission errors etc. — skip but don't abort the whole walk.
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		h, err := hashFile(path)
		if err != nil {
			return nil // skip unreadable files
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes so the JSON baseline is portable.
		rel = filepath.ToSlash(rel)
		out[rel] = h
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", rootDir, err)
	}
	return out, nil
}

// hashFile computes the SHA256 of one regular file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileDiff is one entry in the diff between two snapshots.
type FileDiff struct {
	Path    string `json:"path"`
	OldHash string `json:"old_hash,omitempty"`
	NewHash string `json:"new_hash,omitempty"`
	Action  string `json:"action"` // "modified" | "added" | "deleted"
}

// Diff returns the set of files that changed between two snapshots.
// Output is stable-sorted by path for deterministic event ordering.
func Diff(before, after map[string]string) []FileDiff {
	var diffs []FileDiff
	for p, oldH := range before {
		newH, ok := after[p]
		if !ok {
			diffs = append(diffs, FileDiff{Path: p, OldHash: oldH, Action: "deleted"})
		} else if oldH != newH {
			diffs = append(diffs, FileDiff{Path: p, OldHash: oldH, NewHash: newH, Action: "modified"})
		}
	}
	for p, newH := range after {
		if _, ok := before[p]; !ok {
			diffs = append(diffs, FileDiff{Path: p, NewHash: newH, Action: "added"})
		}
	}
	// stable sort by path
	sortDiffs(diffs)
	return diffs
}

func sortDiffs(d []FileDiff) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && strings.Compare(d[j-1].Path, d[j].Path) > 0; j-- {
			d[j-1], d[j] = d[j], d[j-1]
		}
	}
}

// WriteJSON saves a snapshot to a JSON file. Use "-" for stdout.
func WriteJSON(path string, snap map[string]string) error {
	var w io.Writer
	if path == "-" {
		w = os.Stdout
	} else {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}

// ReadJSON loads a snapshot from a JSON file.
func ReadJSON(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out map[string]string
	if err := json.NewDecoder(f).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return out, nil
}

