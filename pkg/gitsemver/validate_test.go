package gitsemver

import "testing"

func Test_IsValidStable(t *testing.T) {
	t.Parallel()

	valid := []string{
		"1.2.3",
		"0.0.0",
		"10.20.30",
		"v1.2.3",
		"v0.0.0",
		"v10.20.30",
	}
	invalid := []string{
		"",
		"1.2",
		"1.2.3.4",
		"1.2.3-rc.1",
		"v1.2.3-rc.1",
		"1.2.3-dev.main.2026-01-27.09-49-59",
		"v1.2.3-dev.main.2026-01-27.09-49-59",
		"1.2.3-anything",
		"abc",
		"v",
		"1.2.x",
		// leading zeros in version components
		"01.2.3",
		"1.02.3",
		"1.2.03",
	}

	for _, v := range valid {
		if !IsValidStable(v) {
			t.Errorf("IsValidStable(%q) = false, want true", v)
		}
	}
	for _, v := range invalid {
		if IsValidStable(v) {
			t.Errorf("IsValidStable(%q) = true, want false", v)
		}
	}
}

func Test_IsValidRC(t *testing.T) {
	t.Parallel()

	valid := []string{
		"1.2.3-rc.1",
		"1.2.3-rc.0",
		"1.2.3-rc.42",
		"v1.2.3-rc.1",
		"v0.0.1-rc.99",
	}
	invalid := []string{
		"",
		"1.2.3",
		"v1.2.3",
		"1.2.3-rc",
		"1.2.3-rc.",
		"1.2.3-rc.a",
		"1.2.3-RC.1",
		"1.2.3-rc.1.extra",
		"1.2.3-dev.main.2026-01-27.09-49-59",
		"abc",
		// leading zeros
		"01.2.3-rc.1",
		"1.02.3-rc.1",
		"1.2.03-rc.1",
		"1.2.3-rc.01",
		"1.2.3-rc.00",
	}

	for _, v := range valid {
		if !IsValidRC(v) {
			t.Errorf("IsValidRC(%q) = false, want true", v)
		}
	}
	for _, v := range invalid {
		if IsValidRC(v) {
			t.Errorf("IsValidRC(%q) = true, want false", v)
		}
	}
}

