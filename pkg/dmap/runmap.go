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
func (rm *RunMap) RunOptions(proxyOptions *docker.ProxyOptions) (docker.RunOptions, func() error, error) {
	isProxied := proxyOptions != nil
	hostOptions, clean, err := rm.HostOptions(isProxied)
	if err != nil {
		return docker.RunOptions{}, clean, err
	}
	env, err := rm.Env()
	if err != nil {
		return docker.RunOptions{}, clean, err
	}

	name := devboxName(rm.cfg.ProjectDir(), rm.cfg.IsVNC(), isProxied)
	return docker.RunOptions{
		RunHostOptions:  hostOptions,
		Name:            name,
		Tag:             docker.ImageTag + tagSuffix(rm.cfg.ProjectDir(), rm.tagByProject, rm.cfg.IsVNC()),
		Env:             env,
		Proxy:           proxyOptions,
		ServeVNC:        rm.cfg.IsVNC(),
		PortBindingsTCP: rm.cfg.PortBindingsTCP(),
		HostGatewayTCP:  rm.cfg.HostGatewayTCP(),
		HoldersDir:      docker.HoldersDir(name),
		LogFile:         devboxLogPath(rm.cfg.ProjectDir(), rm.cfg.IsVNC(), isProxied),
	}, clean, nil
}

// HostOptions renders the run's host options for docker.Run; vnc mounts the
// GUI app's install volume, which seeds from the desktop variant's image.
func (rm *RunMap) HostOptions(isProxied bool) (docker.RunHostOptions, func() error, error) {
	projectDir := rm.cfg.ProjectDir()
	binds, clean, err := rm.binds(isProxied)
	if err != nil {
		return docker.RunHostOptions{}, clean, err
	}

	return docker.RunHostOptions{
		ProjectDir:     projectDir,
		WorkspaceMount: workspaceMount(projectDir),
		Mounts:         binds,
		TmpfsPaths:     append(tmpfsMasks(projectDir, rm.cfg.TmpfsMasksDirs()), miseDataMount),
	}, clean, nil
}

// Env renders the container's env in one map.
func (rm *RunMap) Env() (map[string]string, error) {
	cli := rm.cfg.CLI()
	pkgInfo, err := cli.PkgInfoJSON()
	if err != nil {
		return nil, err
	}
	runConfig, err := rm.cfg.RunConfigJSON()
	if err != nil {
		return nil, err
	}
	env := mergeempty.Map(cli.Setup.Env, forwardEnv(rm.cfg.ForwardEnv))
	env = mergeempty.Map(env, map[string]string{pkginfo.EnvVar: pkgInfo, pkginfo.RunConfigEnvVar: runConfig})
	if rm.cfg.IsVNC() {
		body, err := rm.cfg.VNC.InfoJSON()
		if err != nil {
			return nil, err
		}
		// the container serves the desktop whenever pkginfo.VNCConfigEnvVar is set —
		// even with the `{}` a config without a vnc section sends
		env[pkginfo.VNCConfigEnvVar] = body
	}
	if rm.cfg.HasGUIApp() {
		env = mergeempty.Map(env, rm.cfg.VNC.GUIApp().Setup.Env)
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
