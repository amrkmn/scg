const std = @import("std");
const Translator = @import("translate_c").Translator;

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{
        .default_target = .{
            .os_tag = .windows,
            .cpu_arch = .x86_64,
        },
    });
    const optimize = b.standardOptimizeOption(.{
        .preferred_optimize_mode = .ReleaseSmall,
    });

    // Translate the Win32 headers with the ZSF translate-c package
    // (Zig 0.17 removed @cImport). Exposed to src/main.zig as @import("c").
    const tc_dep = b.dependency("translate_c", .{});
    const win32_zig = Translator.init(tc_dep, .{
        .name = "win32",
        .c_source_file = b.path("src/win32.h"),
        .target = target,
        .optimize = optimize,
    });
    if (target.result.cpu.arch == .x86) {
        win32_zig.defineCMacro("_X86_", "1");
    }

    const exe = b.addExecutable(.{
        .name = "shim",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "c", .module = win32_zig.mod },
            },
        }),
    });

    // Strip debug info for minimal binary.
    exe.root_module.strip = true;

    // Required for the C translation module and std.
    exe.root_module.link_libc = true;

    // Required Win32 libraries (kernel32 must be explicit for cross targets).
    exe.root_module.linkSystemLibrary("kernel32", .{});
    exe.root_module.linkSystemLibrary("shell32", .{});
    exe.root_module.linkSystemLibrary("shlwapi", .{});

    b.installArtifact(exe);
}
