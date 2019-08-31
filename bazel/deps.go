package bazel

import (
	"bytes"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

	bpb "gitlab.com/minorhacks/bazeldeps/proto/build_proto"
)

var (
	// Path to bazel execution_root - equivalent to `bazel-$WORKSPACE` symlink
	executionRoot string
	// Absolute path to bazel workspace
	WorkspaceRoot string
	// Absolute path to bazel-bin
	bazelBinRoot string
)

func init() {
	var err error

	WorkspaceRoot = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if WorkspaceRoot == "" {
		panic("$BUILD_WORKSPACE_DIRECTORY is unset; is this run using `bazel run`?")
	}

	executionRoot, err = findExecutionRoot()
	if err != nil {
		panic(err)
	}

	bazelBinRoot, err = findBazelBin()
	if err != nil {
		panic(err)
	}
}

// TargetNode wraps a Target proto message with a hash code and a list of
// dependent TargetNodes.
type TargetNode struct {
	Hash *uint32
	Deps []*TargetNode
	*bpb.Target
}

func bazelCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("bazel", args...)
	cmd.Dir = WorkspaceRoot
	return cmd
}

func findExecutionRoot() (string, error) {
	out, err := bazelCommand("info", "execution_root").Output()
	if err != nil {
		return "", fmt.Errorf("failed to find bazel execution_root: %v", err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func findBazelBin() (string, error) {
	// `bazel info` could be used here, if we knew enough to pass the right flags
	// to it (e.g. `-c opt`) otherwise, we'll get the wrong dir. For now, just
	// use the bazel-bin symlink.
	return filepath.Join(WorkspaceRoot, "bazel-bin"), nil
}

// sourcePathFromLabel attempts to construct an absolute repo source path from
// a label.
func sourcePathFromLabel(l string) string {
	switch {
	case strings.HasPrefix(l, "@"):
		l = strings.Replace(l, "@", filepath.Join(executionRoot, "external")+"/", 1)
	default:
		l = strings.Replace(l, "//", WorkspaceRoot+"/", -1)
	}
	return strings.Replace(l, ":", "/", -1)
}

// genPathFromLabel attempts to construct an absolute bazel-bin/ generated file
// path from a label.
func genPathFromLabel(l string) string {
	switch {
	case strings.HasPrefix(l, "@"):
		l = strings.Replace(l, "@", filepath.Join(bazelBinRoot, "external")+"/", 1)
	default:
		l = strings.Replace(l, "//", bazelBinRoot+"/", -1)
	}
	return strings.Replace(l, ":", "/", -1)
}

// workspacePath returns an absolute path from a path p relative to the
// workspace root.
func workspacePath(p string) string {
	return filepath.Join(WorkspaceRoot, p)
}

// hashFile adds the contents of a file at path `path` to the provided
// hash.Hash.
func hashFile(h hash.Hash, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("can't open file for hashing: %v", err)
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("can't read file for hashing: %v", err)
	}
	return nil
}

func attrValue(attr *bpb.Attribute) string {
	switch attr.GetType() {
	case bpb.Attribute_INTEGER:
		return strconv.FormatInt(int64(attr.GetIntValue()), 10)

	case bpb.Attribute_INTEGER_LIST:
		var s []string
		for _, i := range attr.GetIntListValue() {
			s = append(s, strconv.FormatInt(int64(i), 10))
		}
		// Assume that order matters here, so don't sort the strings
		return strings.Join(s, ",")

	case bpb.Attribute_BOOLEAN:
		return strconv.FormatBool(attr.GetBooleanValue())

	case bpb.Attribute_TRISTATE:
		return attr.GetTristateValue().String()

	case bpb.Attribute_STRING,
		bpb.Attribute_LABEL,
		bpb.Attribute_OUTPUT:
		return attr.GetStringValue()

	case bpb.Attribute_STRING_LIST,
		bpb.Attribute_LABEL_LIST,
		bpb.Attribute_OUTPUT_LIST,
		bpb.Attribute_DISTRIBUTION_SET:
		val := attr.GetStringListValue()
		// Assume that order matters here, so don't sort the strings
		return strings.Join(val, ",")

	case bpb.Attribute_STRING_DICT:
		val := attr.GetStringDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		return strings.Join(sort.StringSlice(pairs), ",")

	case bpb.Attribute_LABEL_DICT_UNARY:
		val := attr.GetLabelDictUnaryValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		return strings.Join(sort.StringSlice(pairs), ",")

	case bpb.Attribute_LABEL_LIST_DICT:
		val := attr.GetLabelListDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+strings.Join(entry.GetValue(), ":"))
		}
		return strings.Join(sort.StringSlice(pairs), ",")

	case bpb.Attribute_LABEL_KEYED_STRING_DICT:
		val := attr.GetLabelKeyedStringDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+entry.GetValue())
		}
		return strings.Join(sort.StringSlice(pairs), ",")

	case bpb.Attribute_STRING_LIST_DICT:
		val := attr.GetStringListDictValue()
		var pairs []string
		for _, entry := range val {
			pairs = append(pairs, entry.GetKey()+"="+strings.Join(entry.GetValue(), ":"))
		}
		return strings.Join(sort.StringSlice(pairs), ",")

	case bpb.Attribute_LICENSE:
		// License changes shouldn't trigger a rebuild; don't include in the hash
		return ""
	default:
		// TODO: Determine how to handle these cases
		//case bpb.Attribute_FILESET_ENTRY_LIST:
		//case bpb.Attribute_UNKNOWN:
		//case bpb.Attribute_SELECTOR_LIST:
		//case bpb.Attribute_DEPRECATED_STRING_DICT_UNARY:
		panic(fmt.Sprintf("unsupported attribute type: %v", attr.GetType()))
	}
}

