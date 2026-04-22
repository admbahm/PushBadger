// Package git wraps git CLI operations needed by pushbadger.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// IsRepo reports whether the current working directory is inside a git repository.
func IsRepo() bool {
	_, err := runGit("rev-parse", "--git-dir")
	return err == nil
}

// ResolveBase determines the base ref using the priority order specified in the
// behavior contract:
//  1. base argument (non-empty → use as-is)
//  2. git symbolic-ref refs/remotes/origin/HEAD
//  3. First of main, master, trunk that exists as a local ref
//
// Returns an error if none of the above resolves.
func ResolveBase(base string) (string, error) {
	if base != "" {
		return base, nil
	}

	// Try origin/HEAD symbolic ref.
	out, err := runGit("symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil && out != "" {
		// out is typically "origin/main"; strip the remote prefix.
		ref := strings.TrimSpace(out)
		if idx := strings.IndexByte(ref, '/'); idx != -1 {
			ref = ref[idx+1:]
		}
		return ref, nil
	}

	// Fall back to well-known branch names.
	for _, name := range []string{"main", "master", "trunk"} {
		if _, err := runGit("rev-parse", "--verify", "--quiet", name); err == nil {
			return name, nil
		}
	}

	return "", fmt.Errorf("cannot determine base branch: set --base, or ensure origin/HEAD is configured, or have a main/master/trunk branch")
}

// runGit runs git with the given arguments and returns trimmed stdout.
func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
