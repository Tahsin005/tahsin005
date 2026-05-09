package vcs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	vcsDirName   = ".govcs"
	trackedFile  = "tracked.json"
	indexFile    = "index.json"
	objectsDir   = "objects"
	commitsDir   = "commits"
	headFileName = "HEAD"
)

type Repository struct {
	Root string
}

type Commit struct {
	ID        string            `json:"id"`
	Message   string            `json:"message"`
	Timestamp string            `json:"timestamp"`
	Previous  string            `json:"previous,omitempty"`
	Files     map[string]string `json:"files"`
}

func Init(path string) error {
	root, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	vcsRoot := filepath.Join(root, vcsDirName)
	if _, err := os.Stat(vcsRoot); err == nil {
		return errors.New("repository already initialized")
	}

	if err := os.MkdirAll(filepath.Join(vcsRoot, objectsDir), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(vcsRoot, commitsDir), 0o755); err != nil {
		return err
	}

	if err := writeJSON(filepath.Join(vcsRoot, trackedFile), []string{}); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(vcsRoot, indexFile), map[string]string{}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(vcsRoot, headFileName), []byte(""), 0o644); err != nil {
		return err
	}

	return nil
}

func OpenFrom(path string) (*Repository, error) {
	start, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	current := start
	for {
		vcsPath := filepath.Join(current, vcsDirName)
		if st, err := os.Stat(vcsPath); err == nil && st.IsDir() {
			return &Repository{Root: current}, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return nil, errors.New("not inside a govcs repository, run `govcs init` first")
		}
		current = parent
	}
}

func (r *Repository) Track(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, errors.New("no files provided to track")
	}

	tracked, err := r.loadTracked()
	if err != nil {
		return nil, err
	}

	existing := make(map[string]struct{}, len(tracked))
	for _, p := range tracked {
		existing[p] = struct{}{}
	}

	var added []string
	for _, p := range paths {
		rel, err := r.normalizePath(p)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(filepath.Join(r.Root, rel))
		if err != nil {
			return nil, fmt.Errorf("track %q: %w", rel, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("track %q: directories are not supported", rel)
		}

		if _, ok := existing[rel]; ok {
			continue
		}
		existing[rel] = struct{}{}
		tracked = append(tracked, rel)
		added = append(added, rel)
	}

	sort.Strings(tracked)
	if err := r.saveTracked(tracked); err != nil {
		return nil, err
	}

	return added, nil
}

func (r *Repository) Stage(paths []string) ([]string, error) {
	tracked, err := r.loadTracked()
	if err != nil {
		return nil, err
	}

	if len(tracked) == 0 {
		return nil, errors.New("no tracked files, run `govcs track <file>` first")
	}

	trackedSet := make(map[string]struct{}, len(tracked))
	for _, p := range tracked {
		trackedSet[p] = struct{}{}
	}

	var targets []string
	if len(paths) == 0 {
		targets = tracked
	} else {
		targets = make([]string, 0, len(paths))
		for _, p := range paths {
			rel, err := r.normalizePath(p)
			if err != nil {
				return nil, err
			}
			if _, ok := trackedSet[rel]; !ok {
				return nil, fmt.Errorf("file %q is not tracked", rel)
			}
			targets = append(targets, rel)
		}
	}

	index, err := r.loadIndex()
	if err != nil {
		return nil, err
	}

	var staged []string
	for _, rel := range targets {
		fullPath := filepath.Join(r.Root, rel)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("stage %q: %w", rel, err)
		}

		sum := sha256.Sum256(content)
		hash := hex.EncodeToString(sum[:])
		objectPath := filepath.Join(r.vcsPath(), objectsDir, hash)
		if _, err := os.Stat(objectPath); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(objectPath, content, 0o644); err != nil {
				return nil, err
			}
		}

		index[rel] = hash
		staged = append(staged, rel)
	}

	sort.Strings(staged)
	if err := r.saveIndex(index); err != nil {
		return nil, err
	}

	return staged, nil
}

func (r *Repository) Commit(message string) (*Commit, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, errors.New("commit message cannot be empty")
	}

	index, err := r.loadIndex()
	if err != nil {
		return nil, err
	}
	if len(index) == 0 {
		return nil, errors.New("nothing staged to commit")
	}

	previous := strings.TrimSpace(string(mustReadFile(filepath.Join(r.vcsPath(), headFileName))))
	now := time.Now().UTC().Format(time.RFC3339Nano)

	idRaw := fmt.Sprintf("%s|%s|%s|%d", previous, message, now, len(index))
	sum := sha256.Sum256([]byte(idRaw))
	commitID := hex.EncodeToString(sum[:])[:16]

	commit := &Commit{
		ID:        commitID,
		Message:   message,
		Timestamp: now,
		Previous:  previous,
		Files:     index,
	}

	commitPath := filepath.Join(r.vcsPath(), commitsDir, commit.ID+".json")
	if err := writeJSON(commitPath, commit); err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(r.vcsPath(), headFileName), []byte(commit.ID), 0o644); err != nil {
		return nil, err
	}
	if err := r.saveIndex(map[string]string{}); err != nil {
		return nil, err
	}

	return commit, nil
}

func (r *Repository) normalizePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path cannot be empty")
	}

	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(r.Root, path)
	}

	abs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(r.Root, abs)
	if err != nil {
		return "", err
	}

	rel = filepath.Clean(rel)
	if rel == "." {
		return "", errors.New("path must point to a file")
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", errors.New("path is outside repository")
	}
	if rel == vcsDirName || strings.HasPrefix(rel, vcsDirName+string(os.PathSeparator)) {
		return "", errors.New("cannot operate on internal govcs data")
	}

	return filepath.ToSlash(rel), nil
}

func (r *Repository) loadTracked() ([]string, error) {
	var tracked []string
	err := readJSON(filepath.Join(r.vcsPath(), trackedFile), &tracked)
	return tracked, err
}

func (r *Repository) saveTracked(tracked []string) error {
	return writeJSON(filepath.Join(r.vcsPath(), trackedFile), tracked)
}

func (r *Repository) loadIndex() (map[string]string, error) {
	index := map[string]string{}
	err := readJSON(filepath.Join(r.vcsPath(), indexFile), &index)
	return index, err
}

func (r *Repository) saveIndex(index map[string]string) error {
	return writeJSON(filepath.Join(r.vcsPath(), indexFile), index)
}

func (r *Repository) vcsPath() string {
	return filepath.Join(r.Root, vcsDirName)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func mustReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return []byte{}
	}
	return data
}