// GetHash returns a hash for the given TargetNode. If this hash changes, it is
// assumed that the corresponding Target was affected by some change. Targets
// are affected if source file contents change, or if rules' attributes or
// dependent targets change. Since dependent target changes are detected by
// changes in their respective hashes, GetHash will call GetHash on
// dependencies recursively down the build graph.
func (n *TargetNode) GetHash() uint32 {
	// Memoize this target's hash so it doesn't need to be recalculated
	if n.Hash != nil {
		return *n.Hash
	}
	h := fnv.New32()
	// Assume deps are sorted when created
	for _, dep := range n.Deps {
		fmt.Fprintf(h, "%d", dep.GetHash())
	}
	switch n.Target.GetType() {
	case bpb.Target_RULE:
		// Add rule attribute contents to hash
		attrList := n.Target.GetRule().GetAttribute()
		// Sort the attributes by name so they are added to the hash in a stable
		// order
		sort.Slice(attrList, func(i, j int) bool { return attrList[i].GetName() < attrList[j].GetName() })
		for _, attr := range attrList {
			fmt.Fprintf(h, "%s=%s", attr.GetName(), attrValue(attr))
		}
	case bpb.Target_SOURCE_FILE:
		path := sourcePathFromLabel(n.Target.GetSourceFile().GetName())
		if err := hashFile(h, path); err != nil {
			// TODO: This seems to happen with third-party deps that aren't actually
			// dependencies and therefore aren't downloaded?
			glog.Warning(err)
		}
	case bpb.Target_GENERATED_FILE:
		path := genPathFromLabel(n.Target.GetGeneratedFile().GetName())
		if err := hashFile(h, path); err != nil {
			glog.Warning(err)
		}
	}
	n.Hash = new(uint32)
	*n.Hash = h.Sum32()
	return *n.Hash
}

// GetName fetches the target name, which is in one of a few embedded messages
// depending on the target type.
func (n *TargetNode) GetName() string {
	switch n.Target.GetType() {
	case bpb.Target_RULE:
		return n.Target.GetRule().GetName()
	case bpb.Target_SOURCE_FILE:
		return n.Target.GetSourceFile().GetName()
	case bpb.Target_GENERATED_FILE:
		return n.Target.GetGeneratedFile().GetName()
	default:
		// TODO: do something more sane then panic here
		panic(fmt.Sprintf("can't get name for type %s", n.Target.GetType()))
	}
}

func (n *TargetNode) GetDeps() []string {
	switch n.Target.GetType() {
	case bpb.Target_RULE:
		return n.Target.GetRule().GetRuleInput()
	}
	return nil
}

func (n *TargetNode) String() string {
	return fmt.Sprintf("Hash:%d Deps:%v Target:%+v", n.Hash, n.Deps, n.Target)
}

// CalcTargetHashes returns a map of (target name) -> (target hash) for all
// dependencies of the named universe (such as `//...`). If the hash changes
// for a given target, it is assumed that the target was affected by some
// change.
func CalcTargetHashes(universe []string) (map[string]uint32, error) {
	hashes := map[string]uint32{}
	targets, err := cqueryDeps(universe)
	if err != nil {
		return nil, fmt.Errorf("can't calculate target hashes: %v", err)
	}
	for name, target := range targets {
		hashes[name] = target.GetHash()
	}

	return hashes, nil
}

// cqueryDeps performs a `bazel cquery` to dump information for `targets` and
// their transitive dependencies. The output data is parsed into a map of
// target names to target data nodes, where the nodes are connected into a
// graph by their dependencies.
func cqueryDeps(targets []string) (map[string]*TargetNode, error) {
	output, err := bazelCommand("cquery", fmt.Sprintf("deps(%s)", strings.Join(targets, ", ")), "--output=textproto").Output()
	if err != nil {
		return nil, fmt.Errorf("error running 'bazel cquery': %v", err)
	}

	var results bpb.QueryResults
	if err := proto.UnmarshalText(string(output), &results); err != nil {
		return nil, fmt.Errorf("error unmarshaling `bazel cquery` output: %v", err)
	}

	// Parse proto results into a map of (target name) -> (target data)
	targetNodes := map[string]*TargetNode{}
	for _, r := range results.Results {
		for _, t := range r.Target {
			newNode := &TargetNode{Target: t}
			targetNodes[newNode.GetName()] = newNode
		}
	}

	// Construct build graph by filling deps for each node
	for _, target := range targetNodes {
		for _, depName := range target.GetDeps() {
			depNode, ok := targetNodes[depName]
			if !ok {
				glog.Warningf("target %q not found", depName) // ...hey, no one's perfect
				continue
			}
			target.Deps = append(target.Deps, depNode)
		}
		// Deps need to be sorted so that they get added to the hash in a stable order
		sort.Slice(target.Deps, func(i, j int) bool {
			return target.Deps[i].GetName() < target.Deps[j].GetName()
		})
	}

	return targetNodes, nil
}
