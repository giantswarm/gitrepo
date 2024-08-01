package gitrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// TopLevel Toplevel finds absolute path of top-level git directory. The output
// is the same as:
//
// `git rev-parse --show-toplevel`
func TopLevel(ctx context.Context, path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	f, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if !f.IsDir() {
		p = filepath.Dir(p)
	}

	for {
		f, err := os.Stat(filepath.Join(p, ".git"))
		if os.IsNotExist(err) {
			// Fall trough.
		} else if err != nil {
			return "", err
		} else if f.IsDir() {
			return p, nil
		}

		d := filepath.Dir(p)
		if p == d {
			break
		}

		p = d
	}

	return "", &executionFailedError{message: fmt.Sprintf("path %#q is not inside git repository", path)}
}
