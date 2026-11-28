package dock

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"

	"github.com/s12chung/ccbox/pkg/util/slicex"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// SortedEnv renders env as the container env — KEY=VALUE entries in sorted-key order,
// for a deterministic spec
func SortedEnv(env map[string]string) []string {
	keys := slices.Sorted(maps.Keys(env))
	return slicex.Map(keys, func(k string) string { return k + "=" + env[k] })
}

// EnvEntry reads one var's value off the container env — "" without it
func EnvEntry(env []string, name string) string {
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, name+"="); ok {
			return value
		}
	}
	return ""
}

// CopyFileFrom copies the container's file at path into memory. CopyFrom tars single
// files under their basename, so the stream holds the one entry to read out.
func CopyFileFrom(ctxD *CtxD, id, path string) ([]byte, error) {
	stream, _, err := ctxD.D.CopyFromContainer(ctxD.Ctx, id, path)
	if err != nil {
		return nil, err
	}
	defer log.Defer("close copy stream", stream.Close)
	tr := tar.NewReader(stream)
	if _, err := tr.Next(); err != nil {
		return nil, err
	}
	return io.ReadAll(tr)
}

// CopyDirFrom copies the container's directory at dir into memory, its files keyed
// by their paths relative to dir. CopyFrom tars the tree under dir's basename.
func CopyDirFrom(ctxD *CtxD, id, dir string) (map[string][]byte, error) {
	stream, _, err := ctxD.D.CopyFromContainer(ctxD.Ctx, id, dir)
	if err != nil {
		return nil, err
	}
	defer log.Defer("close copy stream", stream.Close)
	return filesFromTar(stream, filepath.Base(dir)+"/")
}

// filesFromTar reads the regular files out of a CopyFrom stream, dropping the entries' name prefix
func filesFromTar(stream io.Reader, prefix string) (map[string][]byte, error) {
	files := map[string][]byte{}
	tr := tar.NewReader(stream)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[strings.TrimPrefix(hdr.Name, prefix)] = body
	}
}

// OwnedVolume is a volume EnsureOwnedVolumes ensures: created (with labels) if absent.
type OwnedVolume struct {
	Name   string
	Labels map[string]string
}

// EnsureOwnedVolumes creates each missing volume (with labels) and chowns the fresh
// ones' roots to uid via ONE throwaway root container running image. Existing volumes
// are never re-chowned
func EnsureOwnedVolumes(ctxD *CtxD, image, uid string, vols []OwnedVolume) error {
	var fresh []OwnedVolume
	for _, vol := range vols {
		isFresh, err := ensureVolume(ctxD, vol)
		if err != nil {
			return err
		}
		if isFresh {
			fresh = append(fresh, vol)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	return runChownContainer(ctxD, image, uid, fresh)
}

func ensureVolume(ctxD *CtxD, vol OwnedVolume) (bool, error) {
	switch _, err := ctxD.D.VolumeInspect(ctxD.Ctx, vol.Name); {
	case err == nil:
		return false, nil
	case !errdefs.IsNotFound(err):
		return false, err
	}
	if _, err := ctxD.D.VolumeCreate(ctxD.Ctx, volume.CreateOptions{Name: vol.Name, Labels: vol.Labels}); err != nil {
		return false, err
	}
	return true, nil
}

func runChownContainer(ctxD *CtxD, image, uid string, fresh []OwnedVolume) error {
	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{
			Image:      image,
			User:       "0:0",
			Entrypoint: append([]string{"chown", uid + ":" + uid}, slicex.Map(fresh, chownMnt)...),
		},
		&container.HostConfig{
			NetworkMode: "none",
			Binds:       slicex.Map(fresh, chownBind),
		},
		nil, nil, "")
	if err != nil {
		return err
	}
	// the chown container's remove must complete even on a cancelled ctx
	clean := ctxD.BackgroundedCtx()
	defer log.Defer("remove chown container", func() error {
		return clean.D.ContainerRemove(clean.Ctx, resp.ID, container.RemoveOptions{Force: true})
	})

	var logs bytes.Buffer
	if err := RunOnce(ctxD, resp.ID, &logs); err != nil {
		return fmt.Errorf("chown volumes %v: %w\n%s",
			slicex.Map(fresh, func(vol OwnedVolume) string { return vol.Name }), err, logs.String())
	}
	return nil
}

// chownMnt is vol's mount point in the chown container: /mnt/<volume-name>. Volume names
// are DNS-label-safe ([a-zA-Z0-9_.-]), so they're valid path segments.
func chownMnt(vol OwnedVolume) string { return "/mnt/" + vol.Name }

func chownBind(vol OwnedVolume) string { return vol.Name + ":" + chownMnt(vol) }
