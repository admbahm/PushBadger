package git

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"pushbadger/pkg/types"
)

const (
	// MaxFiles is the maximum number of changed files included in a report.
	MaxFiles = 200
	// MaxDiffBytes is the maximum raw diff size (in bytes) before truncation.
	MaxDiffBytes = 200 * 1024
)

// DiffOptions controls which git diff is computed.
type DiffOptions struct {
	Base    string // explicit base ref; empty means auto-resolve
	Head    string // explicit head ref; empty means "HEAD"
	Staged  bool   // diff HEAD vs index (git diff --cached)
	Working bool   // diff index vs worktree (git diff)
}

// DiffResult is returned by GetChangedFiles.
type DiffResult struct {
	Files         []types.ChangedFile
	Base          string
	Head          string
	FileTruncated bool // true if the 200-file cap was applied
	SizeTruncated bool // true if the 200 KB diff-size cap was applied
}

// Truncated reports whether either cap was applied.
func (r DiffResult) Truncated() bool {
	return r.FileTruncated || r.SizeTruncated
}

// GetChangedFiles runs git diff according to opts and returns a DiffResult.
func GetChangedFiles(opts DiffOptions) (DiffResult, error) {
	diffArgs, base, head := buildDiffArgs(opts)

	// --- 1. Parse the file list via --name-status -z ---
	nsArgs := append([]string{"diff", "--name-status", "-z"}, diffArgs...)
	nsOut, err := runGit(nsArgs...)
	if err != nil {
		return DiffResult{}, fmt.Errorf("git diff --name-status: %w", err)
	}
	rawFiles := parseNameStatus(nsOut)

	// --- 2. Get full diff to check size and detect binary files ---
	fullArgs := append([]string{"diff"}, diffArgs...)
	rawDiff, sizeTruncated, err := readDiffLimited(fullArgs)
	if err != nil {
		return DiffResult{}, fmt.Errorf("git diff: %w", err)
	}

	binaries := parseBinaryPaths(rawDiff)
	for i := range rawFiles {
		if binaries[rawFiles[i].Path] {
			rawFiles[i].Binary = true
		}
	}

	// --- 3. Apply file cap ---
	fileTruncated := false
	if len(rawFiles) > MaxFiles {
		rawFiles = rawFiles[:MaxFiles]
		fileTruncated = true
	}

	return DiffResult{
		Files:         rawFiles,
		Base:          base,
		Head:          head,
		FileTruncated: fileTruncated,
		SizeTruncated: sizeTruncated,
	}, nil
}

// buildDiffArgs returns the git diff argument slice plus the base/head labels
// to use in the report.
func buildDiffArgs(opts DiffOptions) (args []string, base, head string) {
	if opts.Staged {
		return []string{"--cached"}, "HEAD", "index"
	}
	if opts.Working {
		return []string{}, "index", "worktree"
	}
	h := opts.Head
	if h == "" {
		h = "HEAD"
	}
	rangeSpec := fmt.Sprintf("%s...%s", opts.Base, h)
	return []string{rangeSpec}, opts.Base, h
}

// parseNameStatus parses the NUL-separated output of `git diff --name-status -z`.
//
// Format per record:
//
//	<status>\0<path>\0              for A/M/D/T
//	<status>\0<old-path>\0<new-path>\0  for R/C (renamed/copied)
func parseNameStatus(out string) []types.ChangedFile {
	if out == "" {
		return nil
	}
	tokens := strings.Split(out, "\x00")
	var files []types.ChangedFile
	i := 0
	for i < len(tokens) {
		status := tokens[i]
		if status == "" {
			i++
			continue
		}
		i++
		if i >= len(tokens) {
			break
		}

		// Renames and copies: R<score> or C<score>
		if len(status) > 0 && (status[0] == 'R' || status[0] == 'C') {
			i++ // skip old path
			if i >= len(tokens) {
				break
			}
			newPath := tokens[i]
			i++
			files = append(files, types.ChangedFile{Path: newPath})
			continue
		}

		path := tokens[i]
		i++
		cf := types.ChangedFile{Path: path}
		if status == "D" {
			cf.Deleted = true
		}
		files = append(files, cf)
	}
	return files
}

// parseBinaryPaths scans the raw diff output for lines of the form
// "Binary files a/<path> and b/<path> differ" and returns a set of affected paths.
func parseBinaryPaths(diff []byte) map[string]bool {
	result := make(map[string]bool)
	for _, line := range bytes.Split(diff, []byte("\n")) {
		s := string(line)
		if !strings.HasPrefix(s, "Binary files ") {
			continue
		}
		// "Binary files a/foo/bar.png and b/foo/bar.png differ"
		// Extract the b/ path (new path).
		andIdx := strings.Index(s, " and b/")
		if andIdx == -1 {
			continue
		}
		after := s[andIdx+len(" and b/"):]
		path := strings.TrimSuffix(after, " differ")
		if path != "" {
			result[path] = true
		}
	}
	return result
}

// readDiffLimited runs a git diff command and reads up to MaxDiffBytes+1 bytes.
// The boolean return value is true if the output was larger than MaxDiffBytes.
func readDiffLimited(args []string) ([]byte, bool, error) {
	cmd := exec.Command("git", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	if err := cmd.Start(); err != nil {
		return nil, false, err
	}

	buf := make([]byte, MaxDiffBytes+1)
	n, readErr := io.ReadFull(stdout, buf)
	// Drain remaining output so the process can exit cleanly.
	io.Copy(io.Discard, stdout) //nolint:errcheck
	_ = cmd.Wait()

	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return nil, false, readErr
	}

	data := buf[:n]
	if n > MaxDiffBytes {
		return data[:MaxDiffBytes], true, nil
	}
	return data, false, nil
}
