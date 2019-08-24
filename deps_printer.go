package main

import (
	"flag"
	"fmt"
	"os"

	"gitlab.com/minorhacks/bazeldeps/bazel"
)

func main() {
	flag.Parse()
	hashes, err := bazel.CalcTargetHashes([]string{"//..."})
	exitIf(err)
	for n, h := range hashes {
		fmt.Printf("%s=%d\n", n, h)
	}
	//fmt.Print(hashes)
}

func exitIf(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
