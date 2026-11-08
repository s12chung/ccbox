package identity

import (
	"slices"
	"testing"

	"github.com/moby/profiles/seccomp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/seccomputil"
)

func TestDropFilterNames_IsSubsetOfBootProfile(t *testing.T) {
	profile, err := seccomputil.BootProfile()
	require.NoError(t, err)
	budget := map[string]bool{}
	for _, name := range RunCapNames() {
		budget["CAP_"+name] = true
	}
	inBudget := func(capName string) bool { return budget[capName] }

	dropFilter := dropFilterNames
	for _, rule := range profile.Syscalls {
		if rule.Includes == nil || !slices.ContainsFunc(rule.Includes.Caps, inBudget) {
			continue
		}
		for _, name := range rule.Names {
			// umount is left to the kernel's cap check — see dropFilterNames
			if name == "umount" {
				continue
			}
			assert.Contains(t, dropFilter, name, "the drop filter must deny the budget-gated %s", name)
		}
	}
}

func TestDropFilterNames_IsSubsetOfDefaultProfile(t *testing.T) {
	profile, err := seccomputil.DefaultProfile()
	require.NoError(t, err)

	for _, name := range dropFilterNames {
		gated := slices.ContainsFunc(profile.Syscalls, func(rule *seccomp.Syscall) bool {
			return rule.Includes != nil && slices.Contains(rule.Names, name)
		})
		assert.True(t, gated, "%s is not cap-gated in the default profile — denying it would break a capless workload", name)
	}
}
