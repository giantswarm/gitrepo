package gitrepo

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/giantswarm/microerror"
)

func Test_TopLevel(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		inputPath           string
		expectedRelativeDir string
	}{
		{
			name:                "case 0",
			inputPath:           ".",
			expectedRelativeDir: "../..",
		},
		{
			name:                "case 1",
			inputPath:           "./repo.go",
			expectedRelativeDir: "../..",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			ctx := context.Background()

			abs, err := filepath.Abs(".")
			if err != nil {
				t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
			}

			expectedDir := filepath.Clean(filepath.Join(abs, tc.expectedRelativeDir))

			// Check with relative path.
			{
				dir, err := TopLevel(ctx, tc.inputPath)
				if err != nil {
					t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
				}

				if dir != expectedDir {
					t.Fatalf("dir = %v, want %v", dir, expectedDir)
				}
			}

			// Check with absolute path.
			{
				dir, err := TopLevel(ctx, filepath.Join(abs, tc.inputPath))
				if err != nil {
					t.Fatalf("err = %v, want %v", microerror.Stack(err), nil)
				}

				if dir != expectedDir {
					t.Fatalf("dir = %v, want %v", dir, expectedDir)
				}
			}
		})
	}
}
