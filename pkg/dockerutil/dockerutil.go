// Package dockerutil holds generic Docker Engine operations, independent of the devbox
// lifecycle — things that can only be done from inside a container the daemon spawns.
package dockerutil

import (
	"context"
	"fmt"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"

	"github.com/s12chung/ccbox/pkg/log"
)

// CtxD pairs a D Engine client with the context its calls run under.
type CtxD struct {
	Ctx context.Context
	D   *client.Client
}

func NewCtxD(ctx context.Context) (*CtxD, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &CtxD{Ctx: ctx, D: cli}, nil
}

// EnsureOwnedVolume creates the named volume (with labels) if absent and chowns it to uid
func EnsureOwnedVolume(ctxD *CtxD, image, volumeName, uid string, labels map[string]string) error {
	switch _, err := ctxD.D.VolumeInspect(ctxD.Ctx, volumeName); {
	case err == nil:
		return nil
	case !errdefs.IsNotFound(err):
		return err
	}
	if _, err := ctxD.D.VolumeCreate(ctxD.Ctx, volume.CreateOptions{Name: volumeName, Labels: labels}); err != nil {
		return err
	}
	return ChownVolume(ctxD, image, volumeName, uid+":"+uid)
}

// ChownVolume chown's volume to owner ("uid:gid") via a throwaway root container  running image.
func ChownVolume(ctxD *CtxD, image, volumeName, owner string) error {
	mountPoint := "/mnt"

	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{
			Image:      image,
			User:       "0:0",
			Entrypoint: []string{"chown", owner, mountPoint},
		},
		&container.HostConfig{
			NetworkMode: "none",
			Binds:       []string{volumeName + ":" + mountPoint},
		},
		nil, nil, "")
	if err != nil {
		return err
	}
	defer log.Defer("remove chown container", func() error {
		return ctxD.D.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	})

	// Register the wait before start so a fast exit isn't missed.
	statusCh, errCh := ctxD.D.ContainerWait(ctxD.Ctx, resp.ID, container.WaitConditionNextExit)
	if err := ctxD.D.ContainerStart(ctxD.Ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}
	select {
	case err := <-errCh:
		return err
	case st := <-statusCh:
		if st.StatusCode != 0 {
			return fmt.Errorf("chown volume %q: container exited %d", volumeName, st.StatusCode)
		}
		return nil
	}
}
