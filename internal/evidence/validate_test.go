package evidence

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParityCorpus(t *testing.T) {
	type manifestCase struct {
		Path  string `json:"path"`
		Valid bool   `json:"valid"`
	}
	type manifest struct {
		Cases []manifestCase `json:"cases"`
	}

	corpusDir := filepath.Join("..", "..", "testdata", "evidence-contract")
	manifestData, err := os.ReadFile(filepath.Join(corpusDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var corpus manifest
	if err := json.Unmarshal(manifestData, &corpus); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	expectedReason := map[string]string{
		"invalid-pass-actionable.json":          "PASS cannot contain actionable findings",
		"invalid-high-confidence-observed.json": "HIGH_CONFIDENCE requires a STRONG or UNAVOIDABLE code path",
		"invalid-near-integer-pr.json":          "must be an integer",
		"invalid-duplicate-key.json":            "duplicate object key",
	}
	for _, testCase := range corpus.Cases {
		t.Run(testCase.Path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(corpusDir, testCase.Path))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			err = Validate(data)
			if testCase.Valid && err != nil {
				t.Fatalf("valid fixture rejected: %v", err)
			}
			if !testCase.Valid && err == nil {
				t.Fatal("invalid fixture admitted")
			}
			if reason := expectedReason[testCase.Path]; reason != "" && !strings.Contains(err.Error(), reason) {
				t.Fatalf("rejected for the wrong reason: got %q, want substring %q", err, reason)
			}
		})
	}
}

func TestCanonicalBenchmark(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".review", "examples", "benchmark-001.json"))
	if err != nil {
		t.Fatalf("read benchmark: %v", err)
	}
	if err := Validate(data); err != nil {
		t.Fatalf("canonical benchmark rejected: %v", err)
	}
}

func TestAuthoritativeSchemaHasNotDrifted(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".review", "review-evidence.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	canonical, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("canonicalize schema: %v", err)
	}
	const expected = "a08f3e39ec8670459228bddac15a396ba3e6dc76cf7efcb74e6610ca2ab5fbf7"
	if got := fmt.Sprintf("%x", sha256.Sum256(canonical)); got != expected {
		t.Fatalf("authoritative schema changed: got SHA-256 %s, want %s; update the structural validator deliberately", got, expected)
	}
}

func TestStrictParserRejectsDuplicateKeysRecursively(t *testing.T) {
	tests := map[string]string{
		"root":               `{"schema_version":"3","schema_version":"3"}`,
		"nested":             `{"outer":{"key":1,"key":2}}`,
		"array object":       `[{"key":1,"key":2}]`,
		"escaped equivalent": `{"pr":1,"\u0070r":2}`,
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parse([]byte(input))
			if err == nil || !strings.Contains(err.Error(), "duplicate object key") {
				t.Fatalf("got %v, want duplicate-key rejection", err)
			}
		})
	}
}

func TestStrictParserPreservesSurrogateKeyIdentity(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		duplicate bool
	}{
		{name: "distinct lone surrogates", input: `{"\uD800":1,"\uD801":2}`},
		{name: "lone surrogate differs from replacement character", input: `{"\uD800":1,"\uFFFD":2}`},
		{name: "identical lone surrogate", input: `{"\uD800":1,"\ud800":2}`, duplicate: true},
		{name: "surrogate pair equals literal scalar", input: "{\"\\uD83D\\uDE00\":1,\"😀\":2}", duplicate: true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := parse([]byte(testCase.input))
			if testCase.duplicate && (err == nil || !strings.Contains(err.Error(), "duplicate object key")) {
				t.Fatalf("got %v, want duplicate-key rejection", err)
			}
			if !testCase.duplicate && err != nil {
				t.Fatalf("distinct submitted keys rejected: %v", err)
			}
		})
	}
}

