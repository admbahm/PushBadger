// Package types defines the shared data model for PushBadger.
package types

import "math"

// UnclassifiedPriority is the sentinel priority assigned to the "unclassified"
// area so it always sorts last.
const UnclassifiedPriority = math.MaxInt

// ChangedFile is a single file from the git diff.
type ChangedFile struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted,omitempty"`
	Binary  bool   `json:"binary,omitempty"`
}

// AreaResult is one risk area and the files that matched it.
type AreaResult struct {
	Name     string        `json:"name"`
	Priority int           `json:"priority"`
	Files    []ChangedFile `json:"files"`
}

// TruncationInfo describes which limit was hit when a diff is truncated.
// Both threshold fields are always populated so consumers know what the limits are.
type TruncationInfo struct {
	// Reason is one of "files", "diff_size", or "files_and_diff_size".
	Reason        string `json:"reason"`
	MaxFiles      int    `json:"max_files"`
	MaxDiffSizeKB int    `json:"max_diff_size_kb"`
}

// Report is the top-level output of a pushbadger analyze run.
//
// Base and Head are descriptive strings:
//   - For "diff" mode they are the resolved refs (e.g. "main", "HEAD").
//   - For "staged" mode: Base="HEAD", Head="index".
//   - For "working" mode: Base="index", Head="worktree".
//
// Areas are sorted by Priority ascending, then Name ascending.
// Files with no matching rule are collected in a final "unclassified" area
// (Priority = math.MaxInt).
//
// Truncated is set when the diff exceeded 200 files or 200 KB; in that case
// only the first 200 files / 200 KB are reflected in the report.
// TruncationReason is populated whenever Truncated is true.
type Report struct {
	Base             string          `json:"base"`
	Head             string          `json:"head"`
	Mode             string          `json:"mode"`
	RulesetVersion   int             `json:"ruleset_version"`
	Files            []ChangedFile   `json:"files"`
	Areas            []AreaResult    `json:"areas"`
	Truncated        bool            `json:"truncated,omitempty"`
	TruncationReason *TruncationInfo `json:"truncation_reason,omitempty"`
}
