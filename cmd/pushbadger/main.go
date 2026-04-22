package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"pushbadger/internal/analyzer"
	"pushbadger/internal/git"
	"pushbadger/internal/output"
	"pushbadger/pkg/types"
)

const appVersion = "v0.1.0-alpha"

func main() {
	// Load the embedded ruleset once at startup to get its version number.
	// This is fast (in-memory parse) and lets us surface the ruleset version
	// in both --version output and the JSON report.
	_, rulesetVer, err := analyzer.LoadRuleset("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "internal error: could not load embedded ruleset: %s\n", err)
		os.Exit(3)
	}

	if err := rootCmd(rulesetVer).Execute(); err != nil {
		os.Exit(2)
	}
}

func rootCmd(rulesetVersion int) *cobra.Command {
	root := &cobra.Command{
		Use:           "pushbadger",
		Short:         "Analyze git diffs and map changed files to risk areas",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       fmt.Sprintf("%s (ruleset version %d)", appVersion, rulesetVersion),
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.AddCommand(analyzeCmd(rulesetVersion))
	return root
}

func analyzeCmd(rulesetVersion int) *cobra.Command {
	var (
		base    string
		head    string
		staged  bool
		working bool
		format  string
		rules   string
	)

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze changed files and map them to risk areas",
		Long: `Analyze runs a git diff and maps each changed file to one or more risk
areas using a deterministic, path-based ruleset.

Diff modes (pick at most one):
  default              <resolved-base>...HEAD
  --staged             staged changes (HEAD → index)
  --working            unstaged changes (index → worktree)
  --base X / --head Y  explicit refs (combinable, default to auto/HEAD)

The base ref is resolved in order: --base flag → origin/HEAD → main/master/trunk.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(base, head, staged, working, format, rules, rulesetVersion)
		},
	}

	cmd.Flags().StringVar(&base, "base", "", "Base ref for diff (default: auto-detected)")
	cmd.Flags().StringVar(&head, "head", "", "Head ref for diff (default: HEAD)")
	cmd.Flags().BoolVar(&staged, "staged", false, "Diff staged changes (git diff --cached)")
	cmd.Flags().BoolVar(&working, "working", false, "Diff unstaged changes (git diff)")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.Flags().StringVar(&rules, "rules", "", "Path to custom rules YAML file (default: embedded ruleset)")

	return cmd
}

func runAnalyze(base, head string, staged, working bool, format, rulesPath string, rulesetVersion int) error {
	// Validate mutually exclusive flag combinations, naming each conflict.
	if staged && working {
		exit2("cannot use --staged and --working together")
	}
	if staged && base != "" {
		exit2("cannot use --staged with --base")
	}
	if staged && head != "" {
		exit2("cannot use --staged with --head")
	}
	if working && base != "" {
		exit2("cannot use --working with --base")
	}
	if working && head != "" {
		exit2("cannot use --working with --head")
	}
	if format != "text" && format != "json" {
		exit2("--format must be 'text' or 'json'")
	}

	// Confirm we're inside a git repo.
	if !git.IsRepo() {
		exit2("not inside a git repository")
	}

	// Resolve the base ref for normal diff mode.
	var resolvedBase string
	if !staged && !working {
		var err error
		resolvedBase, err = git.ResolveBase(base)
		if err != nil {
			exit2(err.Error())
		}
	}

	// Determine mode label and diff options.
	mode := "diff"
	if staged {
		mode = "staged"
	} else if working {
		mode = "working"
	}

	opts := git.DiffOptions{
		Base:    resolvedBase,
		Head:    head,
		Staged:  staged,
		Working: working,
	}

	// Run git diff.
	result, err := git.GetChangedFiles(opts)
	if err != nil {
		exit2(fmt.Sprintf("git error: %s", err))
	}

	// Build truncation info if either cap was hit.
	var truncInfo *types.TruncationInfo
	if result.Truncated() {
		truncInfo = &types.TruncationInfo{
			Reason:        truncReason(result),
			MaxFiles:      git.MaxFiles,
			MaxDiffSizeKB: git.MaxDiffBytes / 1024,
		}
		fmt.Fprintf(os.Stderr, "warning: diff truncated (%s)\n", truncInfo.Reason)
	}

	// Load rules and match. If --rules was given, re-parse to get its version.
	ruleList, ver, err := analyzer.LoadRuleset(rulesPath)
	if err != nil {
		exit3(fmt.Sprintf("failed to load ruleset: %s", err))
	}
	// Use the version from the actual ruleset in use (matters when --rules overrides).
	if rulesPath != "" {
		rulesetVersion = ver
	}

	areas := analyzer.Match(result.Files, ruleList)

	report := &types.Report{
		Base:             result.Base,
		Head:             result.Head,
		Mode:             mode,
		RulesetVersion:   rulesetVersion,
		Files:            result.Files,
		Areas:            areas,
		Truncated:        result.Truncated(),
		TruncationReason: truncInfo,
	}

	switch format {
	case "json":
		if err := output.WriteJSON(os.Stdout, report); err != nil {
			exit3(fmt.Sprintf("json output: %s", err))
		}
	default:
		if err := output.WriteText(os.Stdout, report); err != nil {
			exit3(fmt.Sprintf("text output: %s", err))
		}
	}

	return nil
}

func truncReason(r git.DiffResult) string {
	switch {
	case r.FileTruncated && r.SizeTruncated:
		return "files_and_diff_size"
	case r.FileTruncated:
		return "files"
	default:
		return "diff_size"
	}
}

func exit2(msg string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	os.Exit(2)
}

func exit3(msg string) {
	fmt.Fprintf(os.Stderr, "internal error: %s\n", msg)
	os.Exit(3)
}
