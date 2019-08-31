package main

import (
	goflag "flag"
	"fmt"

	"gitlab.com/minorhacks/bazeldeps/bazel"
	"gitlab.com/minorhacks/bazeldeps/git"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

var (
	diffTarget = flag.String("diff_target", "local_changes",
		"Changes to compare for affected target. Options are:\n"+
			"  `local_changes`: Diffs local changes from last commit\n"+
			"  `last_commits`: Diffs last commit against its predecessor")
)

// diff returns entries in cur that are either new or different than those in
// last. Missing entries in cur are ignored.
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
		restore, err = git.StashWithRestore()
	case "last_commits":
		restore, err = git.CheckoutWithRestore("HEAD~1")
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

	if len(diffs) == 0 {
		fmt.Println("No changes detected")
	} else {
		for n, h := range diffs {
			fmt.Printf("%s=%d\n", n, h)
		}
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
