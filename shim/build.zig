const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{
        .default_target = .{
            .os_tag = .windows,
            .cpu_arch = .x86_64,
        },
    });
    const exe = b.addExecutable(.{
        .name = "shim",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = .ReleaseSmall,
        }),
    });

    // Strip debug info for minimal binary.
    exe.root_module.strip = true;

    // Required for Win32 headers via @cImport.
    exe.root_module.link_libc = true;

    // Required Win32 libraries.
    exe.root_module.linkSystemLibrary("shell32", .{});
    exe.root_module.linkSystemLibrary("shlwapi", .{});

    b.installArtifact(exe);
}
