package analyzer

import (
	"encoding/json"
	"testing"

	"pushbadger/pkg/types"
)

var testRules = []Rule{
	{Area: "payments", Priority: 10, Patterns: []string{"**/payment*/**", "**/billing/**"}},
	{Area: "auth", Priority: 20, Patterns: []string{"**/auth/**", "**/session*"}},
	{Area: "db", Priority: 30, Patterns: []string{"**/*.sql", "**/migrations/**"}},
	{Area: "docs", Priority: 80, Patterns: []string{"**/*.md"}},
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name      string
		files     []types.ChangedFile
		wantAreas []string // area names in expected order
		wantFiles map[string][]string // area -> sorted file paths
	}{
		{
			name: "single file matches single area",
			files: []types.ChangedFile{
				{Path: "internal/auth/login.go"},
			},
			wantAreas: []string{"auth"},
			wantFiles: map[string][]string{
				"auth": {"internal/auth/login.go"},
			},
		},
		{
			name: "file matches multiple areas",
			files: []types.ChangedFile{
				{Path: "internal/auth/login.go"},
				{Path: "db/migrations/0001.sql"},
				{Path: "README.md"},
			},
			wantAreas: []string{"auth", "db", "docs"},
			wantFiles: map[string][]string{
				"auth": {"internal/auth/login.go"},
				"db":   {"db/migrations/0001.sql"},
				"docs": {"README.md"},
			},
		},
		{
			name: "no match produces unclassified",
			files: []types.ChangedFile{
				{Path: "cmd/app/main.go"},
			},
			wantAreas: []string{"unclassified"},
			wantFiles: map[string][]string{
				"unclassified": {"cmd/app/main.go"},
			},
		},
		{
			name: "mix of matched and unclassified — unclassified is last",
			files: []types.ChangedFile{
				{Path: "cmd/app/main.go"},
				{Path: "internal/auth/token.go"},
			},
			wantAreas: []string{"auth", "unclassified"},
			wantFiles: map[string][]string{
				"auth":         {"internal/auth/token.go"},
				"unclassified": {"cmd/app/main.go"},
			},
		},
		{
			name: "path matching is case-insensitive",
			files: []types.ChangedFile{
				{Path: "Internal/AUTH/Login.go"},
			},
			wantAreas: []string{"auth"},
			wantFiles: map[string][]string{
				"auth": {"Internal/AUTH/Login.go"},
			},
		},
		{
			name: "file can match multiple areas",
			files: []types.ChangedFile{
				{Path: "services/payment/session.go"},
			},
			// matches payments (payment*/) AND auth (session*)
			wantAreas: []string{"payments", "auth"},
			wantFiles: map[string][]string{
				"payments": {"services/payment/session.go"},
				"auth":     {"services/payment/session.go"},
			},
		},
		{
			name: "deleted and binary flags preserved",
			files: []types.ChangedFile{
				{Path: "internal/auth/old.go", Deleted: true},
				{Path: "assets/image.png", Binary: true},
			},
			wantAreas: []string{"auth", "unclassified"},
			wantFiles: map[string][]string{
				"auth":         {"internal/auth/old.go"},
				"unclassified": {"assets/image.png"},
			},
		},
		{
			name:      "empty file list produces no areas",
			files:     []types.ChangedFile{},
			wantAreas: []string{},
			wantFiles: map[string][]string{},
		},
		{
			name: "areas sorted by priority then name",
			files: []types.ChangedFile{
				{Path: "README.md"},
				{Path: "db/schema.sql"},
				{Path: "internal/billing/invoice.go"},
			},
			wantAreas: []string{"payments", "db", "docs"},
			wantFiles: map[string][]string{
				"payments": {"internal/billing/invoice.go"},
				"db":       {"db/schema.sql"},
				"docs":     {"README.md"},
			},
		},
		{
			name: "files within area are sorted by path",
			files: []types.ChangedFile{
				{Path: "internal/auth/token.go"},
				{Path: "internal/auth/login.go"},
				{Path: "internal/auth/session.go"},
			},
			wantAreas: []string{"auth"},
			wantFiles: map[string][]string{
				"auth": {
					"internal/auth/login.go",
					"internal/auth/session.go",
					"internal/auth/token.go",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Match(tc.files, testRules)

			// Check area order.
			if len(got) != len(tc.wantAreas) {
				t.Fatalf("got %d areas %v, want %d areas %v", len(got), areaNames(got), len(tc.wantAreas), tc.wantAreas)
			}
			for i, ar := range got {
				if ar.Name != tc.wantAreas[i] {
					t.Errorf("areas[%d]: got %q, want %q", i, ar.Name, tc.wantAreas[i])
				}
			}

			// Check file lists per area.
			for _, ar := range got {
				wantPaths := tc.wantFiles[ar.Name]
				gotPaths := filePaths(ar.Files)
				if !strSliceEq(gotPaths, wantPaths) {
					t.Errorf("area %q files: got %v, want %v", ar.Name, gotPaths, wantPaths)
				}
			}
		})
	}
}

// TestDeterminism asserts that identical input always produces byte-identical JSON output.
func TestDeterminism(t *testing.T) {
	files := []types.ChangedFile{
		{Path: "internal/auth/login.go"},
		{Path: "cmd/app/main.go"},
		{Path: "db/migrations/0001.sql"},
		{Path: "README.md"},
		{Path: "internal/billing/invoice.go"},
		{Path: "assets/logo.png", Binary: true},
		{Path: "internal/auth/old.go", Deleted: true},
	}

	marshal := func() string {
		areas := Match(files, testRules)
		b, err := json.Marshal(areas)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(b)
	}

	first := marshal()
	for i := 0; i < 10; i++ {
		if got := marshal(); got != first {
			t.Fatalf("run %d produced different output:\ngot:  %s\nwant: %s", i+1, got, first)
		}
	}
}

// TestLoadRuleset verifies that the embedded default ruleset is valid, sorted,
// and returns a non-zero version.
func TestLoadRuleset(t *testing.T) {
	rules, version, err := LoadRuleset("")
	if err != nil {
		t.Fatalf("LoadRuleset: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected at least one rule")
	}
	if version <= 0 {
		t.Errorf("expected positive ruleset version, got %d", version)
	}
	for i := 1; i < len(rules); i++ {
		prev, curr := rules[i-1], rules[i]
		if curr.Priority < prev.Priority || (curr.Priority == prev.Priority && curr.Area < prev.Area) {
			t.Errorf("rules not sorted at index %d: %+v before %+v", i, prev, curr)
		}
	}
}

func areaNames(areas []types.AreaResult) []string {
	names := make([]string, len(areas))
	for i, a := range areas {
		names[i] = a.Name
	}
	return names
}

func filePaths(files []types.ChangedFile) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}

func strSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
