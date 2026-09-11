package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMountSpecs(t *testing.T) {
	assert.Equal(t, []string{
		"/host/a:/a",
		"vol-b:/b:ro",
	}, mountSpecs([]Mount{
		NewBind("/host/a", "/a"),
		NewBind("vol-b", "/b").ReadOnly(),
	}))
	assert.Empty(t, mountSpecs(nil))
}

func TestBind(t *testing.T) {
	assert.Equal(t, "/host/dir:/home/ccbox/dir", NewBind("/host/dir", "/home/ccbox/dir").spec())
	assert.Equal(t, "/host/dir:/home/ccbox/dir:ro", NewBind("/host/dir", "/home/ccbox/dir").ReadOnly().spec())
}

func TestVolume(t *testing.T) {
	assert.Equal(t, "ccbox-vol:/home/ccbox/dir", NewVolume("ccbox-vol", "/home/ccbox/dir").spec())
}

func TestVolume_Labels(t *testing.T) {
	projectDir := "/work/myproj"

	assert.Equal(t, map[string]string{
		"ccbox":         "true",
		"ccbox.project": projectDir,
	}, NewVolume("ccbox-vol", "/dir").labels(projectDir))

	// global: spared from per-project cleanup
	assert.Equal(t, map[string]string{
		"ccbox":        "true",
		"ccbox.global": "true",
	}, NewVolume("ccbox-vol", "/dir").Global().labels(projectDir))
}

func TestTmpfsMap(t *testing.T) {
	got := tmpfsMap([]string{"/home/ccbox/proj/.idea", "/home/ccbox/proj/dist"})

	assert.Equal(t, map[string]string{
		"/home/ccbox/proj/.idea": tmpfsOpts,
		"/home/ccbox/proj/dist":  tmpfsOpts,
	}, got)
	assert.Equal(t, "uid=1000,gid=1000,exec", tmpfsOpts)
}
