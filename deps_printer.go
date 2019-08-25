package main

import (
	"bytes"
	goflag "flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
	"gitlab.com/minorhacks/bazeldeps/bazel"
)

var (
	diffTarget = flag.String("diff_target", "local_changes",
		"Changes to compare for affected target. Options are:\n"+
			"  `local_changes`: Diffs local changes from last commit\n"+
			"  `last_commits`: Diffs last commit against its predecessor")
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

func gitCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	return cmd
}

func gitStashWithRestore() (func(), error) {
	_, err := gitCommand("stash").Output()
	if err != nil {
		return func() {}, fmt.Errorf("can't stash: %v", err)
	}
	return func() {
		_, err := gitCommand("stash", "pop").Output()
		if err != nil {
			glog.Exitf("failed to restore changed files: %v", err)
		}
	}, nil
}

func gitCheckoutWithRestore(commitish string) (func(), error) {
	currentCheckout, err := gitCurrentCheckout()
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
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()
	exitIf(checkFlags())

	var restore func()
	var err error
	switch *diffTarget {
	case "local_changes":
		restore, err = gitStashWithRestore()
	case "last_commits":
		restore, err = gitCheckoutWithRestore("HEAD~1")
	default:
		glog.Exit("--diff_target=%q is not implemented", *diffTarget)
	}
	exitIf(err)
	lastHashes, err := bazel.CalcTargetHashes([]string{"//..."})
	exitIf(err)
	restore()
	currentHashes, err := bazel.CalcTargetHashes([]string{"//..."})
	exitIf(err)

	diffs := diff(lastHashes, currentHashes)

	for n, h := range diffs {
		fmt.Printf("%s=%d\n", n, h)
	}
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}

func checkFlags() error {
	switch *diffTarget {
	case "local_changes":
	case "last_commits":
	default:
		return fmt.Errorf("--diff_target must be one of [local_changes, last_commits]")
	}
	return nil
}
