package install

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerArgs(t *testing.T) {
	vars := map[string]string{
		"dir": `C:\apps\tool\current`,
	}

	got, err := installerArgs([]any{"--path", "$dir"}, vars)
	if err != nil {
		t.Fatalf("installerArgs returned error: %v", err)
	}
	if len(got) != 2 || got[0] != "--path" || got[1] != `C:\apps\tool\current` {
		t.Fatalf("installerArgs result = %#v", got)
	}
}

func TestRunInstallerHookUnsupportedMap(t *testing.T) {
	err := RunInstallerHook("installer", map[string]any{"keep": true}, ".", nil)
	if err == nil {
		t.Fatal("expected error for unsupported installer map, got nil")
	}
}

func TestRunInstallerHookFileMissing(t *testing.T) {
	err := RunInstallerHook("installer", map[string]any{"file": "missing.exe"}, ".", nil)
	if err == nil {
		t.Fatal("expected missing file error, got nil")
	}
}

func TestSetupHookEnvVars_GlobalAlwaysPresent(t *testing.T) {
	userEnv := SetupHookEnvVars(`C:\apps\tool\1.0`, `C:\apps\tool\1.0`, "1.0", "64bit", "tool", `C:\persist\tool`, `C:\Users\me\scoop`, "tool.zip", false)
	if got, ok := userEnv["global"]; !ok || got != "" {
		t.Fatalf("user scope global var = %#v, present=%v; want empty present var", got, ok)
	}

	globalEnv := SetupHookEnvVars(`C:\apps\tool\1.0`, `C:\apps\tool\1.0`, "1.0", "64bit", "tool", `C:\persist\tool`, `C:\ProgramData\scoop`, "tool.zip", true)
	if got := globalEnv["global"]; got != "true" {
		t.Fatalf("global scope global var = %q, want true", got)
	}
}

func TestBuildHookPrelude_InjectsHelpersAndVariables(t *testing.T) {
	prelude := buildHookPrelude(map[string]string{
		"dir":      `C:\apps\tool\current`,
		"app":      "tool",
		"global":   "",
		"scoopdir": `C:\Users\me\scoop`,
	})

	checks := []string{
		"function Expand-7zipArchive",
		"function Expand-MsiArchive",
		"function Expand-InnoArchive",
		"function Expand-DarkArchive",
		"function Get-HelperPath",
		"function Find-BucketDirectory",
		"function bucketdir",
		"function Invoke-ExternalCommand",
		"function error_msg",
		"function abort",
		"function warn",
		"function info",
		"function success",
		"Set-Variable -Name 'dir' -Value 'C:\\apps\\tool\\current'",
		"Set-Variable -Name 'app' -Value 'tool'",
		"$env:scoopdir = 'C:\\Users\\me\\scoop'",
	}
	for _, want := range checks {
		if !strings.Contains(prelude, want) {
			t.Fatalf("prelude missing %q", want)
		}
	}
}

func TestFindBucketDirectoryAndBucketdirAlias(t *testing.T) {
	tempDir := t.TempDir()
	bucketsDir := filepath.Join(tempDir, "buckets")
	mainDir := filepath.Join(bucketsDir, "main", "bucket")
	doradoRoot := filepath.Join(bucketsDir, "dorado")
	doradoBucket := filepath.Join(doradoRoot, "bucket")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(doradoBucket, 0o755); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"scoopdir":   tempDir,
		"bucketsdir": bucketsDir,
	}
	script := strings.Join([]string{
		"Write-Output \"MAIN=$(Find-BucketDirectory -Name main)\"",
		"Write-Output \"ROOT=$(Find-BucketDirectory -Name dorado -Root)\"",
		"Write-Output \"SUB=$(Find-BucketDirectory -Name dorado)\"",
		"Write-Output \"ALIAS=$(bucketdir dorado)\"",
	}, "\n")

	stdout, stderr, err := runPowerShellHookScript(t, env, script)
	if err != nil {
		t.Fatalf("script failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	checks := []string{
		"MAIN=" + mainDir,
		"ROOT=" + doradoRoot,
		"SUB=" + doradoBucket,
		"ALIAS=" + doradoBucket,
	}
	for _, want := range checks {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q\nstdout:\n%s", want, stdout)
		}
	}
}

func TestMessageHelpers(t *testing.T) {
	stdout, stderr, err := runPowerShellHookScript(t, nil, strings.Join([]string{
		"error_msg 'bad'",
		"warn 'careful'",
		"info 'heads up'",
		"success 'done'",
	}, "\n"))
	if err != nil {
		t.Fatalf("script failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	for _, want := range []string{"ERROR bad", "WARN  careful", "INFO  heads up", "done"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q\nstdout:\n%s", want, stdout)
		}
	}
}

