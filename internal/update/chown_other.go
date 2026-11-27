//go:build !unix

package update

import "os"

func chownLike(string, os.FileInfo) {}
