package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func Test_runValidate_valid(t *testing.T) {
	t.Parallel()
	if err := runValidate("any", "1.2.3"); err != nil {
		t.Errorf("runValidate(any, 1.2.3) = %v, want nil", err)
	}
}

func Test_runValidate_invalid(t *testing.T) {
	t.Parallel()
	if err := runValidate("any", "not-a-version"); !errors.Is(err, errInvalidVersion) {
		t.Errorf("runValidate(any, not-a-version) = %v, want errInvalidVersion", err)
	}
}

func Test_runValidate_typeFlag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		typFlag string
		version string
		wantErr bool
	}{
		{"stable", "1.2.3", false},
		{"rc", "1.2.3-rc.1", false},
		{"stable", "1.2.3-rc.1", true},
		{"unknown", "1.2.3", true},
	}
	for _, tc := range cases {
		got := runValidate(tc.typFlag, tc.version)
		if (got != nil) != tc.wantErr {
			t.Errorf("runValidate(%q, %q) err=%v, wantErr=%v", tc.typFlag, tc.version, got, tc.wantErr)
		}
	}
}

func Test_runNext_lastTag_patch(t *testing.T) {
	t.Parallel()
	if err := runNext("patch", "v1.2.3"); err != nil {
		t.Errorf("runNext(patch, v1.2.3) = %v, want nil", err)
	}
}

func Test_runNext_lastTag_rc(t *testing.T) {
	t.Parallel()
	if err := runNext("rc", "v1.2.3-rc.1"); err != nil {
		t.Errorf("runNext(rc, v1.2.3-rc.1) = %v, want nil", err)
	}
}

func Test_runNext_unknownBumpType(t *testing.T) {
	t.Parallel()
	if err := runNext("bogus", "v1.2.3"); err == nil {
		t.Error("runNext with unknown bump type should return error")
	}
}

func Test_runBranchHash_explicitBranch(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := runBranchHash(&out, []string{"my-feature"}); err != nil {
		t.Fatalf("runBranchHash(my-feature) = %v, want nil", err)
	}
	// The fingerprint of "my-feature" from the RFC's own example.
	if got := strings.TrimSpace(out.String()); got != "7b5b4fa7" {
		t.Errorf("runBranchHash printed %q, want %q", got, "7b5b4fa7")
	}
}

func Test_runBranchHash_currentBranch(t *testing.T) {
	// Not parallel: t.Setenv pins the branch so the test does not depend on
	// which branch the repository is checked out on.
	t.Setenv("GS_BRANCH_NAME", "my-feature")
	var out bytes.Buffer
	if err := runBranchHash(&out, nil); err != nil {
		t.Fatalf("runBranchHash(nil) = %v, want nil", err)
	}
	if got := strings.TrimSpace(out.String()); got != "7b5b4fa7" {
		t.Errorf("runBranchHash printed %q, want %q", got, "7b5b4fa7")
	}
}

func Test_newRootCmd_registersBranchHash(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"get", "next", "validate", "branch-hash"} {
		if _, _, err := newRootCmd().Find([]string{name}); err != nil {
			t.Errorf("newRootCmd() does not register %q: %v", name, err)
		}
	}
}
