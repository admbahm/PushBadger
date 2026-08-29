package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCommand(t *testing.T) {
	validFixture := filepath.Join("..", "..", "testdata", "evidence-contract", "valid-pass.json")
	invalidFixture := filepath.Join("..", "..", "testdata", "evidence-contract", "invalid-duplicate-key.json")

	t.Run("admits valid evidence", func(t *testing.T) {
		stdout, stderr, err := executeRoot(t, "validate", validFixture)
		if err != nil {
			t.Fatalf("validate: %v; stderr: %s", err, stderr)
		}
		if stdout != "valid\n" || stderr != "" {
			t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("rejects invalid evidence", func(t *testing.T) {
		stdout, stderr, err := executeRoot(t, "validate", invalidFixture)
		if !errors.Is(err, errEvidenceInvalid) {
			t.Fatalf("got error %v, want errEvidenceInvalid", err)
		}
		if stdout != "" || !strings.Contains(stderr, "duplicate object key") {
			t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("requires exactly one path", func(t *testing.T) {
		_, _, err := executeRoot(t, "validate")
		if err == nil || errors.Is(err, errEvidenceInvalid) {
			t.Fatalf("got error %v, want usage error", err)
		}
		_, _, err = executeRoot(t, "validate", validFixture, invalidFixture)
		if err == nil || errors.Is(err, errEvidenceInvalid) {
			t.Fatalf("got error %v, want usage error", err)
		}
	})

	t.Run("rejects unreadable path", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.json")
		stdout, stderr, err := executeRoot(t, "validate", missing)
		if !errors.Is(err, errEvidenceInvalid) {
			t.Fatalf("got error %v, want errEvidenceInvalid", err)
		}
		if stdout != "" || !strings.Contains(stderr, "could not read evidence file") {
			t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
		}
	})
}

func TestValidateCommandDoesNotRequireGitRepository(t *testing.T) {
	document, err := os.ReadFile(filepath.Join("..", "..", "testdata", "evidence-contract", "valid-pass.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(path, document, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	stdout, stderr, err := executeRoot(t, "validate", path)
	if err != nil || stdout != "valid\n" || stderr != "" {
		t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}

func executeRoot(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := rootCmd(1)
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs(args)
	err := command.Execute()
	return stdout.String(), stderr.String(), err
}
