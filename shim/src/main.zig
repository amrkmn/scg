const std = @import("std");
const builtin = @import("builtin");

comptime {
    if (builtin.os.tag != .windows) {
        @compileError("This shim only works on Windows");
    }
}

const win32 = @cImport({
    @cInclude("windows.h");
    @cInclude("shellapi.h");
});

// Job Object limit: kill child processes when job handle is closed
const JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE: u32 = 0x2000;
// CreateProcessW error when target requires UAC elevation
const ERROR_ELEVATION_REQUIRED: u32 = 740;
// Directory placeholder in .shim files
const DIR_PLACEHOLDER = "%~dp0";

// Ctrl+C handler: ignore all signals so they propagate to the child process.
// Without this, Ctrl+C kills the shim and orphans the child.
fn ctrlHandler(_: win32.DWORD) callconv(.c) win32.BOOL {
    return win32.TRUE;
}

// Expand %VAR% environment variable references in a string.
fn expandEnvVars(allocator: std.mem.Allocator, input: []const u8) ![]u8 {
    var result: std.ArrayList(u8) = .empty;
    defer result.deinit(allocator);

    var i: usize = 0;
    while (i < input.len) {
        if (input[i] == '%') {
            const start = i + 1;
            var end = start;
            while (end < input.len and input[end] != '%') : (end += 1) {}

            if (end < input.len and end > start) {
                const var_name = input[start..end];
                if (std.process.getEnvVarOwned(allocator, var_name)) |value| {
                    defer allocator.free(value);
                    try result.appendSlice(allocator, value);
                } else |_| {
                    // Variable not found, keep original text
                    try result.appendSlice(allocator, input[i .. end + 1]);
                }
                i = end + 1;
            } else {
                try result.append(allocator, input[i]);
                i += 1;
            }
        } else {
            try result.append(allocator, input[i]);
            i += 1;
        }
    }

    return result.toOwnedSlice(allocator);
}

// Replace all occurrences of a substring.
fn replaceOwned(allocator: std.mem.Allocator, input: []const u8, needle: []const u8, replacement: []const u8) ![]u8 {
    var result: std.ArrayList(u8) = .empty;
    defer result.deinit(allocator);

    var i: usize = 0;
    while (i < input.len) {
        if (i + needle.len <= input.len and std.mem.eql(u8, input[i .. i + needle.len], needle)) {
            try result.appendSlice(allocator, replacement);
            i += needle.len;
        } else {
            try result.append(allocator, input[i]);
            i += 1;
        }
    }

    return result.toOwnedSlice(allocator);
}

// Build a Windows command line string from argv, quoting args that contain spaces.
fn buildCommandLine(allocator: std.mem.Allocator, argv: []const []const u8) ![]u8 {
    var result: std.ArrayList(u8) = .empty;
    defer result.deinit(allocator);

    for (argv, 0..) |arg, idx| {
        if (idx > 0) try result.append(allocator, ' ');
        const needs_quote = arg.len == 0 or std.mem.indexOfAny(u8, arg, " \t\"") != null;
        if (needs_quote) {
            try result.append(allocator, '"');
            for (arg) |c| {
                if (c == '"') try result.append(allocator, '"');
                try result.append(allocator, c);
            }
            try result.append(allocator, '"');
        } else {
            try result.appendSlice(allocator, arg);
        }
    }

    return result.toOwnedSlice(allocator);
}

