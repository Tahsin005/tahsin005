package vcs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesMetadata(t *testing.T) {
	root := t.TempDir()

	if err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	paths := []string{
		filepath.Join(root, ".govcs", "tracked.json"),
		filepath.Join(root, ".govcs", "index.json"),
		filepath.Join(root, ".govcs", "HEAD"),
		filepath.Join(root, ".govcs", "objects"),
		filepath.Join(root, ".govcs", "commits"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %s to exist: %v", p, err)
		}
	}
}

func TestTrackStageCommitFlow(t *testing.T) {
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	repo, err := OpenFrom(root)
	if err != nil {
		t.Fatalf("OpenFrom() error = %v", err)
	}

	file := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(file, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file error = %v", err)
	}

	added, err := repo.Track([]string{"hello.txt"})
	if err != nil {
		t.Fatalf("Track() error = %v", err)
	}
	if len(added) != 1 || added[0] != "hello.txt" {
		t.Fatalf("Track() added = %v, want [hello.txt]", added)
	}

	staged, err := repo.Stage(nil)
	if err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	if len(staged) != 1 || staged[0] != "hello.txt" {
		t.Fatalf("Stage() staged = %v, want [hello.txt]", staged)
	}

	commit, err := repo.Commit("initial commit")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if commit.ID == "" {
		t.Fatal("Commit() returned empty ID")
	}
	if commit.Message != "initial commit" {
		t.Fatalf("Commit() message = %q", commit.Message)
	}

	if _, ok := commit.Files["hello.txt"]; !ok {
		t.Fatalf("Commit() files = %v, expected hello.txt", commit.Files)
	}

	commitPath := filepath.Join(root, ".govcs", "commits", commit.ID+".json")
	if _, err := os.Stat(commitPath); err != nil {
		t.Fatalf("expected commit file at %s: %v", commitPath, err)
	}

	index, err := repo.loadIndex()
	if err != nil {
		t.Fatalf("loadIndex() error = %v", err)
	}
	if len(index) != 0 {
		t.Fatalf("index should be cleared after commit, got %v", index)
	}
}

func TestStageUntrackedFileFails(t *testing.T) {
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	repo, err := OpenFrom(root)
	if err != nil {
		t.Fatalf("OpenFrom() error = %v", err)
	}

	file := filepath.Join(root, "note.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file error = %v", err)
	}

	if _, err := repo.Stage([]string{"note.txt"}); err == nil {
		t.Fatal("expected Stage() to fail for untracked file")
	}
}
