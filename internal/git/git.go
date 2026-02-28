package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CloneOptions configures git clone behaviour.
type CloneOptions struct {
	Depth      int
	OnProgress func(current, total int)
}

// PullOptions configures git pull behaviour.
type PullOptions struct {
	Quiet bool
}

// run executes a git command in the given directory and returns combined output.
func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// IsGitRepo reports whether path is a git repository (has a .git directory/file).
func IsGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// GetRemoteURL returns the URL of the "origin" remote for the repo at repoPath.
func GetRemoteURL(repoPath string) (string, error) {
	return run(repoPath, "remote", "get-url", "origin")
}

// GetLastCommitDate returns the date of the most recent commit in the repo.
func GetLastCommitDate(repoPath string) (time.Time, error) {
	out, err := run(repoPath, "log", "-1", "--format=%cI")
	if err != nil {
		return time.Time{}, err
	}
	if out == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, out)
	if err != nil {
		// Try without timezone offset
		t, err = time.Parse("2006-01-02T15:04:05-07:00", out)
		if err != nil {
			return time.Time{}, err
		}
	}
	return t, nil
}

// Clone clones a git repository from url into dest.
// If opts.OnProgress is set, stderr is streamed and progress percentages are parsed.
func Clone(url, dest string, opts CloneOptions) error {
	args := []string{"clone", "--progress"}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	args = append(args, url, dest)

	cmd := exec.Command("git", args...)

	if opts.OnProgress == nil {
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone: %w\n%s", err, stderr.String())
		}
		return nil
	}

	// Stream stderr for progress
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stderrPipe)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		current, total := parseCloneProgress(line)
		if total > 0 {
			opts.OnProgress(current, total)
		}
	}

	return cmd.Wait()
}

// parseCloneProgress extracts (current, total) from git clone progress lines like:
// "Receiving objects:  42% (42/100)"
// "Resolving deltas:  80% (80/100)"
func parseCloneProgress(line string) (int, int) {
	line = strings.TrimSpace(line)
	// Find percentage in parentheses: (current/total)
	start := strings.LastIndex(line, "(")
	end := strings.LastIndex(line, ")")
	if start < 0 || end < 0 || end <= start {
		return 0, 0
	}
	fraction := line[start+1 : end]
	parts := strings.SplitN(fraction, "/", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	current, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	total, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return current, total
}

// Pull runs git pull in the given repository directory.
func Pull(repoPath string, opts PullOptions) error {
	args := []string{"pull"}
	if opts.Quiet {
		args = append(args, "--quiet")
	} else {
		args = append(args, "--progress")
	}
	_, err := run(repoPath, args...)
	return err
}

// FetchAndMerge fetches from origin and fast-forward merges only if the remote
// has new commits. Returns ("up-to-date", nil) if already current, ("updated", nil)
// if new commits were merged, or ("", err) on failure.
func FetchAndMerge(repoPath string) (status string, commits []string, err error) {
	if _, err = run(repoPath, "fetch", "--quiet", "origin"); err != nil {
		return "", nil, err
	}

	head, err := run(repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", nil, err
	}

	fetchHead, err := run(repoPath, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return "", nil, err
	}

	if head == fetchHead {
		return "up-to-date", nil, nil
	}

	// Fast-forward merge
	if _, err = run(repoPath, "merge", "--ff-only", "--quiet", "FETCH_HEAD"); err != nil {
		return "", nil, err
	}

	return "updated", nil, nil
}

// Fetch runs git fetch --quiet in the given repository directory.
func Fetch(repoPath string) error {
	_, err := run(repoPath, "fetch", "--quiet")
	return err
}

// HasRemoteUpdates reports whether the local branch is behind its remote tracking branch.
func HasRemoteUpdates(repoPath string) (bool, error) {
	branch, err := GetCurrentBranch(repoPath)
	if err != nil {
		return false, err
	}
	remote := "origin/" + branch
	out, err := run(repoPath, "rev-list", "--count", "HEAD.."+remote)
	if err != nil {
		// Try HEAD..origin/HEAD as fallback
		out, err = run(repoPath, "rev-list", "--count", "HEAD..origin/HEAD")
		if err != nil {
			return false, err
		}
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetCommitCount returns the number of commits in the range from..to in repoPath.
func GetCommitCount(from, to, repoPath string) (int, error) {
	out, err := run(repoPath, "rev-list", "--count", from+".."+to)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// GetCommitsSince returns commit subject lines from hash..HEAD in repoPath.
func GetCommitsSince(hash, repoPath string) ([]string, error) {
	out, err := run(repoPath, "log", hash+"..HEAD", "--pretty=%s")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	result := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			result = append(result, l)
		}
	}
	return result, nil
}

// GetCommitHash returns the full commit hash for the given ref in repoPath.
func GetCommitHash(ref, repoPath string) (string, error) {
	return run(repoPath, "rev-parse", ref)
}

// GetCurrentBranch returns the name of the currently checked-out branch.
func GetCurrentBranch(repoPath string) (string, error) {
	return run(repoPath, "branch", "--show-current")
}

// GetRemoteTrackingBranch returns the remote tracking branch name (e.g. "origin/main").
func GetRemoteTrackingBranch(repoPath string) (string, error) {
	return run(repoPath, "rev-parse", "--abbrev-ref", "HEAD@{u}")
}