func Test_IsValidDev(t *testing.T) {
	t.Parallel()

	valid := []string{
		// current schema: -b<8 hex>t<14 digits>c<7 hex>
		"1.9.2-b7b5b4fa7t20260127094959c1a2b3c4",
		"0.0.0-b00000000t20260127094959c0000000",
		"v1.9.2-b7b5b4fa7t20260127094959c1a2b3c4",
		// all-digit branch hash and commit hash (legal because the "b" and "c"
		// prefixes keep both parts alphanumeric)
		"1.2.4-b00123456t20260127094959c0012345",
		// legacy schema, still published on existing tags
		"1.2.3-dev.main.2026-01-27.09-49-59",
		"0.0.0-dev.main.2026-01-27.09-49-59",
		"1.2.4-dev.my-feature.2026-01-27.09-49-59",
		"v1.2.3-dev.main.2026-01-27.09-49-59",
		"v0.0.0-dev.main.2026-01-27.09-49-59",
		"2.0.1-dev.test-branch.2026-01-27.09-49-59",
		// branch names with hyphens
		"1.0.1-dev.feature-my-thing.2026-05-21.14-30-00",
		// single-char branch
		"1.0.0-dev.x.2026-05-21.00-00-00",
		// with the optional commit-hash segment
		"1.2.4-dev.my-feature.2026-01-27.09-49-59.h1a2b3c4",
		"v1.2.3-dev.main.2026-01-27.09-49-59.habc1234",
		// all-digit hash (legal because the "h" prefix keeps it alphanumeric)
		"1.2.4-dev.main.2026-01-27.09-49-59.h0012345",
		// middle-truncated branch with the "--" marker
		"1.2.4-dev.renovate-up--s-to-latest.2026-01-27.09-49-59.h1a2b3c4",
	}
	invalid := []string{
		"",
		"1.2.3",
		"1.2.3-rc.1",
		// current schema, branch hash not 8 lowercase hex characters
		"1.9.2-b7b5b4fat20260127094959c1a2b3c4",
		"1.9.2-b7b5b4fa7ft20260127094959c1a2b3c4",
		"1.9.2-b7B5B4FA7t20260127094959c1a2b3c4",
		// current schema, timestamp not 14 digits
		"1.9.2-b7b5b4fa7t2026012709495c1a2b3c4",
		"1.9.2-b7b5b4fa7t202601270949590c1a2b3c4",
		// current schema, commit hash not 7 lowercase hex characters
		"1.9.2-b7b5b4fa7t20260127094959c1a2b3c",
		"1.9.2-b7b5b4fa7t20260127094959c1a2b3c45",
		"1.9.2-b7b5b4fa7t20260127094959c1A2B3C4",
		// current schema, missing a part separator
		"1.9.2-b7b5b4fa720260127094959c1a2b3c4",
		"1.9.2-7b5b4fa7t20260127094959c1a2b3c4",
		// current schema must hold no "." and no "-" in the pre-release part
		"1.9.2-b7b5b4fa7.t20260127094959c1a2b3c4",
		"1.9.2-b7b5b4fa7t20260127094959-c1a2b3c4",
		// leading zeros in the version components
		"01.9.2-b7b5b4fa7t20260127094959c1a2b3c4",
		"1.09.2-b7b5b4fa7t20260127094959c1a2b3c4",
		"1.9.02-b7b5b4fa7t20260127094959c1a2b3c4",
		// missing time segment
		"1.2.3-dev.main.2026-01-27",
		// commit-hash segment missing the "h" prefix
		"1.2.3-dev.main.2026-01-27.09-49-59.1a2b3c4",
		// commit-hash too short / too long
		"1.2.3-dev.main.2026-01-27.09-49-59.h1a2b3c",
		"1.2.3-dev.main.2026-01-27.09-49-59.h1a2b3c45",
		// commit-hash not lowercase hex
		"1.2.3-dev.main.2026-01-27.09-49-59.hABC1234",
		"1.2.3-dev.main.2026-01-27.09-49-59.hxyz1234",
		// wrong date separator (slashes)
		"1.2.3-dev.main.2026/01/27.09-49-59",
		// wrong time separator (colons)
		"1.2.3-dev.main.2026-01-27.09:49:59",
		// date wrong length
		"1.2.3-dev.main.26-01-27.09-49-59",
		// branch with slash (not sanitized)
		"1.2.3-dev.feat/foo.2026-01-27.09-49-59",
		// empty branch
		"1.2.3-dev..2026-01-27.09-49-59",
		// extra trailing segment
		"1.2.3-dev.main.2026-01-27.09-49-59.extra",
		// no v prefix required but "vv" is wrong
		"vv1.2.3-dev.main.2026-01-27.09-49-59",
		// leading zeros in version components
		"01.2.3-dev.main.2026-01-27.09-49-59",
		"1.02.3-dev.main.2026-01-27.09-49-59",
		"1.2.03-dev.main.2026-01-27.09-49-59",
	}

	for _, v := range valid {
		if !IsValidDev(v) {
			t.Errorf("IsValidDev(%q) = false, want true", v)
		}
	}
	for _, v := range invalid {
		if IsValidDev(v) {
			t.Errorf("IsValidDev(%q) = true, want false", v)
		}
	}
}

func Test_IsValid(t *testing.T) {
	t.Parallel()

	valid := []string{
		// stable
		"1.2.3",
		"v1.2.3",
		// RC
		"1.2.3-rc.1",
		"v1.2.3-rc.1",
		// dev, current schema
		"1.9.2-b7b5b4fa7t20260127094959c1a2b3c4",
		"v1.9.2-b7b5b4fa7t20260127094959c1a2b3c4",
		// dev, legacy schema
		"1.2.4-dev.main.2026-01-27.09-49-59",
		"v1.2.4-dev.main.2026-01-27.09-49-59",
	}
	invalid := []string{
		"",
		"abc",
		"1.2",
		"1.2.3.4",
		"1.2.3-",
		"1.2.3-rc",
		"1.2.3-dev.main.2026-01-27",
	}

	for _, v := range valid {
		if !IsValid(v) {
			t.Errorf("IsValid(%q) = false, want true", v)
		}
	}
	for _, v := range invalid {
		if IsValid(v) {
			t.Errorf("IsValid(%q) = true, want false", v)
		}
	}
}
