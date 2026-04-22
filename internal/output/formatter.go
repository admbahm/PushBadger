// Package output formats a Report for human or machine consumption.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"pushbadger/pkg/types"
)

// WriteText writes a human-readable report to w.
func WriteText(w io.Writer, r *types.Report) error {
	// Header varies by diff mode.
	switch r.Mode {
	case "staged":
		fmt.Fprintf(w, "Risk analysis: staged changes (HEAD → index)\n")
	case "working":
		fmt.Fprintf(w, "Risk analysis: unstaged changes (index → worktree)\n")
	default:
		fmt.Fprintf(w, "Risk analysis: %s...%s\n", r.Base, r.Head)
	}

	if r.Truncated && r.TruncationReason != nil {
		fmt.Fprintf(w, "(report truncated: %s)\n", truncationSummary(r.TruncationReason))
	}
	fmt.Fprintln(w)

	if len(r.Areas) == 0 {
		fmt.Fprintln(w, "No changed files.")
		return nil
	}

	for _, area := range r.Areas {
		count := len(area.Files)
		fmt.Fprintf(w, "%s (%s)\n", area.Name, pluralize(count, "file"))
		for _, f := range area.Files {
			fmt.Fprintf(w, "  %s%s\n", f.Path, fileSuffix(f))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s, %s\n",
		pluralize(len(r.Files), "file"),
		pluralize(len(r.Areas), "area"),
	)
	return nil
}

// WriteJSON writes a machine-readable JSON report to w.
func WriteJSON(w io.Writer, r *types.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func truncationSummary(t *types.TruncationInfo) string {
	switch t.Reason {
	case "files":
		return fmt.Sprintf("file limit of %d reached", t.MaxFiles)
	case "diff_size":
		return fmt.Sprintf("diff size limit of %d KB reached", t.MaxDiffSizeKB)
	default: // "files_and_diff_size"
		return fmt.Sprintf("file limit of %d and diff size limit of %d KB both reached", t.MaxFiles, t.MaxDiffSizeKB)
	}
}

func fileSuffix(f types.ChangedFile) string {
	var parts []string
	if f.Deleted {
		parts = append(parts, "deleted")
	}
	if f.Binary {
		parts = append(parts, "binary")
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, ", ") + "]"
}

func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
