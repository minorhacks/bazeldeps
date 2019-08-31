# Bazel Change Detector

This repo contains a toy implementation of a target change detector using
Bazel. The goal is to accurately detect which targets need to be rebuilt when
files change.

The basic approach is:

* Dump dependency information using `bazel cquery`
* Build a hash for each bazel target. For targets that represent files, the
  hash is the hash of the files' contents. For rules, it is the attributes of the
  rule and the hashes of its dependencies.
* Repeat at the second repo state
* Diff the hashes of bazel targets to determine changed targets

Naive approaches to this problem perform bazel queries based on a file list.
This approach has the following advantages:

* Accurately detects effects of changes to BUILD files. Naive approaches will
  assume that when BUILD files change, all dependent targets must be rebuilt.
  This approach is resilient to BUILD files due to comment changes, attribute
  changes, etc. and is granular to only the affected targets.
* Accurately detects effects of changes to WORKSPACE file, for the same reasons
  as above
* Doesn't take a list of changed files as input (although it does need to run
  at two repo states, e.g. across a git checkout operation).

## Try it

`bazel run //:bazeldeps -- --diff_changes=local` - diff local changes against last
commit on current branch. Performs `git stash` operations to save/restore local
changes.

`bazel run //:bazeldeps -- --diff_changes=last_commits` - diff changes between
most recent commit on current branch and its predecessor. Performs `git
checkout` operations to save/restore current checkout.

This repo contains a hierarchy of toy C++ dependencies to experiment with.

### File changes

1. Change `math/add.h`
1. `bazel run //:bazeldeps` -> changes `//math:add.h`, `//math:add`, `//calculator`

### Rule changes

1. Change `math/BUILD.bazel` - add `copts = ["-02"]` attribute to `add`
1. `bazel run //:bazeldeps` -> changes `//math:add`, `//calculator` (but not //math:mul)

### BUILD file comment change

1. Change `math/BUILD.bazel` - add comment
1. `bazel run //:bazeldeps` -> no changes

### WORKSPACE changes

1. Change `WORKSPACE` -  change `pflag` git commit to `972238283c0625cf3e881de7699ba8f2524c340a`
1. `bazel run //:bazeldeps` -> changes to pflag files/targets, `//:bazeldeps`, `//:go_default_library` (but not other Go targets)

### Starlark rule changes

TODO: This is currently unsupported (although very possible)