func TestSurrogateStringsPreserveReferenceLength(t *testing.T) {
	tests := []struct {
		name    string
		baseSHA string
		valid   bool
	}{
		{name: "three lone surrogates", baseSHA: `\uD800\uD801\uD802`},
		{name: "seven lone surrogates", baseSHA: `\uD800\uD801\uD802\uD803\uD804\uD805\uD806`, valid: true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			document := []byte(fmt.Sprintf(`{
				"schema_version":"3",
				"review":{"repository":"r","pr":1,"base_sha":"%s","head_sha":"2222222","verdict":"PASS"},
				"verification":[{"id":"v","type":"test","required":true,"outcome":"PASS","detail":"passed"}],
				"findings":[]
			}`, testCase.baseSHA))
			err := Validate(document)
			if testCase.valid && err != nil {
				t.Fatalf("reference-length string rejected: %v", err)
			}
			if !testCase.valid && (err == nil || !strings.Contains(err.Error(), "at least 7 character")) {
				t.Fatalf("got %v, want minimum-length rejection", err)
			}
		})
	}
}

func TestStrictParserRejectsInvalidRepresentations(t *testing.T) {
	tests := map[string][]byte{
		"NaN":           []byte(`{"value":NaN}`),
		"positive inf":  []byte(`{"value":Infinity}`),
		"negative inf":  []byte(`{"value":-Infinity}`),
		"trailing JSON": []byte(`{} {}`),
		"invalid UTF-8": {'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parse(input); err == nil {
				t.Fatal("invalid representation was parsed")
			}
		})
	}
}

func TestExactIntegerSemantics(t *testing.T) {
	hugeInteger := strings.Repeat("9", 10_000)
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{name: "ordinary", token: "1", valid: true},
		{name: "decimal integral", token: "1.0", valid: true},
		{name: "positive exponent integral", token: "1e400", valid: true},
		{name: "negative exponent integral", token: "10e-1", valid: true},
		{name: "large integer", token: hugeInteger, valid: true},
		{name: "zero", token: "0", valid: false},
		{name: "negative", token: "-1", valid: false},
		{name: "fraction", token: "1.5", valid: false},
		{name: "near integer", token: "0.99999999999999999", valid: false},
		{name: "non-integral exponent", token: "1e-1", valid: false},
		{name: "large negative", token: "-" + hugeInteger, valid: false},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			document := []byte(fmt.Sprintf(`{
				"schema_version":"3",
				"review":{"repository":"r","pr":%s,"base_sha":"1111111","head_sha":"2222222","verdict":"PASS"},
				"verification":[{"id":"v","type":"test","required":true,"outcome":"PASS","detail":"passed"}],
				"findings":[]
			}`, testCase.token))
			err := Validate(document)
			if testCase.valid && err != nil {
				t.Fatalf("exact integer rejected: %v", err)
			}
			expectedError := "must be an integer"
			if testCase.token == "0" || strings.HasPrefix(testCase.token, "-") {
				expectedError = "must be at least 1"
			}
			if !testCase.valid && (err == nil || !strings.Contains(err.Error(), expectedError)) {
				t.Fatalf("got %v, want numeric schema rejection", err)
			}
		})
	}
}

func TestSchemaBoundary(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(object)
		want   string
	}{
		{name: "boolean is not integer", mutate: func(root object) { root["review"].(object)["pr"] = true }, want: "must be an integer"},
		{name: "verification rejects extra", mutate: func(root object) { root["verification"].(array)[0].(object)["extra"] = true }, want: "additional property"},
		{name: "location rejects extra", mutate: func(root object) {
			root["findings"] = array{possibleFinding()}
			root["findings"].(array)[0].(object)["locations"].(array)[0].(object)["line"] = number{raw: "1"}
		}, want: "additional property"},
		{name: "code path requires qualification", mutate: func(root object) {
			finding := possibleFinding()
			finding["evidence"] = array{object{"type": "code_path", "detail": "path"}}
			root["findings"] = array{finding}
		}, want: "missing required property"},
		{name: "qualification forbidden elsewhere", mutate: func(root object) {
			finding := possibleFinding()
			finding["evidence"] = array{object{"type": "history", "detail": "history", "qualification": "STRONG"}}
			root["findings"] = array{finding}
		}, want: "only allowed for code_path"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := passDocument()
			testCase.mutate(root)
			err := validateObject(root)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("got %v, want substring %q", err, testCase.want)
			}
		})
	}
}

