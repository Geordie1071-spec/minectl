//go:build unix

package update

import (
	"os"
	"syscall"
)

func chownLike(path string, fi os.FileInfo) {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		_ = os.Chown(path, int(st.Uid), int(st.Gid))
	}
}