func TestAbortHelperExitsWithRequestedCode(t *testing.T) {
	stdout, stderr, err := runPowerShellHookScript(t, nil, "abort 'stop now' 7")
	if err == nil {
		t.Fatal("expected abort helper to terminate the script")
	}
	if !strings.Contains(stdout, "stop now") {
		t.Fatalf("stdout missing abort message\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(err.Error(), "exit status 7") {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 7 {
			t.Fatalf("abort helper exit = %v, want exit code 7", err)
		}
	}
}

func TestInvokeExternalCommandSupportsCommonForms(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "invoke.log")
	script := strings.Join([]string{
		"$cmd = (Get-Process -Id $PID).Path",
		"$ok = Invoke-ExternalCommand -Path $cmd -Args @('-NoProfile', '-Command', \"Write-Output 'hello'; exit 0\") -Activity 'run' -RunAs:$false -Quiet:$false",
		"Write-Output \"OK=$ok\"",
		"$continued = Invoke-ExternalCommand -FilePath $cmd -ArgumentList @('-NoProfile', '-Command', \"Write-Output 'continued'; exit 5\") -Msg 'retry' -cec @{ '5' = 'reboot required' }",
		"Write-Output \"CONTINUED=$continued\"",
		"$logged = Invoke-ExternalCommand -FilePath $cmd -ArgumentList @('-NoProfile', '-Command', \"Write-Output 'stdout'; [Console]::Error.WriteLine('stderr'); exit 0\") -Log '" + filepath.ToSlash(logPath) + "'",
		"Write-Output \"LOGGED=$logged\"",
	}, "\n")

	stdout, stderr, err := runPowerShellHookScript(t, nil, script)
	if err != nil {
		t.Fatalf("script failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	for _, want := range []string{"run hello", "done.", "OK=True", "retry continued", "WARN  reboot required", "CONTINUED=True", "LOGGED=True"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q\nstdout:\n%s", want, stdout)
		}
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	logText := string(logBytes)
	for _, want := range []string{"stdout", "stderr"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("log missing %q\nlog:\n%s", want, logText)
		}
	}
}

