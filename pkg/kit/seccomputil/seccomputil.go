// Package seccomputil renders the boot's seccomp security-opt — the userns
// door mounted on the engine's default profile.
package seccomputil

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/moby/profiles/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// The door's Linux wire numbers, as consts so the host binary builds on
// every platform — x/sys/unix gates CLONE_NEWUSER to linux.
const (
	cloneNewUser uint64 = 0x10000000
	errnoPerm    uint   = 1
	errnoNoSys   uint   = 38
)

var (
	// s390Arches names the libseccomp arches whose clone ABI orders the flags at
	// arg1; every other ABI, unshare included, carries them at arg0.
	s390Arches = []string{"s390", "s390x"}

	// the carried render of the library's DefaultProfile — re-render on engine
	// bumps: the profile only grows upstream
	//
	//go:generate go run ../../../tools/seccompgen/main.go -o seccomp_default.json
	//go:embed seccomp_default.json
	seccompDefaultJSON []byte
)

// BootOpt renders the boot's seccomp security-opt.
func BootOpt() (string, error) {
	profile, err := bootProfile()
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(profile)
	if err != nil {
		return "", fmt.Errorf("seccomp profile: %w", err)
	}
	return "seccomp=" + string(out), nil
}

// bootProfile mounts the userns door on the engine's default profile: a
// whitelist
func bootProfile() (*seccomp.Seccomp, error) {
	profile := &seccomp.Seccomp{}
	if err := json.Unmarshal(seccompDefaultJSON, profile); err != nil {
		return nil, fmt.Errorf("seccomp default profile: %w", err)
	}
	profile.Syscalls = mergeSyscalls(doorOverlay(), profile.Syscalls)
	return profile, nil
}

// doorOverlay carries the door's allows and its denies.
func doorOverlay() []*seccomp.Syscall {
	return []*seccomp.Syscall{
		withAllowUserNS(&seccomp.Syscall{
			Excludes:     &seccomp.Filter{Arches: s390Arches},
			LinuxSyscall: specs.LinuxSyscall{Names: []string{"clone"}},
		}, 0),
		withAllowUserNS(&seccomp.Syscall{
			Includes:     &seccomp.Filter{Arches: s390Arches},
			LinuxSyscall: specs.LinuxSyscall{Names: []string{"clone"}},
		}, 1),
		withAllowUserNS(&seccomp.Syscall{
			LinuxSyscall: specs.LinuxSyscall{Names: []string{"unshare"}},
		}, 0),
		// clone3 is blocked outright — the filter can't see its flags; glibc
		// threads read the ENOSYS as an old kernel and fall back to clone, where
		// the userns denial still applies
		{LinuxSyscall: specs.LinuxSyscall{Names: []string{"clone3"}, Action: specs.ActErrno, ErrnoRet: new(errnoNoSys)}},
		// io_uring's operations never pass seccomp's syscall entry, so the ring
		// is blocked — apps read the ENOSYS as "unsupported" and fall back to
		// policed plain syscalls
		{LinuxSyscall: specs.LinuxSyscall{
			Names: []string{"io_uring_setup", "io_uring_enter", "io_uring_register"}, Action: specs.ActErrno, ErrnoRet: new(errnoNoSys),
		}},
		// the ptrace family is denied outright — sessions share a uid and the
		// kernel has no Yama, so its calls would read sibling sessions' memory,
		// LLM keys included; the names also trim the default profile's ptrace allows
		{LinuxSyscall: specs.LinuxSyscall{
			Names:  []string{"ptrace", "process_vm_readv", "process_vm_writev", "pidfd_getfd", "process_madvise", "kcmp"},
			Action: specs.ActErrno, ErrnoRet: new(errnoPerm),
		}},
	}
}

// withAllowUserNS returns the syscall with allow perms for every use but a
// userns request — that one falls to the whitelist's EPERM
func withAllowUserNS(rule *seccomp.Syscall, index uint) *seccomp.Syscall {
	rule.Action = specs.ActAllow
	rule.Args = []specs.LinuxSeccompArg{{Index: index, Value: cloneNewUser, ValueTwo: 0, Op: specs.OpMaskedEqual}}
	return rule
}

// mergeSyscalls merges src into dest, trimming the syscall names dest carries
func mergeSyscalls(dest, src []*seccomp.Syscall) []*seccomp.Syscall {
	names := map[string]bool{}
	for _, rule := range dest {
		for _, name := range rule.Names {
			names[name] = true
		}
	}
	kept := dest
	for _, rule := range src {
		rule.Names = slices.DeleteFunc(rule.Names, func(name string) bool { return names[name] })
		if len(rule.Names) == 0 {
			continue // libseccomp refuses equal-specificity rules whose actions clash
		}
		kept = append(kept, rule)
	}
	return kept
}
