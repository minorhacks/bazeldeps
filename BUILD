load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix gitlab.com/minorhacks/bazeldeps
gazelle(name = "gazelle")

go_library(
    name = "go_default_library",
    srcs = ["deps_printer.go"],
    importpath = "gitlab.com/minorhacks/bazeldeps",
    visibility = ["//visibility:private"],
    deps = ["//bazel:go_default_library"],
)

go_binary(
    name = "bazeldeps",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