func TestOpencodeDesktopFixture(t *testing.T) {
	// Modeled after opencode-desktop.json pre_install hook:
	//   - Outer self-extracting .exe is first extracted to $dir (giving $dir\$PLUGINSDIR\app-*.7z)
	//   - Then: Expand-7zipArchive "$dir\$PLUGINSDIR\app-*.7z" "$dir"
	// This test validates the second step: wildcard resolution of nested 7z archives.

	psPath := FindPowerShell()
	if psPath == "" {
		t.Skip("PowerShell not available")
	}
	cmd7z := exec.Command(psPath, "-NoProfile", "-Command",
		"(Get-Command 7z -CommandType Application -ErrorAction Stop -TotalCount 1).Source")
	sevenZip, err := cmd7z.Output()
	if err != nil {
		t.Skip("7z not available: " + err.Error())
	}
	sevenZipPath := strings.TrimSpace(string(sevenZip))

	tempDir := t.TempDir()
	installDir := filepath.Join(tempDir, "apps", "opencode-desktop", "1.0")
	pluginsDir := filepath.Join(installDir, "$PLUGINSDIR")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create nested .7z files inside $PLUGINSDIR.
	for _, entry := range []string{"hello.txt:hello wildcard", "world.txt:world wildcard"} {
		parts := strings.SplitN(entry, ":", 2)
		fname, content := parts[0], parts[1]
		srcFile := filepath.Join(tempDir, fname)
		if err := os.WriteFile(srcFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		base := strings.TrimSuffix(fname, ".txt")
		inner7z := filepath.Join(pluginsDir, "app-"+base+".7z")
		cmd := exec.Command(sevenZipPath, "a", "-t7z", inner7z, srcFile)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to create inner 7z: %v\n%s", err, string(out))
		}
	}

	// Simulate the pre_install hook: wildcard expand the nested archives.
	// The wildcard "$dir\$PLUGINSDIR\app-*.7z" resolves to all app-*.7z files.
	innerGlob := filepath.Join(installDir, "$PLUGINSDIR", "app-*.7z")
	script := "Expand-7zipArchive -Path '" + innerGlob + "' -DestinationPath '" + installDir + "'"
	stdout, stderr, err := runPowerShellHookScript(t, nil, script)
	if err != nil {
		t.Fatalf("Expand-7zipArchive wildcard failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	// hello.txt and world.txt should be extracted into installDir.
	for _, fname := range []string{"hello.txt", "world.txt"} {
		if _, err := os.Stat(filepath.Join(installDir, fname)); os.IsNotExist(err) {
			t.Fatalf("expected %q in installDir after wildcard extraction\ndir: %s",
				fname, dirContents(installDir))
		}
	}
}

func dirContents(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err.Error()
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return strings.Join(names, ", ")
}

func TestExpandZipArchive(t *testing.T) {
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	dstDir := filepath.Join(tempDir, "dst")
	extractDir := filepath.Join(dstDir, "extracted")

	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(srcDir, "hello.txt")
	if err := os.WriteFile(srcFile, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(tempDir, "test.zip")
	psPath := FindPowerShell()
	if psPath == "" {
		t.Skip("PowerShell not available")
	}
	compressCmd := exec.Command(psPath, "-NoProfile", "-Command",
		"Compress-Archive -Path '"+srcFile+"' -DestinationPath '"+zipPath+"' -Force")
	if err := compressCmd.Run(); err != nil {
		t.Fatalf("failed to create test zip: %v", err)
	}

	script := "Expand-ZipArchive -Path '" + zipPath + "' -DestinationPath '" + dstDir + "' -Removal"
	stdout, stderr, err := runPowerShellHookScript(t, nil, script)
	if err != nil {
		t.Fatalf("Expand-ZipArchive failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	extracted := filepath.Join(dstDir, "hello.txt")
	if _, err := os.Stat(extracted); os.IsNotExist(err) {
		t.Fatalf("expected %q to exist after extraction", extracted)
	}
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Fatal("expected zip to be removed after -Removal")
	}

	// Test with ExtractDir
	zipPath2 := filepath.Join(tempDir, "test2.zip")
	innerDir := filepath.Join(tempDir, "inner")
	if err := os.MkdirAll(filepath.Join(innerDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(innerDir, "sub", "nested.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}
	compressCmd2 := exec.Command(psPath, "-NoProfile", "-Command",
		"Compress-Archive -Path '"+filepath.Join(innerDir, "*")+"' -DestinationPath '"+zipPath2+"' -Force")
	if err := compressCmd2.Run(); err != nil {
		t.Fatalf("failed to create nested test zip: %v", err)
	}

	_ = os.RemoveAll(extractDir)
	script2 := "Expand-ZipArchive -Path '" + zipPath2 + "' -DestinationPath '" + extractDir + "' -ExtractDir 'sub'"
	_, _, err = runPowerShellHookScript(t, nil, script2)
	if err != nil {
		t.Fatalf("Expand-ZipArchive with ExtractDir failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(extractDir, "nested.txt")); os.IsNotExist(err) {
		t.Fatal("expected nested.txt to exist after ExtractDir extraction")
	}
}

func TestExpandZipArchive_PositionalArgs(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "pos.txt")
	dstDir := filepath.Join(tempDir, "dst")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("positional"), 0o644); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(tempDir, "pos.zip")
	psPath := FindPowerShell()
	if psPath == "" {
		t.Skip("PowerShell not available")
	}
	compressCmd := exec.Command(psPath, "-NoProfile", "-Command",
		"Compress-Archive -Path '"+srcFile+"' -DestinationPath '"+zipPath+"' -Force")
	if err := compressCmd.Run(); err != nil {
		t.Fatalf("failed to create test zip: %v", err)
	}

	// Positional: Path, DestinationPath
	script := "Expand-ZipArchive '" + zipPath + "' '" + dstDir + "'"
	stdout, stderr, err := runPowerShellHookScript(t, nil, script)
	if err != nil {
		t.Fatalf("Expand-ZipArchive positional failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "pos.txt")); os.IsNotExist(err) {
		t.Fatal("expected pos.txt to exist after positional extraction")
	}
}

func TestArchiveHelperFileNotFound(t *testing.T) {
	// Archive helpers are tested via their "file not found" error path,
	// which validates that the helper resolves, invokes the external tool,
	// and throws the expected failure message.

	tests := []struct {
		name    string
		call    string
		errMsgs []string
	}{
		{
			name:    "Expand-7zipArchive file not found",
			call:    "Expand-7zipArchive -Path 'missing.7z' -DestinationPath '.'",
			errMsgs: []string{"Failed to extract files from missing.7z", "7-Zip is required"},
		},
		{
			name:    "Expand-InnoArchive file not found",
			call:    "Expand-InnoArchive -Path 'missing.exe' -DestinationPath '.'",
			errMsgs: []string{"Failed to extract files from missing.exe", "Inno Setup Unpacker is required"},
		},
		{
			name:    "Expand-DarkArchive file not found",
			call:    "Expand-DarkArchive -Path 'missing.msi' -DestinationPath '.'",
			errMsgs: []string{"Failed to extract files from missing.msi", "WiX Toolset (dark) is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runPowerShellHookScript(t, nil, tt.call)
			if err == nil {
				t.Fatalf("expected error for missing file, got nil\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
			combined := stdout + "\n" + stderr
			matched := false
			for _, msg := range tt.errMsgs {
				if strings.Contains(combined, msg) {
					matched = true
					break
				}
			}
			if !matched {
				t.Fatalf("expected error message containing one of %v\nerr=%v\nstdout:\n%s\nstderr:\n%s", tt.errMsgs, err, stdout, stderr)
			}
		})
	}
}

func TestArchiveHelperNamedAndWildcardParams(t *testing.T) {
	// Tests that archive helpers accept named params without
	// PowerShell binding errors, even if the actual tool is missing.
	tests := []struct {
		name string
		call string
	}{
		{
			name: "Expand-7zipArchive named params",
			call: "try { Expand-7zipArchive -Path 'x.7z' -ExtractDir 'sub' -Switches '-mx=9' -Overwrite Skip -Removal } catch { Write-Output \"OK=$($_.Exception.Message)\" }",
		},
		{
			name: "Expand-MsiArchive named params",
			call: "try { Expand-MsiArchive -Path 'x.msi' -ExtractDir 'sub' -Switches '-quiet' -Removal } catch { Write-Output \"OK=$($_.Exception.Message)\" }",
		},
		{
			name: "Expand-InnoArchive positional params",
			call: "try { Expand-InnoArchive 'x.exe' '.' -ExtractDir '{app}' } catch { Write-Output \"OK=$($_.Exception.Message)\" }",
		},
		{
			name: "Expand-DarkArchive named params",
			call: "try { Expand-DarkArchive -Path 'x.msi' -Switches '-sval' -Removal } catch { Write-Output \"OK=$($_.Exception.Message)\" }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runPowerShellHookScript(t, nil, tt.call)
			if err != nil {
				t.Fatalf("unexpected error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
			}
			if strings.Contains(stdout, "cannot be found") || strings.Contains(stdout, "is not recognized") {
				t.Fatalf("parameter binding error in stdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	tempDir := t.TempDir()
	appDir := filepath.Join(tempDir, "apps", "myapp")
	verDir := filepath.Join(appDir, "1.0")
	curDir := filepath.Join(appDir, "current")
	persistDir := filepath.Join(tempDir, "persist", "myapp")

	env := map[string]string{
		"scoopdir": tempDir,
	}

	tests := []struct {
		name   string
		script string
		checks []string
	}{
		{
			name:   "ensure creates directory",
			script: "ensure '" + filepath.Join(tempDir, "newdir") + "' | Out-Null; if (Test-Path '" + filepath.Join(tempDir, "newdir") + "') { Write-Output 'CREATED' }",
			checks: []string{"CREATED"},
		},
		{
			name:   "ensure returns path of existing directory",
			script: "$r = ensure '" + tempDir + "'; Write-Output \"DIR=$r\"",
			checks: []string{"DIR=" + tempDir},
		},
		{
			name:   "appdir returns correct path",
			script: "$d = appdir 'myapp' $false; Write-Output \"APPDIR=$d\"",
			checks: []string{"APPDIR=" + appDir},
		},
		{
			name:   "versiondir returns correct path",
			script: "$d = versiondir 'myapp' '1.0' $false; Write-Output \"VERDIR=$d\"",
			checks: []string{"VERDIR=" + verDir},
		},
		{
			name:   "currentdir returns correct path",
			script: "$d = currentdir 'myapp' $false; Write-Output \"CURDIR=$d\"",
			checks: []string{"CURDIR=" + curDir},
		},
		{
			name:   "persistdir returns correct path",
			script: "$d = persistdir 'myapp' $false; Write-Output \"PERSISTDIR=$d\"",
			checks: []string{"PERSISTDIR=" + persistDir},
		},
		{
			name:   "is_admin returns boolean",
			script: "$r = is_admin; Write-Output \"ADMIN=$r\"",
			checks: []string{"ADMIN="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runPowerShellHookScript(t, env, tt.script)
			if err != nil {
				t.Fatalf("script failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
			}
			for _, want := range tt.checks {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout missing %q\nstdout:\n%s", want, stdout)
				}
			}
		})
	}
}

func runPowerShellHookScript(t *testing.T, envVars map[string]string, script string) (string, string, error) {
	t.Helper()

	psPath := FindPowerShell()
	if psPath == "" {
		t.Skip("PowerShell not available")
	}

	fullScript := buildHookPrelude(envVars) + "\n" + script
	cmd := exec.Command(psPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", fullScript)
	cmd.Dir = t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
