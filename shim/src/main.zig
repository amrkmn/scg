const std = @import("std");
const builtin = @import("builtin");

comptime {
    if (builtin.os.tag != .windows) {
        @compileError("This shim only works on Windows");
    }
}

const win32 = @cImport({
    if (builtin.cpu.arch == .x86) {
        @cDefine("_X86_", "1");
    }
    @cInclude("windows.h");
    @cInclude("shellapi.h");
    @cInclude("shlwapi.h");
});

const DIR_PLACEHOLDER = "%~dp0";

fn ctrlHandler(_: win32.DWORD) callconv(.c) win32.BOOL {
    return win32.TRUE;
}

fn writeStderr(msg: []const u8) void {
    const stderr_handle = win32.GetStdHandle(win32.STD_ERROR_HANDLE);
    const invalid = @as(win32.HANDLE, @ptrFromInt(~@as(usize, 0)));
    if (stderr_handle == null or stderr_handle == invalid) return;

    var written: win32.DWORD = 0;
    _ = win32.WriteFile(stderr_handle, msg.ptr, @intCast(msg.len), &written, null);
}

fn stripUtf8Bom(contents: []const u8) []const u8 {
    if (contents.len >= 3 and contents[0] == 0xEF and contents[1] == 0xBB and contents[2] == 0xBF) {
        return contents[3..];
    }
    return contents;
}

fn appendPathForCommandLine(buf: *std.ArrayList(u8), allocator: std.mem.Allocator, path: []const u8) !void {
    if (path.len >= 2 and path[0] == '"' and path[path.len - 1] == '"') {
        try buf.appendSlice(allocator, path);
        return;
    }
    if (std.mem.indexOfScalar(u8, path, ' ') == null) {
        try buf.appendSlice(allocator, path);
        return;
    }
    var end = path.len;
    while (end > 0 and path[end - 1] == '\\') end -= 1;
    try buf.append(allocator, '"');
    try buf.appendSlice(allocator, path[0..end]);
    try buf.appendNTimes(allocator, '\\', (path.len - end) * 2);
    try buf.append(allocator, '"');
}

fn isGuiApplication(path: []const u8) bool {
    const allocator = std.heap.page_allocator;
    const path_w = std.unicode.utf8ToUtf16LeAllocZ(allocator, path) catch return false;
    defer allocator.free(path_w);

    const handle = win32.CreateFileW(
        path_w.ptr,
        win32.GENERIC_READ,
        win32.FILE_SHARE_READ,
        null,
        win32.OPEN_EXISTING,
        win32.FILE_ATTRIBUTE_NORMAL,
        null,
    );
    const INVALID_HANDLE_VALUE = @as(win32.HANDLE, @ptrFromInt(~@as(usize, 0)));
    if (handle == INVALID_HANDLE_VALUE) return false;
    defer _ = win32.CloseHandle(handle);

    var read: win32.DWORD = 0;

    // Verify MZ signature at offset 0.
    var mz_buf: [2]u8 = undefined;
    if (win32.ReadFile(handle, &mz_buf, 2, &read, null) == 0 or read != 2) return false;
    if (std.mem.readInt(u16, &mz_buf, .little) != 0x5A4D) return false;

    // Read e_lfanew at offset 0x3C.
    _ = win32.SetFilePointer(handle, 0x3C, null, win32.FILE_BEGIN);
    var e_lfanew_buf: [4]u8 = undefined;
    if (win32.ReadFile(handle, &e_lfanew_buf, 4, &read, null) == 0 or read != 4) return false;
    const e_lfanew = std.mem.readInt(u32, &e_lfanew_buf, .little);

    // Verify PE signature at e_lfanew.
    _ = win32.SetFilePointer(handle, @intCast(e_lfanew), null, win32.FILE_BEGIN);
    var pe_sig: [4]u8 = undefined;
    if (win32.ReadFile(handle, &pe_sig, 4, &read, null) == 0 or read != 4) return false;
    if (std.mem.readInt(u32, &pe_sig, .little) != 0x00004550) return false;

    // Subsystem is at PE signature (4) + COFF header (20) + optional header offset 68.
    _ = win32.SetFilePointer(handle, @intCast(e_lfanew + 4 + 20 + 68), null, win32.FILE_BEGIN);
    var sub_buf: [2]u8 = undefined;
    if (win32.ReadFile(handle, &sub_buf, 2, &read, null) == 0 or read != 2) return false;
    const subsystem = std.mem.readInt(u16, &sub_buf, .little);

    // IMAGE_SUBSYSTEM_WINDOWS_GUI = 2
    return subsystem == 2;
}

