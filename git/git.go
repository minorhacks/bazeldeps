package git

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/golang/glog"
)

// CurrentCheckout returns the current:
// * branch name, if a branch is checked out
// * commit hash, if in detached HEAD state
func CurrentCheckout() (string, error) {
	contents, err := ioutil.ReadFile(filepath.Join(os.Getenv("BUILD_WORKSPACE_DIRECTORY"), ".git", "HEAD"))
	if err != nil {
		return "", err
	}
	c := string(bytes.TrimSpace(contents))
	matches := regexp.MustCompile(`^ref: refs/heads/(.*)$`).FindStringSubmatch(c)
	if len(matches) < 2 {
		// Assuming this is a Git commit hash
		// TODO: don't make this assumption; actually validate this
		// See https://stackoverflow.com/a/19585361
		return c, nil
	}
	return matches[1], nil
}

// StashWithRestore stashes uncommitted changes and returns a restore func that
// will unstash said changes.
func StashWithRestore() (func(), error) {
	_, err := gitCommand("stash", "--include-untracked").Output()
	if err != nil {
		return func() {}, fmt.Errorf("can't stash: %v", err)
	}
	return func() {
		out, err := gitCommand("stash", "pop").CombinedOutput()
		if err != nil {
			switch string(bytes.TrimSpace(out)) {
			// Ignore the case where there are no local changes.
			case "No stash entries found.":
			default:
				glog.Exitf("failed to restore changed files: %v", err)
			}
		}
	}, nil
}

// CheckoutWithRestore performs a git checkout of the named committish and
// returns a restore func that checks out the repo to its original
// (branch/commit).
func CheckoutWithRestore(commitish string) (func(), error) {
	currentCheckout, err := CurrentCheckout()
	if err != nil {
		return nil, fmt.Errorf("can't save current checkout: %v", err)
	}
	err = gitCheckout(commitish)
	if err != nil {
		return nil, err
	}
	return func() {
		err := gitCheckout(currentCheckout)
		if err != nil {
			glog.Exitf("error while restoring to commit %s: %v", currentCheckout, err)
		}
	}, nil
}

func gitCheckout(commitish string) error {
	_, err := gitCommand("checkout", commitish).Output()
	if err != nil {
		return fmt.Errorf("can't checkout %q: %v", commitish, err)
	}
	return nil
}

// gitCommand returns a git command rooted in the bazel source workspace
// directory.
func gitCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	return cmd
}
