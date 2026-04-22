// Package integration tests pushbadger analyze against a real git repository
// created in a temporary directory during the test run.
package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"pushbadger/internal/analyzer"
	"pushbadger/internal/git"
	"pushbadger/pkg/types"
)

// fixtureRepo creates a temporary git repo, makes an initial commit, then adds
// additional files and commits them on top. It returns the repo directory and
// the SHA of the initial commit (used as the base for diffs).
func fixtureRepo(t *testing.T) (dir string, baseRef string) {
	t.Helper()
	dir = t.TempDir()

	gitCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	writeFile := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	gitCmd("init")
	gitCmd("config", "user.email", "test@example.com")
	gitCmd("config", "user.name", "Test")

	// Initial commit — baseline.
	writeFile("go.mod", "module example.com/app\n\ngo 1.21\n")
	gitCmd("add", ".")
	gitCmd("commit", "-m", "initial")

	// Record base commit SHA.
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	sha, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	baseRef = strings.TrimSpace(string(sha))

	// Second commit — the "diff" we'll analyze.
	writeFile("internal/auth/login.go", "package auth\n")
	writeFile("internal/auth/session.go", "package auth\n")
	writeFile("db/migrations/0001_init.sql", "CREATE TABLE users (id INT);\n")
	writeFile("internal/payments/checkout.go", "package payments\n")
	writeFile("cmd/app/main.go", "package main\n")
	writeFile("README.md", "# App\n")
	gitCmd("add", ".")
	gitCmd("commit", "-m", "add features")

	return dir, baseRef
}

func TestAnalyzeFixtureRepo(t *testing.T) {
	dir, baseRef := fixtureRepo(t)

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	if !git.IsRepo() {
		t.Fatal("IsRepo() returned false for fixture repo")
	}

	rules, _, err := analyzer.LoadRuleset("")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}

	result, err := git.GetChangedFiles(git.DiffOptions{Base: baseRef, Head: "HEAD"})
	if err != nil {
		t.Fatalf("GetChangedFiles: %v", err)
	}
	if result.Truncated() {
		t.Error("unexpected truncation for fixture repo")
	}
	if result.Base != baseRef {
		t.Errorf("base label: got %q, want %q", result.Base, baseRef)
	}
	if result.Head != "HEAD" {
		t.Errorf("head label: got %q, want %q", result.Head, "HEAD")
	}

	// Expect 6 files.
	if len(result.Files) != 6 {
		t.Errorf("got %d files, want 6: %v", len(result.Files), filePaths(result.Files))
	}

	areas := analyzer.Match(result.Files, rules)
	areaMap := make(map[string][]string)
	for _, a := range areas {
		areaMap[a.Name] = filePaths(a.Files)
	}

	// auth area must exist and include both auth files.
	authFiles, ok := areaMap["auth"]
	if !ok {
		t.Error("auth area missing")
	}
	if !containsPath(authFiles, "internal/auth/login.go") || !containsPath(authFiles, "internal/auth/session.go") {
		t.Errorf("auth area files: got %v", authFiles)
	}

	// payments area must include checkout.go.
	paymentFiles, ok := areaMap["payments"]
	if !ok {
		t.Error("payments area missing")
	}
	if !containsPath(paymentFiles, "internal/payments/checkout.go") {
		t.Errorf("payments area files: got %v", paymentFiles)
	}

	// db area must include the SQL file.
	dbFiles, ok := areaMap["db"]
	if !ok {
		t.Error("db area missing")
	}
	if !containsPath(dbFiles, "db/migrations/0001_init.sql") {
		t.Errorf("db area files: got %v", dbFiles)
	}

	// cmd/app/main.go has no matching rule → unclassified.
	unclFiles, ok := areaMap["unclassified"]
	if !ok {
		t.Error("unclassified area missing")
	}
	if !containsPath(unclFiles, "cmd/app/main.go") {
		t.Errorf("unclassified area files: got %v", unclFiles)
	}

	// Unclassified must be last.
	if areas[len(areas)-1].Name != "unclassified" {
		t.Errorf("last area should be unclassified, got %q", areas[len(areas)-1].Name)
	}
}

// TestDeterminismEndToEnd asserts that the same diff + ruleset always produces
// byte-identical JSON output.
func TestDeterminismEndToEnd(t *testing.T) {
	dir, baseRef := fixtureRepo(t)

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	rules, rulesetVersion, err := analyzer.LoadRuleset("")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}

	run := func() string {
		result, err := git.GetChangedFiles(git.DiffOptions{Base: baseRef, Head: "HEAD"})
		if err != nil {
			t.Fatalf("GetChangedFiles: %v", err)
		}
		report := types.Report{
			Base:           result.Base,
			Head:           result.Head,
			Mode:           "diff",
			RulesetVersion: rulesetVersion,
			Files:          result.Files,
			Areas:          analyzer.Match(result.Files, rules),
		}
		b, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(b)
	}

	first := run()
	for i := 0; i < 5; i++ {
		if got := run(); got != first {
			t.Fatalf("run %d output differs:\ngot:  %s\nwant: %s", i+1, got, first)
		}
	}
}

