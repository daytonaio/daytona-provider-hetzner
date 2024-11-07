package provider

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/daytonaio/daytona/pkg/agent/ssh/config"
	"github.com/daytonaio/daytona/pkg/docker"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"tailscale.com/tsnet"
)

func (h *HetznerProvider) getTsnetConn() (*tsnet.Server, error) {
	if h.tsnetConn == nil {
		tsnetConn, err := tailscale.GetConnection(&tailscale.TsnetConnConfig{
			AuthKey:    *h.NetworkKey,
			ControlURL: *h.ServerUrl,
			Dir:        filepath.Join(*h.BasePath, "tsnet", uuid.NewString()),
			Logf:       func(format string, args ...any) {},
			Hostname:   fmt.Sprintf("hetzner-provider-%s", uuid.NewString()),
		})
		if err != nil {
			return nil, err
		}
		h.tsnetConn = tsnetConn
	}

	return h.tsnetConn, nil
}

func (h *HetznerProvider) waitForDial(workspaceId string, dialTimeout time.Duration) error {
	tsnetConn, err := h.getTsnetConn()
	if err != nil {
		return err
	}

	dialStartTime := time.Now()
	for {
		if time.Since(dialStartTime) > dialTimeout {
			return fmt.Errorf("timeout: dialing timed out after %f minutes", dialTimeout.Minutes())
		}

		dialConn, err := tsnetConn.Dial(context.Background(), "tcp", fmt.Sprintf("%s:%d", workspaceId, config.SSH_PORT))
		if err == nil {
			dialConn.Close()
			return nil
		}

		time.Sleep(time.Second)
	}
}

func (h *HetznerProvider) getDockerClient(workspaceId string) (docker.IDockerClient, error) {
	tsnetConn, err := h.getTsnetConn()
	if err != nil {
		return nil, err
	}

	remoteHost := fmt.Sprintf("tcp://%s:2375", workspaceId)
	cli, err := client.NewClientWithOpts(client.WithDialContext(tsnetConn.Dial), client.WithHost(remoteHost), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return docker.NewDockerClient(docker.DockerClientConfig{
		ApiClient: cli,
	}), nil
}
