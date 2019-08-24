package bazel

import (
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	bpb "gitlab.com/minorhacks/bazeldeps/proto/build_proto"
)

type TargetNode struct {
	Hash *uint32
	Deps []*TargetNode
	*bpb.Target
}

func sourcePathFromLabel(l string) string {
	switch {
	case strings.HasPrefix(l, "@"):
		l = strings.Replace(l, "@", filepath.Join("bazel-bazel_deps", "external")+"/", 1)
	default:
		l = strings.Replace(l, "//", "", -1)
	}
	return strings.Replace(l, ":", "/", -1)
}

func genPathFromLabel(l string) string {
	switch {
	case strings.HasPrefix(l, "@"):
		l = strings.Replace(l, "@", filepath.Join("bazel-bin", "external")+"/", 1)
	default:
		l = strings.Replace(l, "//", "bazel-bin/", -1)
	}
	return strings.Replace(l, ":", "/", -1)
}

func workspacePath(p string) string {
	return filepath.Join(os.Getenv("BUILD_WORKSPACE_DIRECTORY"), p)
}

func hashFile(h hash.Hash, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("can't open file %s: %v", path, err)
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("can't read file %s: %v", path, err)
	}
	return nil
}

func (n *TargetNode) GetHash() uint32 {
	if n.Hash != nil {
		return *n.Hash
	}
	h := fnv.New32()
	// TODO: sort deps?
	for _, dep := range n.Deps {
		fmt.Fprintf(h, "%d", dep.GetHash())
	}
	switch n.Target.GetType() {
	case bpb.Target_RULE:
	case bpb.Target_SOURCE_FILE:
		path := workspacePath(sourcePathFromLabel(n.Target.GetSourceFile().GetName()))
		if err := hashFile(h, path); err != nil {
			glog.Warning(err)
		}
	case bpb.Target_GENERATED_FILE:
		path := workspacePath(genPathFromLabel(n.Target.GetGeneratedFile().GetName()))
		if err := hashFile(h, path); err != nil {
			glog.Warning(err)
		}
	}
	n.Hash = new(uint32)
	*n.Hash = h.Sum32()
	return *n.Hash
}

func (n *TargetNode) GetName() string {
	switch n.Target.GetType() {
	case bpb.Target_RULE:
		return n.Target.GetRule().GetName()
	case bpb.Target_SOURCE_FILE:
		return n.Target.GetSourceFile().GetName()
	case bpb.Target_GENERATED_FILE:
		return n.Target.GetGeneratedFile().GetName()
	default:
		panic(fmt.Sprintf("can't get name for type %s", n.Target.GetType()))
	}
}

func CalcTargetHashes(universe []string) (map[string]uint32, error) {
	hashes := map[string]uint32{}
	targets, err := cqueryDeps(universe)
	if err != nil {
		return nil, fmt.Errorf("can't calculate target hashes: %v", err)
	}
	for _, t := range targets {
		hashes[t.GetName()] = t.GetHash()
	}
	return hashes, nil
}

func cqueryDeps(targets []string) ([]*TargetNode, error) {
	cmd := exec.Command("bazel", "cquery", fmt.Sprintf("deps(%s)", strings.Join(targets, ", ")), "--output=textproto")
	cmd.Dir = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error running 'bazel cquery': %v", err)
	}

	var results bpb.QueryResults
	if err := proto.UnmarshalText(string(output), &results); err != nil {
		return nil, fmt.Errorf("error unmarshaling `bazel cquery` output: %v", err)
	}

	var targetNodes []*TargetNode
	for _, r := range results.Results {
		for _, t := range r.Target {
			targetNodes = append(targetNodes, &TargetNode{Target: t})
		}
	}

	return targetNodes, nil
}
