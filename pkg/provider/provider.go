package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/daytonaio/daytona-provider-hetzner/internal"
	logwriters "github.com/daytonaio/daytona-provider-hetzner/internal/log"
	hetznerutil "github.com/daytonaio/daytona-provider-hetzner/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-hetzner/pkg/types"
	"github.com/daytonaio/daytona/pkg/agent/ssh/config"
	"github.com/daytonaio/daytona/pkg/docker"
	"github.com/daytonaio/daytona/pkg/models"
	"github.com/daytonaio/daytona/pkg/ssh"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"tailscale.com/tsnet"

	"github.com/daytonaio/daytona/pkg/logs"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/provider/util"
)

type HetznerProvider struct {
	BasePath           *string
	DaytonaDownloadUrl *string
	DaytonaVersion     *string
	ServerUrl          *string
	NetworkKey         *string
	ApiUrl             *string
	ApiKey             *string
	ApiPort            *uint32
	ServerPort         *uint32
	WorkspaceLogsDir   *string
	TargetLogsDir      *string
	tsnetConn          *tsnet.Server
}

func (h *HetznerProvider) Initialize(req provider.InitializeProviderRequest) (*util.Empty, error) {
	h.BasePath = &req.BasePath
	h.DaytonaDownloadUrl = &req.DaytonaDownloadUrl
	h.DaytonaVersion = &req.DaytonaVersion
	h.ServerUrl = &req.ServerUrl
	h.NetworkKey = &req.NetworkKey
	h.ApiUrl = &req.ApiUrl
	h.ApiKey = req.ApiKey
	h.ApiPort = &req.ApiPort
	h.ServerPort = &req.ServerPort
	h.WorkspaceLogsDir = &req.WorkspaceLogsDir
	h.TargetLogsDir = &req.TargetLogsDir

	return new(util.Empty), nil
}

func (h *HetznerProvider) GetInfo() (models.ProviderInfo, error) {
	return models.ProviderInfo{
		Name:                 "hetzner-provider",
		Label:                hcloud.Ptr("Hetzner"),
		Version:              internal.Version,
		TargetConfigManifest: *types.GetTargetConfigManifest(),
	}, nil
}

func (h *HetznerProvider) GetPresetTargetConfigs() (*[]provider.TargetConfig, error) {
	return new([]provider.TargetConfig), nil
}

func (h *HetznerProvider) CreateTarget(targetReq *provider.TargetRequest) (*util.Empty, error) {
	if h.DaytonaDownloadUrl == nil {
		return nil, errors.New("DaytonaDownloadUrl not set. Did you forget to call Initialize")
	}
	logWriter, cleanupFunc := h.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target config options: " + err.Error() + "\n"))
		return nil, err
	}

	initScript := fmt.Sprintf(`curl -sfL -H "Authorization: Bearer %s" %s | bash`, targetReq.Target.ApiKey, *h.DaytonaDownloadUrl)
	err = hetznerutil.CreateTarget(targetReq.Target, targetOptions, initScript, logWriter)
	if err != nil {
		logWriter.Write([]byte("Failed to create target: " + err.Error() + "\n"))
		return nil, err
	}

	agentSpinner := logwriters.ShowSpinner(logWriter, "Waiting for the agent to start", "Agent started")
	err = h.waitForDial(targetReq.Target.Id, 10*time.Minute)
	close(agentSpinner)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := h.getDockerClient(targetReq.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	targetDir := getTargetDir(targetReq.Target.Id)
	sshClient, err := tailscale.NewSshClient(h.tsnetConn, &ssh.SessionConfig{
		Hostname: targetReq.Target.Id,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), client.CreateTarget(targetReq.Target, targetDir, logWriter, sshClient)
}

func (h *HetznerProvider) StartTarget(targetReq *provider.TargetRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target config options: " + err.Error() + "\n"))
		return nil, err
	}

	err = h.waitForDial(targetReq.Target.Id, 10*time.Minute)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), hetznerutil.StartTarget(targetReq.Target, targetOptions)
}

func (h *HetznerProvider) StopTarget(targetReq *provider.TargetRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target config options: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), hetznerutil.StopTarget(targetReq.Target, targetOptions)
}

func (h *HetznerProvider) DestroyTarget(targetReq *provider.TargetRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target config options: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), hetznerutil.DeleteTarget(targetReq.Target, targetOptions)
}