fn rawUserArgsTail(allocator: std.mem.Allocator) ![]u8 {
    const cmdline_w = win32.GetCommandLineW();
    if (cmdline_w == null) return allocator.dupe(u8, "");

    const cmdline = std.mem.span(cmdline_w.?);
    if (cmdline.len == 0) return allocator.dupe(u8, "");

    var i: usize = 0;
    while (i < cmdline.len and cmdline[i] == ' ') : (i += 1) {}
    if (i < cmdline.len and cmdline[i] == '"') {
        i += 1;
        while (i < cmdline.len and cmdline[i] != '"') : (i += 1) {}
        if (i < cmdline.len) i += 1;
    } else {
        while (i < cmdline.len and cmdline[i] != ' ') : (i += 1) {}
    }
    while (i < cmdline.len and cmdline[i] == ' ') : (i += 1) {}

    return std.unicode.wtf16LeToWtf8Alloc(allocator, cmdline[i..]) catch |err| switch (err) {
        error.OutOfMemory => return error.OutOfMemory,
    };
}

fn expandEnvVars(allocator: std.mem.Allocator, input: []const u8) ![]u8 {
    const input_w = try std.unicode.utf8ToUtf16LeAllocZ(allocator, input);
    defer allocator.free(input_w);

    const needed = win32.ExpandEnvironmentStringsW(input_w.ptr, null, 0);
    if (needed == 0) return allocator.dupe(u8, input);

    const buf = try allocator.alloc(u16, needed);
    defer allocator.free(buf);

    const len = win32.ExpandEnvironmentStringsW(input_w.ptr, buf.ptr, needed);
    if (len == 0 or len > needed) return allocator.dupe(u8, input);

    return std.unicode.wtf16LeToWtf8Alloc(allocator, buf[0 .. len - 1]);
}

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

fn spawnWithJob(allocator: std.mem.Allocator, path: []const u8, shim_args: ?[]const u8, user_tail: []const u8, cwd: ?[]const u8, wait_for_exit: bool) !u32 {
    var cmdline: std.ArrayList(u8) = .empty;
    defer cmdline.deinit(allocator);

    try appendPathForCommandLine(&cmdline, allocator, path);
    if (shim_args) |args| {
        if (args.len > 0) {
            try cmdline.append(allocator, ' ');
            try cmdline.appendSlice(allocator, args);
        }
    }
    try cmdline.appendSlice(allocator, user_tail);

    const cmdline_utf8 = try cmdline.toOwnedSlice(allocator);
    defer allocator.free(cmdline_utf8);

    const cmdline_utf16 = try std.unicode.utf8ToUtf16LeAllocZ(allocator, cmdline_utf8);
    defer allocator.free(cmdline_utf16);

    var cwd_utf16: ?[:0]const u16 = null;
    if (cwd) |c| {
        cwd_utf16 = try std.unicode.utf8ToUtf16LeAllocZ(allocator, c);
    }
    defer if (cwd_utf16) |c| allocator.free(c);

    _ = win32.SetConsoleCtrlHandler(ctrlHandler, win32.TRUE);

    var job: ?win32.HANDLE = null;
    defer {
        if (job) |h| {
            _ = win32.CloseHandle(h);
        }
    }

    if (wait_for_exit) {
        job = win32.CreateJobObjectW(null, null);
        if (job == null) return error.CreateJobObjectFailed;

        var job_info: win32.JOBOBJECT_EXTENDED_LIMIT_INFORMATION = std.mem.zeroes(win32.JOBOBJECT_EXTENDED_LIMIT_INFORMATION);
        job_info.BasicLimitInformation.LimitFlags = win32.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | win32.JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK;
        if (win32.SetInformationJobObject(
            job.?,
            win32.JobObjectExtendedLimitInformation,
            &job_info,
            @sizeOf(@TypeOf(job_info)),
        ) == 0) {
            return error.SetJobInfoFailed;
        }
    }

    var si: win32.STARTUPINFOW = std.mem.zeroes(win32.STARTUPINFOW);
    win32.GetStartupInfoW(&si);
    si.cb = @sizeOf(win32.STARTUPINFOW);
    var pi: win32.PROCESS_INFORMATION = std.mem.zeroes(win32.PROCESS_INFORMATION);

    const cmdline_buf = try allocator.dupeZ(u16, cmdline_utf16);
    defer allocator.free(cmdline_buf);

    const create_result = win32.CreateProcessW(
        null,
        @ptrCast(cmdline_buf),
        null,
        null,
        win32.TRUE,
        win32.CREATE_SUSPENDED,
        null,
        if (cwd_utf16) |c| c.ptr else null,
        &si,
        &pi,
    );

    if (create_result == 0) {
        const err = win32.GetLastError();
        if (err == win32.ERROR_ELEVATION_REQUIRED) {
            return try shellExecuteElevated(allocator, path, shim_args, user_tail, wait_for_exit);
        }
        writeStderr("Shim: could not create process.\n");
        return error.CreateProcessFailed;
    }

    defer {
        _ = win32.CloseHandle(pi.hProcess);
        _ = win32.CloseHandle(pi.hThread);
    }

    if (job) |h| {
        _ = win32.AssignProcessToJobObject(h, pi.hProcess);
    }
    if (win32.ResumeThread(pi.hThread) == 0xFFFFFFFF) {
        writeStderr("Shim: could not resume child process.\n");
        return error.ResumeThreadFailed;
    }

    if (!wait_for_exit) {
        return 0;
    }

    _ = win32.WaitForSingleObject(pi.hProcess, win32.INFINITE);

    var exit_code: u32 = 0;
    _ = win32.GetExitCodeProcess(pi.hProcess, &exit_code);

    return exit_code;
}

