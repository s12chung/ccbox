package docker

import (
	"errors"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"

	"github.com/s12chung/ccbox/pkg/kit/dock"
)

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

// ContainerClean removes the projectDir's devbox containers (every variant), found
// by their label — running ones are stopped, like conflictErr's `docker rm -f` fix
func ContainerClean(ctxD *dock.CtxD, projectDir string) error {
	resp, err := ctxD.D.ContainerList(ctxD.Ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", ccboxProjectLabelKey+"="+projectDir)),
	})
	if err != nil {
		return err
	}

	var errs []error
	for _, c := range resp {
		if err := ctxD.D.ContainerRemove(ctxD.Ctx, c.ID, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
