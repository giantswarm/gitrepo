package gitrepo

import (
	"context"
	"os"
	"testing"
)

// TestResolveVersion tests ResolveVersion method which resolve a git reference and find the project version for it.
func TestResolveVersion(t *testing.T) {
	testCases := []struct {
		Ref             string
		ExpectedVersion string
	}{
		{
			// untagged version
			Ref:             "2e7604b8b3806b20ff305eb4e1a852c784ba34ca",
			ExpectedVersion: "v0.0.0-2e7604b8b3806b20ff305eb4e1a852c784ba34ca",
		},
		{
			// tagged version
			Ref:             "d1dcd7e42b044858f14ad51ea68e2809c16deb84",
			ExpectedVersion: "test-tag",
		},
		//{
		//      // above tagged version
		//	Ref:             "next sha",
		//	ExpectedVersion: "test-tag-d1dcd7e42b044858f14ad51ea68e2809c16deb84",
		//},
		{
			// branch reference
			Ref:             "test-branch",
			ExpectedVersion: "test-tag",
		},
		{
			// tag reference
			Ref:             "test-tag",
			ExpectedVersion: "test-tag",
		},
	}

	c := Config{
		AuthBasicToken: os.Getenv("GITHUB_BOT_TOKEN"),
		Dir:            "/tmp/gitrepo",
		URL:            "git@github.com:giantswarm/gitrepo.git",
	}
	repo, err := New(c)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	err = repo.EnsureUpToDate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range testCases {
		version, err := repo.ResolveVersion(ctx, test.Ref)
		if err != nil {
			t.Errorf("%#v", err)
		}
		if version != test.ExpectedVersion {
			t.Errorf("got %q, expected %q\n", version, test.ExpectedVersion)
		}
	}
}