// TestResolveBase checks that ResolveBase works in a repo with a main branch.
func TestResolveBase(t *testing.T) {
	dir, _ := fixtureRepo(t)

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// Explicit base should be returned as-is.
	got, err := git.ResolveBase("somebranch")
	if err != nil {
		t.Fatalf("ResolveBase(explicit): %v", err)
	}
	if got != "somebranch" {
		t.Errorf("got %q, want %q", got, "somebranch")
	}
}

// TestStagedDiff verifies GetChangedFiles in staged mode and that labels are correct.
func TestStagedDiff(t *testing.T) {
	dir, _ := fixtureRepo(t)

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// Stage a new file.
	newFile := filepath.Join(dir, "internal", "auth", "newfile.go")
	if err := os.WriteFile(newFile, []byte("package auth\n"), 0644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	cmd := exec.Command("git", "add", "internal/auth/newfile.go")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	result, err := git.GetChangedFiles(git.DiffOptions{Staged: true})
	if err != nil {
		t.Fatalf("GetChangedFiles staged: %v", err)
	}
	if result.Truncated() {
		t.Error("unexpected truncation")
	}
	if result.Base != "HEAD" || result.Head != "index" {
		t.Errorf("staged labels: got base=%q head=%q, want HEAD/index", result.Base, result.Head)
	}
	if !containsPath(filePaths(result.Files), "internal/auth/newfile.go") {
		t.Errorf("staged files missing newfile.go: %v", filePaths(result.Files))
	}
}

// TestRulesetVersionInReport verifies that RulesetVersion is set on the report.
func TestRulesetVersionInReport(t *testing.T) {
	dir, baseRef := fixtureRepo(t)

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	rules, rulesetVersion, err := analyzer.LoadRuleset("")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}
	if rulesetVersion <= 0 {
		t.Fatalf("expected positive ruleset version, got %d", rulesetVersion)
	}

	result, err := git.GetChangedFiles(git.DiffOptions{Base: baseRef, Head: "HEAD"})
	if err != nil {
		t.Fatalf("GetChangedFiles: %v", err)
	}

	report := types.Report{
		Base:           result.Base,
		Head:           result.Head,
		Mode:           "diff",
		RulesetVersion: rulesetVersion,
		Files:          result.Files,
		Areas:          analyzer.Match(result.Files, rules),
	}

	b, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Verify the field appears in JSON output.
	if !strings.Contains(string(b), `"ruleset_version"`) {
		t.Errorf("JSON output missing ruleset_version field: %s", b)
	}
}

// TestMultiMatchOrderingStable verifies that a file matching multiple areas
// always lands in the same areas in the same order across repeated runs.
func TestMultiMatchOrderingStable(t *testing.T) {
	dir, baseRef := fixtureRepo(t)

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// Add a file that matches two rules: auth (*/auth/**) and tests (*_test.go).
	authTestFile := filepath.Join(dir, "internal", "auth", "session_test.go")
	if err := os.WriteFile(authTestFile, []byte("package auth\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	gitCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitCmd("add", "internal/auth/session_test.go")
	gitCmd("commit", "-m", "add auth test")

	rules, _, err := analyzer.LoadRuleset("")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}

	run := func() []string {
		result, err := git.GetChangedFiles(git.DiffOptions{Base: baseRef, Head: "HEAD"})
		if err != nil {
			t.Fatalf("GetChangedFiles: %v", err)
		}
		areas := analyzer.Match(result.Files, rules)
		names := make([]string, len(areas))
		for i, a := range areas {
			names[i] = a.Name
		}
		return names
	}

	first := run()
	for i := 0; i < 5; i++ {
		got := run()
		if len(got) != len(first) {
			t.Fatalf("run %d area count differs: got %v want %v", i+1, got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d area[%d] differs: got %q want %q", i+1, j, got[j], first[j])
			}
		}
	}

	// session_test.go must appear in both auth and tests.
	areaNames := first
	inAuth, inTests := false, false
	for _, n := range areaNames {
		if n == "auth" {
			inAuth = true
		}
		if n == "tests" {
			inTests = true
		}
	}
	if !inAuth {
		t.Error("auth area missing; expected session_test.go to match")
	}
	if !inTests {
		t.Error("tests area missing; expected session_test.go to match")
	}

	// auth (priority 20) must sort before tests (priority 70).
	authIdx, testsIdx := -1, -1
	for i, n := range areaNames {
		if n == "auth" {
			authIdx = i
		}
		if n == "tests" {
			testsIdx = i
		}
	}
	if authIdx >= testsIdx {
		t.Errorf("auth (priority 20) should precede tests (priority 70), got order: %v", areaNames)
	}
}

func filePaths(files []types.ChangedFile) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}

func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}
