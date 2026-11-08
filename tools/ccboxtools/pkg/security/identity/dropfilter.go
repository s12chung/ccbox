package identity

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/cbpf"
)

// DropFilterPath is the file WriteDropFilter places the drop filter at —
// Drop's setpriv reads it.
const (
	DropFilterPath                 = "/run/ccbox/drop-filter.bpf"
	dropFilterDirMode  os.FileMode = 0o700 // root-only: the workload never reads or forges it
	dropFilterFileMode os.FileMode = 0o400
)

// dropFilterNames is the drop filter's deny list: the default profile's
// SYS_ADMIN-gated names.
var dropFilterNames = []string{
	"bpf", "fanotify_init", "fsconfig", "fsmount", "fsopen",
	"fspick", "lookup_dcookie", "lsm_get_self_attr", "lsm_list_modules",
	"lsm_set_self_attr", "mount", "mount_setattr", "move_mount",
	"open_by_handle_at", // its cap is not in the boot budget
	"open_tree", "perf_event_open", "quotactl",
	"quotactl_fd", "setdomainname", "sethostname", "setns", "syslog",
	// umount is out — no const on every build arch; the kernel's cap check is
	// its backstop
	"umount2", "unshare",
}

// WriteDropFilter writes the drop's seccomp backstop — the boot runs it while
// root, before Drop's setpriv stacks it.
func WriteDropFilter() error {
	prog, err := cbpf.RenderDenyList(dropFilterNames)
	if err != nil {
		return fmt.Errorf("drop filter: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(DropFilterPath), dropFilterDirMode); err != nil {
		return err
	}
	return os.WriteFile(DropFilterPath, prog, dropFilterFileMode)
}