// Spawn a process with Job Object (orphan prevention) and Ctrl+C handling.
// Falls back to ShellExecuteExW for UAC elevation if needed.
fn spawnWithJob(allocator: std.mem.Allocator, argv: []const []const u8, cwd: ?[]const u8) !u32 {
    const cmdline_utf8 = try buildCommandLine(allocator, argv);
    defer allocator.free(cmdline_utf8);

    const cmdline_utf16 = try std.unicode.utf8ToUtf16LeAllocZ(allocator, cmdline_utf8);
    defer allocator.free(cmdline_utf16);

    var cwd_utf16: ?[:0]const u16 = null;
    if (cwd) |c| {
        cwd_utf16 = try std.unicode.utf8ToUtf16LeAllocZ(allocator, c);
    }
    defer if (cwd_utf16) |c| allocator.free(c);

    // Register Ctrl+C handler so signals pass to the child
    _ = win32.SetConsoleCtrlHandler(ctrlHandler, win32.TRUE);

    // Create a Job Object with KILL_ON_JOB_CLOSE so children die if we die
    const job = win32.CreateJobObjectW(null, null);
    if (job == null) return error.CreateJobObjectFailed;
    defer _ = win32.CloseHandle(job);

    var job_info: win32.JOBOBJECT_EXTENDED_LIMIT_INFORMATION = std.mem.zeroes(win32.JOBOBJECT_EXTENDED_LIMIT_INFORMATION);
    job_info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    if (win32.SetInformationJobObject(
        job,
        win32.JobObjectExtendedLimitInformation,
        &job_info,
        @sizeOf(@TypeOf(job_info)),
    ) == 0) {
        return error.SetJobInfoFailed;
    }

    var si: win32.STARTUPINFOW = std.mem.zeroes(win32.STARTUPINFOW);
    si.cb = @sizeOf(win32.STARTUPINFOW);
    var pi: win32.PROCESS_INFORMATION = std.mem.zeroes(win32.PROCESS_INFORMATION);

    // CreateProcessW may modify the command line buffer
    const cmdline_buf = try allocator.dupeZ(u16, cmdline_utf16);
    defer allocator.free(cmdline_buf);

    const create_result = win32.CreateProcessW(
        null, // application name (use command line)
        @ptrCast(cmdline_buf), // command line (mutable)
        null, // process security attributes
        null, // thread security attributes
        win32.TRUE, // inherit handles (stdin/stdout/stderr)
        0, // creation flags
        null, // environment (inherit parent)
        if (cwd_utf16) |c| c.ptr else null, // current directory
        &si,
        &pi,
    );

    if (create_result == 0) {
        const err = win32.GetLastError();
        if (err == ERROR_ELEVATION_REQUIRED) {
            return try shellExecuteElevated(allocator, argv);
        }
        return error.CreateProcessFailed;
    }

    defer {
        _ = win32.CloseHandle(pi.hProcess);
        _ = win32.CloseHandle(pi.hThread);
    }

    // Assign child to job object for lifecycle management
    _ = win32.AssignProcessToJobObject(job, pi.hProcess);

    _ = win32.WaitForSingleObject(pi.hProcess, win32.INFINITE);

    var exit_code: u32 = 0;
    _ = win32.GetExitCodeProcess(pi.hProcess, &exit_code);

    return exit_code;
}

// UAC elevation fallback using ShellExecuteExW with "runas" verb.
fn shellExecuteElevated(allocator: std.mem.Allocator, argv: []const []const u8) !u32 {
    if (argv.len == 0) return error.EmptyArgs;

    const exe_utf16 = try std.unicode.utf8ToUtf16LeAllocZ(allocator, argv[0]);
    defer allocator.free(exe_utf16);

    var params_utf8: std.ArrayList(u8) = .empty;
    defer params_utf8.deinit(allocator);

    for (argv[1..], 0..) |arg, idx| {
        if (idx > 0) try params_utf8.append(allocator, ' ');
        try params_utf8.appendSlice(allocator, arg);
    }

    const params_utf16 = if (params_utf8.items.len > 0)
        try std.unicode.utf8ToUtf16LeAllocZ(allocator, params_utf8.items)
    else
        null;
    defer if (params_utf16) |p| allocator.free(p);

    const verb = std.unicode.utf8ToUtf16LeStringLiteral("runas");

    var sei: win32.SHELLEXECUTEINFOW = std.mem.zeroes(win32.SHELLEXECUTEINFOW);
    sei.cbSize = @sizeOf(win32.SHELLEXECUTEINFOW);
    sei.lpVerb = verb;
    sei.lpFile = exe_utf16;
    sei.lpParameters = if (params_utf16) |p| p.ptr else null;
    sei.nShow = win32.SW_NORMAL;

    if (win32.ShellExecuteExW(&sei) == 0) {
        return error.ShellExecuteFailed;
    }

    if (sei.hProcess != null) {
        defer _ = win32.CloseHandle(sei.hProcess);
        _ = win32.WaitForSingleObject(sei.hProcess, win32.INFINITE);

        var exit_code: u32 = 0;
        _ = win32.GetExitCodeProcess(sei.hProcess, &exit_code);
        return exit_code;
    }

    return 0;
}

const ShimConfig = struct {
    path: []const u8,
    args: ?[]const u8,
    env_vars: std.ArrayList(struct { key: []const u8, val: []const u8 }) = .empty,

    fn deinit(self: *ShimConfig, allocator: std.mem.Allocator) void {
        allocator.free(self.path);
        if (self.args) |a| allocator.free(a);
        for (self.env_vars.items) |item| {
            allocator.free(item.key);
            allocator.free(item.val);
        }
        self.env_vars.deinit(allocator);
    }
};

