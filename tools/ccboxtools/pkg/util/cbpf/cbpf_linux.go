//go:build linux

package cbpf

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// syscallNrsTable resolves syscall names onto the linux wire numbers of the
// build arch's kernel ABI.
var syscallNrsTable = map[string]int{
	"bpf":               unix.SYS_BPF,
	"fanotify_init":     unix.SYS_FANOTIFY_INIT,
	"fsconfig":          unix.SYS_FSCONFIG,
	"fsmount":           unix.SYS_FSMOUNT,
	"fsopen":            unix.SYS_FSOPEN,
	"fspick":            unix.SYS_FSPICK,
	"lookup_dcookie":    unix.SYS_LOOKUP_DCOOKIE,
	"lsm_get_self_attr": unix.SYS_LSM_GET_SELF_ATTR,
	"lsm_list_modules":  unix.SYS_LSM_LIST_MODULES,
	"lsm_set_self_attr": unix.SYS_LSM_SET_SELF_ATTR,
	"mount":             unix.SYS_MOUNT,
	"mount_setattr":     unix.SYS_MOUNT_SETATTR,
	"move_mount":        unix.SYS_MOVE_MOUNT,
	"open_by_handle_at": unix.SYS_OPEN_BY_HANDLE_AT,
	"open_tree":         unix.SYS_OPEN_TREE,
	"perf_event_open":   unix.SYS_PERF_EVENT_OPEN,
	"quotactl":          unix.SYS_QUOTACTL,
	"quotactl_fd":       unix.SYS_QUOTACTL_FD,
	"setdomainname":     unix.SYS_SETDOMAINNAME,
	"sethostname":       unix.SYS_SETHOSTNAME,
	"setns":             unix.SYS_SETNS,
	"syslog":            unix.SYS_SYSLOG,
	"umount2":           unix.SYS_UMOUNT2,
	"unshare":           unix.SYS_UNSHARE,
}

func syscallNrs(names []string) ([]int, error) {
	nrs := make([]int, 0, len(names))
	for _, name := range names {
		nr, ok := syscallNrsTable[name]
		if !ok {
			return nil, fmt.Errorf("%s has no wire number", name)
		}
		nrs = append(nrs, nr)
	}
	return nrs, nil
}