func (h *HetznerProvider) GetTargetProviderMetadata(targetReq *provider.TargetRequest) (string, error) {
	logWriter, cleanupFunc := h.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return "", err
	}

	server, err := hetznerutil.GetServer(targetReq.Target, targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get machine: " + err.Error() + "\n"))
		return "", err

	}

	metadata := types.ToTargetMetadata(server)
	jsonMetadata, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	return string(jsonMetadata), nil
}

func (h *HetznerProvider) CreateWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()
	logWriter.Write([]byte("\033[?25h\n"))

	dockerClient, err := h.getDockerClient(workspaceReq.Workspace.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(h.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.Target.Id,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.CreateWorkspace(&docker.CreateWorkspaceOptions{Workspace: workspaceReq.Workspace,
		WorkspaceDir:        getWorkspaceDir(workspaceReq),
		ContainerRegistries: workspaceReq.ContainerRegistries,
		BuilderImage:        workspaceReq.BuilderImage,
		LogWriter:           logWriter,
		Gpc:                 workspaceReq.GitProviderConfig,
		SshClient:           sshClient,
	})
}

func (h *HetznerProvider) StartWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	if h.DaytonaDownloadUrl == nil {
		return nil, errors.New("DaytonaDownloadUrl not set. Did you forget to call Initialize")
	}
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := h.getDockerClient(workspaceReq.Workspace.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(h.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.Target.Id,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.StartWorkspace(&docker.CreateWorkspaceOptions{
		Workspace:           workspaceReq.Workspace,
		WorkspaceDir:        getWorkspaceDir(workspaceReq),
		ContainerRegistries: workspaceReq.ContainerRegistries,
		BuilderImage:        workspaceReq.BuilderImage,
		LogWriter:           logWriter,
		Gpc:                 workspaceReq.GitProviderConfig,
		SshClient:           sshClient,
	}, *h.DaytonaDownloadUrl)
}

func (h *HetznerProvider) StopWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := h.getDockerClient(workspaceReq.Workspace.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), dockerClient.StopWorkspace(workspaceReq.Workspace, logWriter)
}

func (h *HetznerProvider) DestroyWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := h.getDockerClient(workspaceReq.Workspace.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(h.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.Target.Id,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.DestroyWorkspace(workspaceReq.Workspace, getWorkspaceDir(workspaceReq), sshClient)
}

func (h *HetznerProvider) GetWorkspaceProviderMetadata(workspaceReq *provider.WorkspaceRequest) (string, error) {
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := h.getDockerClient(workspaceReq.Workspace.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return "", err
	}

	return dockerClient.GetWorkspaceProviderMetadata(workspaceReq.Workspace)
}

func (h *HetznerProvider) getWorkspaceLogWriter(workspaceId, workspaceName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if h.WorkspaceLogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(logs.LoggerFactoryConfig{
			LogsDir:     *h.WorkspaceLogsDir,
			ApiUrl:      h.ApiUrl,
			ApiKey:      h.ApiKey,
			ApiBasePath: &logs.ApiBasePathWorkspace,
		})
		workspaceLogWriter, err := loggerFactory.CreateLogger(workspaceId, workspaceName, logs.LogSourceProvider)
		if err == nil {
			logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, workspaceLogWriter)
			cleanupFunc = func() { workspaceLogWriter.Close() }
		}
	}

	return logWriter, cleanupFunc
}

func (h *HetznerProvider) getTargetLogWriter(targetId, targetName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if h.TargetLogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(logs.LoggerFactoryConfig{
			LogsDir:     *h.TargetLogsDir,
			ApiUrl:      h.ApiUrl,
			ApiKey:      h.ApiKey,
			ApiBasePath: &logs.ApiBasePathTarget,
		})
		targetLogWriter, err := loggerFactory.CreateLogger(targetId, targetName, logs.LogSourceProvider)
		if err == nil {
			logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, targetLogWriter)
			cleanupFunc = func() { targetLogWriter.Close() }
		}
	}

	return logWriter, cleanupFunc
}

func (h *HetznerProvider) CheckRequirements() (*[]provider.RequirementStatus, error) {
	results := []provider.RequirementStatus{}
	return &results, nil
}

func getWorkspaceDir(workspaceReq *provider.WorkspaceRequest) string {
	return path.Join(
		getTargetDir(workspaceReq.Workspace.TargetId),
		workspaceReq.Workspace.Id,
		workspaceReq.Workspace.WorkspaceFolderName(),
	)
}

func getTargetDir(targetId string) string {
	return fmt.Sprintf("/home/daytona/%s", targetId)
}
