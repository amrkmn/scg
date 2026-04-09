const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{
        .default_target = .{
            .os_tag = .windows,
            .cpu_arch = .x86_64,
        },
    });

    const optimize = b.standardOptimizeOption(.{});

    const exe = b.addExecutable(.{
        .name = "shim",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });

    // Strip debug info for minimal binary.
    exe.root_module.strip = true;

    // Required for Win32 API (@cImport of windows.h)
    exe.linkLibC();

    b.installArtifact(exe);
}
