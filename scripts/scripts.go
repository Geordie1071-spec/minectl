// Package scripts embeds the shell scripts the binary runs itself.
package scripts

import _ "embed"

// Uninstall is uninstall.sh, run by 'minectl uninstall'.
//
//go:embed uninstall.sh
var Uninstall string
