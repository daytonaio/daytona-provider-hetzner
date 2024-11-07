package util

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	logwriters "github.com/daytonaio/daytona-provider-hetzner/internal/log"
	"github.com/daytonaio/daytona-provider-hetzner/pkg/types"
	"github.com/daytonaio/daytona/pkg/workspace"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

func CreateWorkspace(workspace *workspace.Workspace, opts *types.TargetOptions, initScript string, logWriter io.Writer) error {
	envVars := workspace.EnvVars
	envVars["DAYTONA_AGENT_LOG_FILE_PATH"] = "/home/daytona/.daytona-agent.log"

	customData := `#!/bin/bash
useradd -m -d /home/daytona daytona

curl -fsSL https://get.docker.com | bash

# Modify Docker daemon configuration
cat > /etc/docker/daemon.json <<EOF
{
  "hosts": ["unix:///var/run/docker.sock", "tcp://127.0.0.1:2375"]
}
EOF

# Create a systemd drop-in file to modify the Docker service
mkdir -p /etc/systemd/system/docker.service.d
cat > /etc/systemd/system/docker.service.d/override.conf <<EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd
EOF

systemctl daemon-reload
systemctl restart docker
systemctl start docker

usermod -aG docker daytona

if grep -q sudo /etc/group; then
	usermod -aG sudo,docker daytona
elif grep -q wheel /etc/group; then
	usermod -aG wheel,docker daytona
fi

echo "daytona ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/91-daytona

`

	for k, v := range envVars {
		customData += fmt.Sprintf("export %s=%s\n", k, v)
	}
	customData += initScript
	customData += `
echo '[Unit]
Description=Daytona Agent Service
After=network.target

[Service]
User=daytona
ExecStart=/usr/local/bin/daytona agent --host
Restart=always
`

	for k, v := range envVars {
		customData += fmt.Sprintf("Environment='%s=%s'\n", k, v)
	}

	customData += `
[Install]
WantedBy=multi-user.target' > /etc/systemd/system/daytona-agent.service
systemctl daemon-reload
systemctl enable daytona-agent.service
systemctl start daytona-agent.service
`
	return createServer(workspace.Id, customData, opts, logWriter)
}

func StartWorkspace(workspace *workspace.Workspace, opts *types.TargetOptions) error {
	client := hcloud.NewClient(hcloud.WithToken(opts.APIToken))

	server, err := GetServer(workspace, opts)
	if err != nil {
		return err
	}

	if server.Status == hcloud.ServerStatusRunning {
		return nil
	}

	action, _, err := client.Server.Poweron(context.Background(), server)
	if err != nil {
		return err
	}

	return action.Error()
}

func StopWorkspace(workspace *workspace.Workspace, opts *types.TargetOptions) error {
	client := hcloud.NewClient(hcloud.WithToken(opts.APIToken))

	server, err := GetServer(workspace, opts)
	if err != nil {
		return err
	}

	if server.Status == hcloud.ServerStatusStopping {
		return nil
	}

	action, _, err := client.Server.Poweroff(context.Background(), server)
	if err != nil {
		return err
	}

	return action.Error()
}

func DeleteWorkspace(workspace *workspace.Workspace, opts *types.TargetOptions) error {
	client := hcloud.NewClient(hcloud.WithToken(opts.APIToken))

	server, err := GetServer(workspace, opts)
	if err != nil {
		return err
	}

	result, _, err := client.Server.DeleteWithResult(context.Background(), server)
	if err != nil {
		return err
	}

	err = waitForAction(client, result.Action)
	if err != nil {
		return err
	}

	for _, volume := range server.Volumes {
		_, err = client.Volume.Delete(context.Background(), volume)
		if err != nil {
			return err
		}
	}

	return result.Action.Error()
}

// createServer creates a new Hetzner server and volume.
func createServer(workspaceId, customData string, opts *types.TargetOptions, logWriter io.Writer) error {
	client := hcloud.NewClient(hcloud.WithToken(opts.APIToken))

	location, _, err := client.Location.GetByName(context.Background(), opts.Location)
	if err != nil {
		return err
	}

	spinner := logwriters.ShowSpinner(logWriter, "Creating Hetzner volume", "Hetzner volume created")
	volume, _, err := client.Volume.Create(context.Background(), hcloud.VolumeCreateOpts{
		Location: location,
		Name:     fmt.Sprintf("daytona-%s", workspaceId),
		Size:     opts.DiskSize,
		Format:   hcloud.Ptr("ext4"),
	})
	if err != nil {
		return err
	}
	close(spinner)

	spinner = logwriters.ShowSpinner(logWriter, "Creating Hetzner server", "Hetzner server created")
	defer close(spinner)

	serverType, _, err := client.ServerType.GetByName(context.Background(), opts.ServerType)
	if err != nil {
		return err
	}

	vmArch := hcloud.ArchitectureX86
	if strings.HasPrefix(opts.ServerType, "cax") {
		// Server types with cax prefix are Arm64 architecture
		vmArch = hcloud.ArchitectureARM
	}

	image, _, err := client.Image.GetByNameAndArchitecture(context.Background(), opts.DiskImage, vmArch)
	if err != nil {
		return err
	}

	_, _, err = client.Server.Create(context.Background(), hcloud.ServerCreateOpts{
		Name:             fmt.Sprintf("daytona-%s", workspaceId),
		ServerType:       serverType,
		Image:            image,
		Location:         location,
		UserData:         customData,
		StartAfterCreate: hcloud.Ptr(true),
		Automount:        hcloud.Ptr(true),
		Volumes:          []*hcloud.Volume{volume.Volume},
	})
	return err
}

// GetServer returns the virtual machine instance for the given workspace.
func GetServer(workspace *workspace.Workspace, opts *types.TargetOptions) (*hcloud.Server, error) {
	client := hcloud.NewClient(hcloud.WithToken(opts.APIToken))
	server, _, s := client.Server.GetByName(context.Background(), fmt.Sprintf("daytona-%s", workspace.Id))
	if s != nil {
		return nil, s
	}
	return server, nil
}

// waitForAction waits for the action to complete.
func waitForAction(client *hcloud.Client, action *hcloud.Action) error {
	for {
		action, _, err := client.Action.GetByID(context.Background(), action.ID)
		if err != nil {
			return err
		}

		if action.Status == hcloud.ActionStatusSuccess {
			return nil
		}

		if action.Status == hcloud.ActionStatusError {
			return action.Error()
		}

		time.Sleep(2 * time.Second)
	}
}
