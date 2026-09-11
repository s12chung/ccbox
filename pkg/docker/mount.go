package docker

import (
	"errors"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
)

// ensureMounts creates each mount's named volume, so the run's mounts resolve.
func ensureMounts(ctxD *dock.CtxD, imageTag, projectDir string, mounts []Mount) error {
	for _, mount := range mounts {
		if err := mount.ensure(ctxD, imageTag, projectDir); err != nil {
			return err
		}
	}
	return nil
}

// mountSpecs renders each mount, in slice order, for the HostConfig.
func mountSpecs(mounts []Mount) []string {
	specs := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		specs = append(specs, mount.spec())
	}
	return specs
}

// Mount is a mount into the devbox container
type Mount interface {
	spec() string                                              // renders the mount's spec
	ensure(ctxD *dock.CtxD, imageTag, projectDir string) error // ensures the resources are available to mount
}

// Bind bind-mounts a host path at a container path.
type Bind struct {
	host      string
	container string
	readOnly  bool
}

// NewBind bind-mounts host at container.
func NewBind(host, container string) Bind {
	return Bind{host: host, container: container}
}

// ReadOnly mounts the bind read-only.
func (b Bind) ReadOnly() Bind {
	b.readOnly = true
	return b
}

func (b Bind) spec() string {
	if b.readOnly {
		return b.host + ":" + b.container + ":ro"
	}
	return b.host + ":" + b.container
}

func (Bind) ensure(*dock.CtxD, string, string) error { return nil }

// Volume mounts a named volume at a container path. Global shares the volume across
// every project; otherwise it is project-scoped (labeled with projectDir). Owned chowns the
// volume to the container user when ensured, so masked dirs hold content the run owns.
type Volume struct {
	name      string
	container string
	global    bool
	owned     bool
}

// NewVolume mounts the named volume at container.
func NewVolume(name, container string) Volume {
	return Volume{name: name, container: container}
}

// Global shares the volume across every project.
func (v Volume) Global() Volume {
	v.global = true
	return v
}

// Owned chowns the volume to the container user when ensured.
func (v Volume) Owned() Volume {
	v.owned = true
	return v
}

func (v Volume) spec() string { return v.name + ":" + v.container }

func (v Volume) ensure(ctxD *dock.CtxD, imageTag, projectDir string) error {
	if v.owned {
		return dock.EnsureOwnedVolume(ctxD, imageTag, v.name, containerUID, v.labels(projectDir))
	}
	_, err := ctxD.D.VolumeCreate(ctxD.Ctx, volume.CreateOptions{Name: v.name, Labels: v.labels(projectDir)})
	return err
}

const (
	ccboxLabelKey        = "ccbox"
	ccboxProjectLabelKey = "ccbox.project"
	ccboxGlobalLabelKey  = "ccbox.global"
	trueVolValue         = "true"
)

// volumeLabels marks a volume as ccbox's own.
func volumeLabels() map[string]string { return map[string]string{ccboxLabelKey: trueVolValue} }

// labels stamps v's volume with the ccbox mark plus its scope.
func (v Volume) labels(projectDir string) map[string]string {
	scopedLabels := map[string]string{ccboxProjectLabelKey: projectDir}
	if v.global {
		scopedLabels = map[string]string{ccboxGlobalLabelKey: trueVolValue}
	}
	return mergeempty.Map(volumeLabels(), scopedLabels)
}

// VolumeClean removes the projectDir's volumes (cache and mask volumes), found by their labels
func VolumeClean(ctxD *dock.CtxD, projectDir string) error {
	resp, err := ctxD.D.VolumeList(ctxD.Ctx, volume.ListOptions{Filters: filters.NewArgs(
		filters.Arg("label", ccboxLabelKey+"="+trueVolValue),
		filters.Arg("label", ccboxProjectLabelKey+"="+projectDir),
	)})
	if err != nil {
		return err
	}

	var errs []error
	for _, vol := range resp.Volumes {
		if err := ctxD.D.VolumeRemove(ctxD.Ctx, vol.Name, false); err != nil && !errdefs.IsNotFound(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// containerUID is the unprivileged in-container user (Dockerfile: useradd --uid 1000).
const containerUID = "1000"

// tmpfsOpts makes a masked dir's tmpfs writable+executable by the container user, so masked
// build outputs (e.g. dist/) can be written and run — Docker's default is root-owned noexec.
const tmpfsOpts = "uid=" + containerUID + ",gid=" + containerUID + ",exec"

// tmpfsMap maps each masked container path to its tmpfs options.
func tmpfsMap(paths []string) map[string]string {
	tmpfs := make(map[string]string, len(paths))
	for _, p := range paths {
		tmpfs[p] = tmpfsOpts
	}
	return tmpfs
}