fn readShimConfig(allocator: std.mem.Allocator, shim_path: []const u8, shim_dir: []const u8) !ShimConfig {
    const file = try std.fs.cwd().openFile(shim_path, .{});
    defer file.close();

    const contents = try file.readToEndAlloc(allocator, 8192);
    defer allocator.free(contents);

    var config = ShimConfig{
        .path = &.{},
        .args = null,
    };
    errdefer config.deinit(allocator);

    var line_iter = std.mem.splitSequence(u8, contents, "\n");
    while (line_iter.next()) |line| {
        const trimmed = std.mem.trim(u8, line, " \t\r");
        if (trimmed.len == 0 or trimmed[0] == '#') continue;

        if (std.mem.indexOf(u8, trimmed, "=")) |eq_pos| {
            const key = std.mem.trim(u8, trimmed[0..eq_pos], " \t");
            const val = std.mem.trim(u8, trimmed[eq_pos + 1 ..], " \t");

            if (std.mem.eql(u8, key, "path")) {
                var path_val = val;
                // Strip surrounding quotes
                if (path_val.len >= 2 and path_val[0] == '"' and path_val[path_val.len - 1] == '"') {
                    path_val = path_val[1 .. path_val.len - 1];
                }
                // Expand %VAR% environment variables
                var expanded = try expandEnvVars(allocator, path_val);
                // Expand %~dp0 to shim's directory
                if (std.mem.indexOf(u8, expanded, DIR_PLACEHOLDER)) |_| {
                    const old = expanded;
                    expanded = try replaceOwned(allocator, expanded, DIR_PLACEHOLDER, shim_dir);
                    allocator.free(old);
                }
                config.path = expanded;
            } else if (std.mem.eql(u8, key, "args")) {
                if (val.len > 0) {
                    config.args = try replaceOwned(allocator, val, DIR_PLACEHOLDER, shim_dir);
                }
            } else {
                // Any other key is treated as a custom environment variable
                const expanded_val = try expandEnvVars(allocator, val);
                try config.env_vars.append(allocator, .{
                    .key = try allocator.dupe(u8, key),
                    .val = expanded_val,
                });
            }
        }
    }

    return config;
}

pub fn main() !void {
    var gpa: std.heap.DebugAllocator(.{}) = .init;
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    // Get our own executable path
    const exe_path = try std.fs.selfExePathAlloc(allocator);
    defer allocator.free(exe_path);

    // Build the .shim file path: replace .exe with .shim
    const shim_path = if (std.mem.endsWith(u8, exe_path, ".exe"))
        try std.fmt.allocPrint(allocator, "{s}.shim", .{exe_path[0 .. exe_path.len - 4]})
    else
        try std.fmt.allocPrint(allocator, "{s}.shim", .{exe_path});
    defer allocator.free(shim_path);

    // Get shim's directory for %~dp0 expansion
    const shim_dir = std.fs.path.dirnamePosix(shim_path) orelse ".";

    // Read and parse the .shim file
    var config = readShimConfig(allocator, shim_path, shim_dir) catch {
        std.process.exit(1);
    };
    defer config.deinit(allocator);

    if (config.path.len == 0) {
        std.process.exit(1);
    }

    // Collect user arguments (skip argv[0] which is the shim itself)
    const user_args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, user_args);

    // Build argv: target_path [shim_args...] [user_args...]
    var argv_list: std.ArrayList([]const u8) = .empty;
    defer argv_list.deinit(allocator);

    try argv_list.append(allocator, config.path);

    // Append shim args if present
    if (config.args) |shim_args| {
        var iter = std.mem.splitSequence(u8, shim_args, " ");
        while (iter.next()) |arg| {
            if (arg.len > 0) {
                try argv_list.append(allocator, arg);
            }
        }
    }

    // Append user arguments
    for (user_args[1..]) |arg| {
        try argv_list.append(allocator, arg);
    }

    // Set custom environment variables from .shim file
    for (config.env_vars.items) |item| {
        const key_w = try std.unicode.utf8ToUtf16LeAllocZ(allocator, item.key);
        defer allocator.free(key_w);
        const val_w = try std.unicode.utf8ToUtf16LeAllocZ(allocator, item.val);
        defer allocator.free(val_w);
        _ = win32.SetEnvironmentVariableW(key_w.ptr, val_w.ptr);
    }

    // Spawn with Job Object and Ctrl+C handling
    const target_dir = std.fs.path.dirnamePosix(config.path) orelse ".";
    const exit_code = spawnWithJob(allocator, argv_list.items, target_dir) catch {
        std.process.exit(1);
    };

    std.process.exit(@truncate(exit_code));
}
