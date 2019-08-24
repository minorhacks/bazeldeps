load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "build_proto",
    srcs = ["build.proto"],
)

go_proto_library(
    name = "build_go_proto",
    importpath = "gitlab.com/minorhacks/bazeldeps/proto/build_proto",
    protos = [":build_proto"],
    visibility = ["//bazel:__subpackages__"],
)

proto_library(
    name = "blaze_query_proto",
    srcs = ["build.proto"],
    visibility = ["//visibility:public"],
)

go_proto_library(
    name = "blaze_query_go_proto",
    importpath = "gitlab.com/minorhacks/bazeldeps/proto",
    proto = ":blaze_query_proto",
    visibility = ["//visibility:public"],
)

go_library(
    name = "go_default_library",
    embed = [":blaze_query_go_proto"],
    importpath = "gitlab.com/minorhacks/bazeldeps/proto",
    visibility = ["//visibility:public"],
)
