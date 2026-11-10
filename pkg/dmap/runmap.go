package dmap

import (
	"os"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// RunMap maps one run's project config to the docker pkg options
type RunMap struct {
	cfg          *cfg.Config
	cacheVolumes []string
	tagByProject bool
}

// NewRunMap returns a new RunMap
func NewRunMap(cfg *cfg.Config, cacheVolumes []string, tagByProject bool) *RunMap {
	return &RunMap{cfg: cfg, cacheVolumes: cacheVolumes, tagByProject: tagByProject}
}

// RunOptions renders the run's full docker.RunOptions; nil proxyOptions runs plain
func (rm *RunMap) RunOptions(tag string, proxyOptions *docker.ProxyOptions) (docker.RunOptions, func() error, error) {
	serveVNC, noProxy := rm.cfg.ServeVNC(), proxyOptions == nil
	hostOptions, clean, err := rm.HostOptions(serveVNC, noProxy)
	if err != nil {
		return docker.RunOptions{}, clean, err
	}
	env, err := rm.Env(serveVNC)
	if err != nil {
		return docker.RunOptions{}, clean, err
	}

	name := devboxName(rm.cfg.ProjectDir(), serveVNC, noProxy)
	return docker.RunOptions{
		RunHostOptions: hostOptions,
		Name:           name,
		Tag:            tag + tagSuffix(rm.cfg.ProjectDir(), rm.tagByProject, serveVNC),
		Env:            env,
		Proxy:          proxyOptions,
		ServeVNC:       serveVNC,
		HoldersDir:     devboxHoldersDir(name),
		LogFile:        devboxLogPath(serveVNC, noProxy),
	}, clean, nil
}

// HostOptions renders the run's host options for docker.Run; vnc mounts the
// GUI app's install volume, which seeds from the desktop variant's image.
func (rm *RunMap) HostOptions(serveVNC, noProxy bool) (docker.RunHostOptions, func() error, error) {
	projectDir := rm.cfg.ProjectDir()
	binds, clean, err := rm.binds(serveVNC, noProxy)
	if err != nil {
		return docker.RunHostOptions{}, clean, err
	}

	return docker.RunHostOptions{
		ProjectDir:     projectDir,
		WorkspaceMount: workspaceMount(projectDir),
		Mounts:         binds,
		TmpfsPaths:     append(tmpfsMasks(projectDir, rm.cfg.TmpfsMasksPresent()), miseDataMount),
	}, clean, nil
}

// vncCfg returns the config's vnc section — an empty one when the config carries none
func (rm *RunMap) vncCfg() *cfg.VNC {
	if rm.cfg.VNC == nil {
		return &cfg.VNC{}
	}
	return rm.cfg.VNC
}

// Env renders the container's env in one map.
func (rm *RunMap) Env(serveVNC bool) (map[string]string, error) {
	cli := rm.cfg.CLI()
	pkgInfo, err := cli.PkgInfoJSON()
	if err != nil {
		return nil, err
	}
	env := mergeempty.Map(cli.Env, forwardEnv(rm.cfg.ForwardEnv))
	env = mergeempty.Map(env, map[string]string{pkginfo.EnvVar: pkgInfo})
	if serveVNC {
		body, err := rm.vncCfg().InfoJSON()
		if err != nil {
			return nil, err
		}
		// pkginfo.VNCConfigEnvVar's presence — even `{}` from a config without a vnc section — is the
		// container's cue to serve the desktop; a headless run sends none, its config's
		// vnc section left unused, unwarned.
		env[pkginfo.VNCConfigEnvVar] = body
	}
	return env, nil
}

// forwardEnv reads the config's vars off the host, skipping empty ones
func forwardEnv(names []string) map[string]string {
	env := make(map[string]string, len(names))
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			env[name] = v
		}
	}
	return env
}
