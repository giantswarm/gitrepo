package gitrepo

import (
	"context"
	"os"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	testCases := []struct {
		Ref             string
		ExpectedVersion string
	}{
		{
			Ref:             "2e7604b8b3806b20ff305eb4e1a852c784ba34ca",
			ExpectedVersion: "v0.0.0-2e7604b8b3806b20ff305eb4e1a852c784ba34ca",
		},
		{
			Ref:             "d1dcd7e42b044858f14ad51ea68e2809c16deb84",
			ExpectedVersion: "test",
		},
		//{
		//	Ref:             "next sha",
		//	ExpectedVersion: "test-next sha",
		//},
	}

	c := Config{
		AuthBasicToken: os.Getenv("GITHUB_TOKEN"),
		Dir:            "../../.git",
		URL:            "git@github.com:giantswarm/gitrepo.git",
	}
	repo, err := New(c)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

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