func TestIDPatternsMatchAcceptedRegexBehavior(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(object)
		valid  bool
	}{
		{name: "verification final newline", mutate: func(root object) {
			root["verification"].(array)[0].(object)["id"] = "required\n"
		}, valid: true},
		{name: "verification two final newlines", mutate: func(root object) {
			root["verification"].(array)[0].(object)["id"] = "required\n\n"
		}},
		{name: "finding final newline", mutate: func(root object) {
			finding := possibleFinding()
			finding["id"] = "DI-REV-001\n"
			root["findings"] = array{finding}
		}, valid: true},
		{name: "finding two final newlines", mutate: func(root object) {
			finding := possibleFinding()
			finding["id"] = "DI-REV-001\n\n"
			root["findings"] = array{finding}
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := passDocument()
			testCase.mutate(root)
			err := validateObject(root)
			if testCase.valid && err != nil {
				t.Fatalf("schema-compatible ID rejected: %v", err)
			}
			if !testCase.valid && (err == nil || !strings.Contains(err.Error(), "required pattern")) {
				t.Fatalf("got %v, want ID pattern rejection", err)
			}
		})
	}
}

func TestSemanticAdmissionPolicy(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(object)
		valid  bool
		want   string
	}{
		{name: "PASS", mutate: func(object) {}, valid: true},
		{name: "PASS optional failure ignored", mutate: func(root object) {
			root["verification"] = append(root["verification"].(array), verification("optional", false, "FAIL"))
		}, valid: true},
		{name: "PASS requires required verification", mutate: func(root object) { root["verification"] = array{} }, want: "at least one required verification"},
		{name: "PASS requires all required to pass", mutate: func(root object) { root["verification"].(array)[0].(object)["outcome"] = "FAIL" }, want: "every required verification"},
		{name: "PASS permits non-actionable", mutate: func(root object) { root["findings"] = array{possibleFinding()} }, valid: true},
		{name: "PASS permits STYLE", mutate: func(root object) {
			finding := possibleFinding()
			finding["classification"] = "STYLE"
			root["findings"] = array{finding}
		}, valid: true},
		{name: "PASS forbids actionable", mutate: func(root object) { root["findings"] = array{provenUnavoidable()} }, want: "PASS cannot contain actionable"},
		{name: "FAIL requires actionable", mutate: func(root object) { root["review"].(object)["verdict"] = "FAIL" }, want: "FAIL requires at least one"},
		{name: "FAIL with unavoidable PROVEN", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			root["findings"] = array{provenUnavoidable()}
		}, valid: true},
		{name: "PROVEN with completed executable", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := actionableFinding("PROVEN")
			finding["evidence"] = array{object{"type": "test", "detail": "reproduced", "head": "FAIL"}}
			root["findings"] = array{finding}
		}, valid: true},
		{name: "PROVEN with completed base only", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := actionableFinding("PROVEN")
			finding["evidence"] = array{object{"type": "test", "detail": "reproduced", "base": "PASS"}}
			root["findings"] = array{finding}
		}, valid: true},
		{name: "PROVEN with completed base and incomplete head", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := actionableFinding("PROVEN")
			finding["evidence"] = array{object{"type": "test", "detail": "reproduced", "base": "FAIL", "head": "UNKNOWN"}}
			root["findings"] = array{finding}
		}, valid: true},
		{name: "PROVEN rejects unexecuted neighbor", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := provenUnavoidable()
			finding["evidence"] = append(finding["evidence"].(array), object{"type": "test", "detail": "not run", "head": "NOT_RUN"})
			root["findings"] = array{finding}
		}, want: "must include at least one structured PASS or FAIL outcome"},
		{name: "PROVEN requires qualifying evidence", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := actionableFinding("PROVEN")
			finding["evidence"] = array{object{"type": "code_path", "qualification": "STRONG", "detail": "strong"}}
			root["findings"] = array{finding}
		}, want: "PROVEN findings require executed evidence"},
		{name: "HIGH_CONFIDENCE with STRONG", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := actionableFinding("HIGH_CONFIDENCE")
			finding["evidence"] = array{object{"type": "code_path", "qualification": "STRONG", "detail": "strong"}}
			root["findings"] = array{finding}
		}, valid: true},
		{name: "HIGH_CONFIDENCE rejects OBSERVED", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := actionableFinding("HIGH_CONFIDENCE")
			finding["evidence"] = array{object{"type": "code_path", "qualification": "OBSERVED", "detail": "observed"}}
			root["findings"] = array{finding}
		}, want: "HIGH_CONFIDENCE requires a STRONG"},
		{name: "HIGH_CONFIDENCE ignores incomplete executable context", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := actionableFinding("HIGH_CONFIDENCE")
			finding["evidence"] = array{
				object{"type": "code_path", "qualification": "STRONG", "detail": "strong"},
				object{"type": "test", "detail": "context", "base": "UNKNOWN", "head": "NOT_RUN"},
			}
			root["findings"] = array{finding}
		}, valid: true},
		{name: "actionable reproduction required", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := provenUnavoidable()
			finding["reproduction"] = array{}
			root["findings"] = array{finding}
		}, want: "require reproduction steps"},
		{name: "actionable attempted disproof required", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			finding := provenUnavoidable()
			finding["attempted_disproof"] = array{}
			root["findings"] = array{finding}
		}, want: "require at least one attempted disproof"},
		{name: "INCONCLUSIVE NOT_RUN", mutate: func(root object) {
			root["review"].(object)["verdict"] = "INCONCLUSIVE"
			root["verification"].(array)[0].(object)["outcome"] = "NOT_RUN"
		}, valid: true},
		{name: "INCONCLUSIVE UNKNOWN", mutate: func(root object) {
			root["review"].(object)["verdict"] = "INCONCLUSIVE"
			root["verification"].(array)[0].(object)["outcome"] = "UNKNOWN"
		}, valid: true},
		{name: "INCONCLUSIVE ignores optional incomplete", mutate: func(root object) {
			root["review"].(object)["verdict"] = "INCONCLUSIVE"
			root["verification"] = append(root["verification"].(array), verification("optional", false, "UNKNOWN"))
		}, want: "requires a required verification"},
		{name: "INCONCLUSIVE forbids actionable", mutate: func(root object) {
			root["review"].(object)["verdict"] = "INCONCLUSIVE"
			root["verification"].(array)[0].(object)["outcome"] = "UNKNOWN"
			root["findings"] = array{provenUnavoidable()}
		}, want: "INCONCLUSIVE cannot contain actionable"},
		{name: "duplicate verification ids", mutate: func(root object) {
			root["verification"] = append(root["verification"].(array), verification("required", false, "PASS"))
		}, want: "duplicate id"},
		{name: "duplicate finding ids", mutate: func(root object) {
			root["findings"] = array{possibleFinding(), possibleFinding()}
		}, want: "duplicate id"},
		{name: "invalid finding neighbor is not masked", mutate: func(root object) {
			root["review"].(object)["verdict"] = "FAIL"
			invalid := actionableFinding("HIGH_CONFIDENCE")
			invalid["id"] = "DI-REV-002"
			invalid["evidence"] = array{object{"type": "code_path", "qualification": "OBSERVED", "detail": "observed"}}
			root["findings"] = array{provenUnavoidable(), invalid}
		}, want: "HIGH_CONFIDENCE requires a STRONG"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := passDocument()
			testCase.mutate(root)
			err := validateObject(root)
			if testCase.valid && err != nil {
				t.Fatalf("valid policy case rejected: %v", err)
			}
			if !testCase.valid && err == nil {
				t.Fatal("invalid policy case admitted")
			}
			if testCase.want != "" && !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("got %q, want substring %q", err, testCase.want)
			}
		})
	}
}

