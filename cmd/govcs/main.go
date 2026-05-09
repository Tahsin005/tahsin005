package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tahsin005/tahsin005/internal/vcs"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "track":
		runTrack(os.Args[2:])
	case "stage":
		runStage(os.Args[2:])
	case "commit":
		runCommit(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func runInit(args []string) {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	if err := vcs.Init(target); err != nil {
		exitWithErr(err)
	}
	abs, _ := filepath.Abs(target)
	fmt.Printf("Initialized govcs repository in %s/.govcs\n", abs)
}

func runTrack(args []string) {
	if len(args) == 0 {
		exitWithErr(fmt.Errorf("usage: govcs track <file> [file ...]"))
	}
	repo := openRepoFromCWD()
	added, err := repo.Track(args)
	if err != nil {
		exitWithErr(err)
	}
	if len(added) == 0 {
		fmt.Println("No new files tracked.")
		return
	}
	fmt.Printf("Tracked: %s\n", strings.Join(added, ", "))
}

func runStage(args []string) {
	repo := openRepoFromCWD()
	staged, err := repo.Stage(args)
	if err != nil {
		exitWithErr(err)
	}
	fmt.Printf("Staged: %s\n", strings.Join(staged, ", "))
}

func runCommit(args []string) {
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	message := fs.String("m", "", "commit message")
	if err := fs.Parse(args); err != nil {
		exitWithErr(err)
	}

	repo := openRepoFromCWD()
	commit, err := repo.Commit(*message)
	if err != nil {
		exitWithErr(err)
	}
	fmt.Printf("[%s] %s\n", commit.ID, commit.Message)
}

func openRepoFromCWD() *vcs.Repository {
	wd, err := os.Getwd()
	if err != nil {
		exitWithErr(err)
	}
	repo, err := vcs.OpenFrom(wd)
	if err != nil {
		exitWithErr(err)
	}
	return repo
}

func exitWithErr(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func usage() {
	fmt.Println("govcs - tiny git-like VCS")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  govcs init [path]")
	fmt.Println("  govcs track <file> [file ...]")
	fmt.Println("  govcs stage [file ...]")
	fmt.Println("  govcs commit -m \"message\"")
}
