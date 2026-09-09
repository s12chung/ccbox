package docker

import (
	"errors"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/volume"

	"github.com/s12chung/ccbox/pkg/kit/dock"
)

// VolumeClean removes hostCwd's project volumes (cache and mask volumes), found by their labels
func VolumeClean(ctxD *dock.CtxD, hostCwd string) error {
	resp, err := ctxD.D.VolumeList(ctxD.Ctx, volume.ListOptions{Filters: projectVolumeFilter(hostCwd)})
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
