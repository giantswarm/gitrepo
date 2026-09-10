package gitsemver

import (
	"strings"
	"testing"
	"time"
)

func Test_BranchHash(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		branch   string
		expected string
	}{
		// The RFC's own example. It pins the implementation to CRC-32/ISO-HDLC;
		// another CRC32 variant (e.g. the one POSIX cksum uses) gives a
		// different value here.
		{"RFC example", "my-feature", "7b5b4fa7"},
		// The raw branch name is hashed, slash and all — no sanitizing first.
		{"unsanitized name", "renovate/update-all-dependencies-to-latest", "08a93c50"},
		{"empty name", "", "00000000"},
	}

	for _, tc := range cases {
		got := BranchHash(tc.branch)
		if got != tc.expected {
			t.Errorf("%s: BranchHash(%q) = %q, want %q", tc.name, tc.branch, got, tc.expected)
		}
		if len(got) != 8 {
			t.Errorf("%s: BranchHash(%q) = %q, want 8 characters", tc.name, tc.branch, got)
		}
	}
}

func Test_buildDevVersion(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 1, 27, 9, 49, 59, 0, time.UTC)
	sha := "1a2b3c4d5e6f7a8b9c0d"

	t.Run("matches the RFC example", func(t *testing.T) {
		got := buildDevVersion("1.9.2", "my-feature", sha, ts)
		want := "1.9.2-b7b5b4fa7t20260127094959c1a2b3c4"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if !IsValidDev(got) {
			t.Errorf("%q is not a valid dev version", got)
		}
	})

	t.Run("the pre-release part is fixed width and label-safe", func(t *testing.T) {
		branches := []string{
			"x",
			"my-feature",
			"renovate/update-all-dependencies-to-latest",
			strings.Repeat("very-long-branch-name/", 20),
		}
		for _, base := range []string{"0.0.0", "1.2.4", "100.200.3456"} {
			for _, branch := range branches {
				got := buildDevVersion(base, branch, sha, ts)
				pre, found := strings.CutPrefix(got, base)
				if !found {
					t.Fatalf("base %s, branch %q: %q does not start with the base", base, branch, got)
				}
				// "-" + "b" + 8 + "t" + 14 + "c" + 7 = 33.
				if len(pre) != 33 {
					t.Errorf("base %s, branch %q: pre-release %q has length %d, want 33", base, branch, pre, len(pre))
				}
				// A trim of a concatenated label must never be able to cut on
				// a "." or a "-", so neither may appear after the leading "-".
				if strings.ContainsAny(pre[1:], ".-") {
					t.Errorf("base %s, branch %q: pre-release %q must hold no %q and no %q", base, branch, pre, ".", "-")
				}
				if !IsValidDev(got) {
					t.Errorf("base %s, branch %q: %q is not a valid dev version", base, branch, got)
				}
			}
		}
	})

	t.Run("commit hash uses the 7-char short form", func(t *testing.T) {
		got := buildDevVersion("0.0.0", "main", sha, ts)
		if !strings.HasSuffix(got, "c1a2b3c4") {
			t.Errorf("got %q, want trailing c1a2b3c4", got)
		}
	})

	t.Run("a short commit hash is embedded as is", func(t *testing.T) {
		got := buildDevVersion("0.0.0", "main", "abc", ts)
		if !strings.HasSuffix(got, "cabc") {
			t.Errorf("got %q, want trailing cabc", got)
		}
	})

	t.Run("per-branch order follows the timestamp", func(t *testing.T) {
		older := buildDevVersion("1.2.4", "feature", sha, ts)
		newer := buildDevVersion("1.2.4", "feature", sha, ts.Add(time.Hour))
		// Same branch and hash, so the "b<hash>t" prefix is constant and the
		// fixed-width timestamp decides the order. semVer compares this single
		// alphanumeric identifier lexically, which matches chronological order.
		if older >= newer {
			t.Errorf("expected %q < %q", older, newer)
		}
	})

	t.Run("different branches get different tags at the same second", func(t *testing.T) {
		a := buildDevVersion("1.2.4", "feature-a", sha, ts)
		b := buildDevVersion("1.2.4", "feature-b", sha, ts)
		if a == b {
			t.Errorf("branches feature-a and feature-b both produced %q", a)
		}
	})
}