fn shellExecuteElevated(allocator: std.mem.Allocator, path: []const u8, shim_args: ?[]const u8, user_tail: []const u8, wait_for_exit: bool) !u32 {
    if (path.len == 0) return error.EmptyArgs;

    const exe_utf16 = try std.unicode.utf8ToUtf16LeAllocZ(allocator, path);
    defer allocator.free(exe_utf16);

    var params_utf8: std.ArrayList(u8) = .empty;
    defer params_utf8.deinit(allocator);

    if (shim_args) |args| {
        if (args.len > 0) {
            try params_utf8.appendSlice(allocator, args);
        }
    }
    try params_utf8.appendSlice(allocator, user_tail);

    const params_utf16 = if (params_utf8.items.len > 0)
        try std.unicode.utf8ToUtf16LeAllocZ(allocator, params_utf8.items)
    else
        null;
    defer if (params_utf16) |p| allocator.free(p);

    const verb = std.unicode.utf8ToUtf16LeStringLiteral("runas");

    var sei: win32.SHELLEXECUTEINFOW = std.mem.zeroes(win32.SHELLEXECUTEINFOW);
    sei.cbSize = @sizeOf(win32.SHELLEXECUTEINFOW);
    sei.fMask = win32.SEE_MASK_NOCLOSEPROCESS;
    sei.lpVerb = verb;
    sei.lpFile = exe_utf16;
    sei.lpParameters = if (params_utf16) |p| p.ptr else null;
    sei.nShow = win32.SW_NORMAL;

    if (win32.ShellExecuteExW(&sei) == 0) {
        writeStderr("Shim: could not create elevated process.\n");
        return error.ShellExecuteFailed;
    }

    if (sei.hProcess != null) {
        defer _ = win32.CloseHandle(sei.hProcess);

        if (!wait_for_exit) {
            return 0;
        }

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

fn readFileToEndAlloc(allocator: std.mem.Allocator, path: []const u8) ![]u8 {
    const path_w = try std.unicode.utf8ToUtf16LeAllocZ(allocator, path);
    defer allocator.free(path_w);

    const handle = win32.CreateFileW(
        path_w.ptr,
        win32.GENERIC_READ,
        win32.FILE_SHARE_READ,
        null,
        win32.OPEN_EXISTING,
        win32.FILE_ATTRIBUTE_NORMAL,
        null,
    );
    const INVALID_HANDLE_VALUE = @as(win32.HANDLE, @ptrFromInt(~@as(usize, 0)));
    if (handle == INVALID_HANDLE_VALUE) return error.OpenFileFailed;
    defer _ = win32.CloseHandle(handle);

    const file_size = win32.GetFileSize(handle, null);
    if (file_size == 0xFFFFFFFF) return error.GetFileSizeFailed;

    const buf = try allocator.alloc(u8, file_size);
    errdefer allocator.free(buf);

    var read: u32 = 0;
    if (win32.ReadFile(handle, buf.ptr, file_size, &read, null) == 0) {
        return error.ReadFileFailed;
    }
    if (read != file_size) {
        return error.PartialRead;
    }

    return buf;
}

fn readShimConfig(allocator: std.mem.Allocator, shim_path: []const u8, shim_dir: []const u8) !ShimConfig {
    const file_contents = try readFileToEndAlloc(allocator, shim_path);
    defer allocator.free(file_contents);
    const contents = stripUtf8Bom(file_contents);

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
                if (path_val.len >= 2 and path_val[0] == '"' and path_val[path_val.len - 1] == '"') {
                    path_val = path_val[1 .. path_val.len - 1];
                }
                var expanded = try expandEnvVars(allocator, path_val);
                if (std.mem.indexOf(u8, expanded, DIR_PLACEHOLDER)) |_| {
                    const old = expanded;
                    expanded = try replaceOwned(allocator, expanded, DIR_PLACEHOLDER, shim_dir);
                    allocator.free(old);
                }
                const normalized = try replaceOwned(allocator, expanded, "/", "\\");
                if (normalized.ptr != expanded.ptr) allocator.free(expanded);
                config.path = normalized;
            } else if (std.mem.eql(u8, key, "args")) {
                if (val.len > 0) {
                    config.args = try replaceOwned(allocator, val, DIR_PLACEHOLDER, shim_dir);
                }
            } else {
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
    const allocator = std.heap.page_allocator;

    var exe_path_w: std.ArrayList(u16) = .empty;
    defer exe_path_w.deinit(allocator);
    var buf_len: u32 = 260;
    while (true) {
        try exe_path_w.resize(allocator, buf_len);
        const len = win32.GetModuleFileNameW(null, exe_path_w.items.ptr, buf_len);
        if (len == 0) {
            writeStderr("Shim: could not resolve current executable path.\n");
            std.process.exit(1);
        }
        if (len < buf_len - 1) {
            exe_path_w.shrinkRetainingCapacity(len);
            break;
        }
        buf_len *= 2;
    }
    const exe_path = try std.unicode.wtf16LeToWtf8Alloc(allocator, exe_path_w.items);
    defer allocator.free(exe_path);

    const shim_path = if (std.mem.endsWith(u8, exe_path, ".exe"))
        try std.fmt.allocPrint(allocator, "{s}.shim", .{exe_path[0 .. exe_path.len - 4]})
    else
        try std.fmt.allocPrint(allocator, "{s}.shim", .{exe_path});
    defer allocator.free(shim_path);

    const shim_dir = std.fs.path.dirnamePosix(shim_path) orelse ".";

    var config = readShimConfig(allocator, shim_path, shim_dir) catch {
        writeStderr("Shim: could not read .shim file.\n");
        std.process.exit(1);
    };
    defer config.deinit(allocator);

    if (config.path.len == 0) {
        writeStderr("Shim: .shim file has no path entry.\n");
        std.process.exit(1);
    }

    const user_tail = rawUserArgsTail(allocator) catch {
        writeStderr("Shim: could not read command line arguments.\n");
        std.process.exit(1);
    };
    defer allocator.free(user_tail);

    const is_gui = isGuiApplication(config.path);
    if (is_gui) {
        _ = win32.FreeConsole();
    }

    for (config.env_vars.items) |item| {
        const key_w = try std.unicode.utf8ToUtf16LeAllocZ(allocator, item.key);
        defer allocator.free(key_w);
        const val_w = try std.unicode.utf8ToUtf16LeAllocZ(allocator, item.val);
        defer allocator.free(val_w);
        _ = win32.SetEnvironmentVariableW(key_w.ptr, val_w.ptr);
    }

    const target_dir = std.fs.path.dirnamePosix(config.path) orelse ".";
    const exit_code = spawnWithJob(allocator, config.path, config.args, user_tail, target_dir, !is_gui) catch {
        writeStderr("Shim: failed to launch target process.\n");
        std.process.exit(1);
    };

    std.process.exit(@truncate(exit_code));
}