func TestSemanticStringsMustContainNonWhitespaceText(t *testing.T) {
	withFinding := func(mutate func(object)) func(object) {
		return func(root object) {
			finding := possibleFinding()
			mutate(finding)
			root["findings"] = array{finding}
		}
	}
	tests := []struct {
		name   string
		mutate func(object)
	}{
		{name: "review repository", mutate: func(root object) { root["review"].(object)["repository"] = " " }},
		{name: "review base sha", mutate: func(root object) { root["review"].(object)["base_sha"] = "       " }},
		{name: "review head sha", mutate: func(root object) { root["review"].(object)["head_sha"] = "       " }},
		{name: "verification detail", mutate: func(root object) { root["verification"].(array)[0].(object)["detail"] = "\t" }},
		{name: "verification command", mutate: func(root object) { root["verification"].(array)[0].(object)["command"] = "\n" }},
		{name: "finding claim", mutate: withFinding(func(finding object) { finding["claim"] = " " })},
		{name: "finding affected behavior", mutate: withFinding(func(finding object) { finding["affected_behavior"] = "\u00a0" })},
		{name: "finding expected", mutate: withFinding(func(finding object) { finding["expected"] = "\t" })},
		{name: "finding observed", mutate: withFinding(func(finding object) { finding["observed"] = "\n" })},
		{name: "finding remaining uncertainty empty", mutate: withFinding(func(finding object) { finding["remaining_uncertainty"] = "" })},
		{name: "finding remaining uncertainty whitespace", mutate: withFinding(func(finding object) { finding["remaining_uncertainty"] = " " })},
		{name: "location path", mutate: withFinding(func(finding object) { finding["locations"].(array)[0].(object)["path"] = " " })},
		{name: "location symbol", mutate: withFinding(func(finding object) { finding["locations"].(array)[0].(object)["symbol"] = "" })},
		{name: "evidence detail", mutate: withFinding(func(finding object) { finding["evidence"].(array)[0].(object)["detail"] = "\t" })},
		{name: "reproduction record", mutate: withFinding(func(finding object) { finding["reproduction"] = array{" "} })},
		{name: "attempted disproof record", mutate: withFinding(func(finding object) { finding["attempted_disproof"] = array{"\n"} })},
		{name: "Python information separator whitespace", mutate: withFinding(func(finding object) { finding["claim"] = "\u001c" })},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := passDocument()
			testCase.mutate(root)
			err := validateObject(root)
			if err == nil || !strings.Contains(err.Error(), "must contain non-whitespace text") {
				t.Fatalf("got %v, want semantic nonblank rejection", err)
			}
		})
	}
}

