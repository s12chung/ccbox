package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func TestRunConfig(t *testing.T) {
	tests := []struct {
		name string
		opts RunOptions
		want *container.Config
	}{
		{
			name: "Base",
			opts: RunOptions{
				RunHostOptions: RunHostOptions{
					ProjectDir:     "/host/proj",
					WorkspaceMount: "/home/ccbox/proj",
					Mounts:         []Mount{NewBind("/host/proj", "/home/ccbox/proj")},
				},
				Tag: "ccbox:latest",
				Env: map[string]string{"GH_TOKEN": "gh", "GOFLAGS": "-mod=mod"},
				Cmd: []string{"claude"},
			},
			want: &container.Config{
				Image:        "ccbox:latest",
				Cmd:          []string{"claude"},
				WorkingDir:   "/home/ccbox/proj",
				Tty:          true,
				OpenStdin:    true,
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				Env: []string{
					"GH_TOKEN=gh",
					"GOFLAGS=-mod=mod",
					"http_proxy=http://ccbox-egress:8888",
					"https_proxy=http://ccbox-egress:8888",
					"no_proxy=localhost,127.0.0.1,::1",
					"NO_PROXY=localhost,127.0.0.1,::1",
				},
			},
		},
		{
			name: "NoProxySkipsWallEnv",
			opts: RunOptions{
				RunHostOptions: RunHostOptions{WorkspaceMount: "/home/ccbox/proj"},
				Tag:            "ccbox:latest",
				Env:            map[string]string{"GH_TOKEN": "gh"},
				NoProxy:        true,
			},
			want: &container.Config{
				Image:        "ccbox:latest",
				WorkingDir:   "/home/ccbox/proj",
				Tty:          true,
				OpenStdin:    true,
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				Env:          []string{"GH_TOKEN=gh"},
			},
		},
		{
			name: "NilCmdUsesImageDefault",
			opts: RunOptions{
				RunHostOptions: RunHostOptions{WorkspaceMount: "/home/ccbox/proj"},
				Tag:            "ccbox:latest",
			},
			want: &container.Config{
				Image:        "ccbox:latest",
				WorkingDir:   "/home/ccbox/proj",
				Tty:          true,
				OpenStdin:    true,
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				Env: []string{
					"http_proxy=http://ccbox-egress:8888",
					"https_proxy=http://ccbox-egress:8888",
					"no_proxy=localhost,127.0.0.1,::1",
					"NO_PROXY=localhost,127.0.0.1,::1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, runConfig(tt.opts))
		})
	}
}
