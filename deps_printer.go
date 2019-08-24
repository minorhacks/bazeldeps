package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"gitlab.com/minorhacks/bazeldeps/bazel"
)

func gitCurrentCheckout() (string, error) {
	contents, err := ioutil.ReadFile(filepath.Join(os.Getenv("BUILD_WORKSPACE_DIRECTORY"), ".git", "HEAD"))
	if err != nil {
		return "", err
	}
	c := string(bytes.TrimSpace(contents))
	matches := regexp.MustCompile(`^ref: refs/heads/(.*)$`).FindStringSubmatch(c)
	if len(matches) < 2 {
		// Assuming this is a Git commit hash
		// TODO: don't make this assumption; actually validate this
		return c, nil
	}
	return matches[1], nil
}

func resolveCommitHash(commitish string) (string, error) {
	cmd := exec.Command("git", "rev-parse", commitish)
	cmd.Dir = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("can't resolve commitish %q: %v", commitish, err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func gitCheckout(commitish string) error {
	cmd := exec.Command("git", "checkout", commitish)
	cmd.Dir = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("can't checkout %q: %v", commitish, err)
	}
	return nil
}

func diff(last, cur map[string]uint32) map[string]uint32 {
	diffs := map[string]uint32{}
	for k, v := range cur {
		oldVal, ok := last[k]
		if !ok || oldVal != v {
			diffs[k] = v
		}
	}
	return diffs
}

func main() {
	flag.Parse()
	currentCommit, err := gitCurrentCheckout()
	exitIf(err)
	lastCommit, err := resolveCommitHash("HEAD~1")
	exitIf(err)

	err = gitCheckout(lastCommit)
	exitIf(err)
	lastHashes, err := bazel.CalcTargetHashes([]string{"//..."})
	exitIf(err)

	err = gitCheckout(currentCommit)
	exitIf(err)
	currentHashes, err := bazel.CalcTargetHashes([]string{"//..."})
	exitIf(err)

	diffs := diff(lastHashes, currentHashes)

	for n, h := range diffs {
		fmt.Printf("%s=%d\n", n, h)
	}
}

func exitIf(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