func TestValidationErrorsAreDeterministic(t *testing.T) {
	data := []byte(`{"schema_version":"3","review":{},"verification":[],"findings":[]}`)
	first := Validate(data)
	if first == nil {
		t.Fatal("invalid document admitted")
	}
	for run := 0; run < 10; run++ {
		if got := Validate(data); got == nil || got.Error() != first.Error() {
			t.Fatalf("run %d: got %v, want %v", run, got, first)
		}
	}
}

func validateObject(root object) error {
	data, err := json.Marshal(root)
	if err != nil {
		return err
	}
	return Validate(data)
}

func passDocument() object {
	return object{
		"schema_version": "3",
		"review": object{
			"repository": "admbahm/PushBadger",
			"pr":         json.Number("1"),
			"base_sha":   "1111111",
			"head_sha":   "2222222",
			"verdict":    "PASS",
		},
		"verification": array{verification("required", true, "PASS")},
		"findings":     array{},
	}
}

func verification(id string, required bool, outcome string) object {
	return object{"id": id, "type": "test", "required": required, "outcome": outcome, "detail": "detail"}
}

func possibleFinding() object {
	finding := actionableFinding("POSSIBLE")
	finding["evidence"] = array{object{"type": "history", "detail": "history"}}
	return finding
}

func provenUnavoidable() object {
	finding := actionableFinding("PROVEN")
	finding["evidence"] = array{object{"type": "code_path", "qualification": "UNAVOIDABLE", "detail": "unavoidable"}}
	return finding
}

func actionableFinding(classification string) object {
	return object{
		"id":                    "DI-REV-001",
		"classification":        classification,
		"severity":              "high",
		"claim":                 "claim",
		"affected_behavior":     "behavior",
		"locations":             array{object{"path": "file.go"}},
		"evidence":              array{object{"type": "history", "detail": "history"}},
		"expected":              "expected",
		"observed":              "observed",
		"reproduction":          array{"step"},
		"attempted_disproof":    array{"attempt"},
		"remaining_uncertainty": "none",
	}
}
