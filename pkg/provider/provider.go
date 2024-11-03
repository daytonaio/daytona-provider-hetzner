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
	"github.com/daytonaio/daytona-provider-hetzner/pkg/types"
	"github.com/daytonaio/daytona/pkg/agent/ssh/config"
	"github.com/daytonaio/daytona/pkg/docker"
	"github.com/daytonaio/daytona/pkg/ssh"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"tailscale.com/tsnet"

	"github.com/daytonaio/daytona/pkg/logs"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/provider/util"
	"github.com/daytonaio/daytona/pkg/workspace"
	"github.com/daytonaio/daytona/pkg/workspace/project"
)

type HetznerProvider struct {
	BasePath           *string
	DaytonaDownloadUrl *string
	DaytonaVersion     *string
	ServerUrl          *string
	NetworkKey         *string
	ApiUrl             *string
	ApiPort            *uint32
	ServerPort         *uint32
	LogsDir            *string
	tsnetConn          *tsnet.Server
}

func (h *HetznerProvider) Initialize(req provider.InitializeProviderRequest) (*util.Empty, error) {
	h.BasePath = &req.BasePath
	h.DaytonaDownloadUrl = &req.DaytonaDownloadUrl
	h.DaytonaVersion = &req.DaytonaVersion
	h.ServerUrl = &req.ServerUrl
	h.NetworkKey = &req.NetworkKey
	h.ApiUrl = &req.ApiUrl
	h.ApiPort = &req.ApiPort
	h.ServerPort = &req.ServerPort
	h.LogsDir = &req.LogsDir

	return new(util.Empty), nil
}

func (h *HetznerProvider) GetInfo() (provider.ProviderInfo, error) {
	return provider.ProviderInfo{
		Name:    "hetzner-provider",
		Version: internal.Version,
	}, nil
}

func (h *HetznerProvider) GetTargetManifest() (*provider.ProviderTargetManifest, error) {
	return types.GetTargetManifest(), nil
}

func (h *HetznerProvider) GetPresetTargets() (*[]provider.ProviderTarget, error) {
	return new([]provider.ProviderTarget), nil
}

func (h *HetznerProvider) CreateWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	if h.DaytonaDownloadUrl == nil {
		return nil, errors.New("DaytonaDownloadUrl not set. Did you forget to call Initialize")
	}
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	_ = targetOptions
	initScript := fmt.Sprintf(`curl -sfL -H "Authorization: Bearer %s" %s | bash`, workspaceReq.Workspace.ApiKey, *h.DaytonaDownloadUrl)
	_ = initScript
	// TODO: call hetzner api to create workspace

	agentSpinner := logwriters.ShowSpinner(logWriter, "Waiting for the agent to start", "Agent started")
	err = h.waitForDial(workspaceReq.Workspace.Id, 10*time.Minute)
	close(agentSpinner)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := h.getDockerClient(workspaceReq.Workspace.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	workspaceDir := getWorkspaceDir(workspaceReq.Workspace.Id)
	sshClient, err := tailscale.NewSshClient(h.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.Id,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), client.CreateWorkspace(workspaceReq.Workspace, workspaceDir, logWriter, sshClient)
}

func (h *HetznerProvider) StartWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	err = h.waitForDial(workspaceReq.Workspace.Id, 10*time.Minute)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}

	_ = targetOptions
	// TODO: call hetzner api to start workspace
	return new(util.Empty), nil
}

func (h *HetznerProvider) StopWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	_ = targetOptions
	// TODO: call hetzner api to stop workspace
	return new(util.Empty), nil
}

func (h *HetznerProvider) DestroyWorkspace(workspaceReq *provider.WorkspaceRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	// TODO: call hetzner api to destroy workspace
	_ = targetOptions
	return new(util.Empty), nil
}

func (h *HetznerProvider) GetWorkspaceInfo(workspaceReq *provider.WorkspaceRequest) (*workspace.WorkspaceInfo, error) {
	workspaceInfo, err := h.getWorkspaceInfo(workspaceReq)
	if err != nil {
		return nil, err
	}

	var projectInfos []*project.ProjectInfo
	for _, project := range workspaceReq.Workspace.Projects {
		projectInfo, err := h.GetProjectInfo(&provider.ProjectRequest{
			TargetOptions: workspaceReq.TargetOptions,
			Project:       project,
		})
		if err != nil {
			return nil, err
		}
		projectInfos = append(projectInfos, projectInfo)
	}
	workspaceInfo.Projects = projectInfos

	return workspaceInfo, nil
}

func (h *HetznerProvider) CreateProject(projectReq *provider.ProjectRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()
	logWriter.Write([]byte("\033[?25h\n"))

	dockerClient, err := h.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(h.tsnetConn, &ssh.SessionConfig{
		Hostname: projectReq.Project.WorkspaceId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.CreateProject(&docker.CreateProjectOptions{
		Project:    projectReq.Project,
		ProjectDir: getProjectDir(projectReq),
		Cr:         projectReq.ContainerRegistry,
		LogWriter:  logWriter,
		Gpc:        projectReq.GitProviderConfig,
		SshClient:  sshClient,
	})
}

func (h *HetznerProvider) StartProject(projectReq *provider.ProjectRequest) (*util.Empty, error) {
	if h.DaytonaDownloadUrl == nil {
		return nil, errors.New("DaytonaDownloadUrl not set. Did you forget to call Initialize")
	}
	logWriter, cleanupFunc := h.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := h.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(h.tsnetConn, &ssh.SessionConfig{
		Hostname: projectReq.Project.WorkspaceId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.StartProject(&docker.CreateProjectOptions{
		Project:    projectReq.Project,
		ProjectDir: getProjectDir(projectReq),
		Cr:         projectReq.ContainerRegistry,
		LogWriter:  logWriter,
		Gpc:        projectReq.GitProviderConfig,
		SshClient:  sshClient,
	}, *h.DaytonaDownloadUrl)
}

func (h *HetznerProvider) StopProject(projectReq *provider.ProjectRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := h.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	return new(util.Empty), dockerClient.StopProject(projectReq.Project, logWriter)
}

func (h *HetznerProvider) DestroyProject(projectReq *provider.ProjectRequest) (*util.Empty, error) {
	logWriter, cleanupFunc := h.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := h.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	sshClient, err := tailscale.NewSshClient(h.tsnetConn, &ssh.SessionConfig{
		Hostname: projectReq.Project.WorkspaceId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(util.Empty), err
	}
	defer sshClient.Close()

	return new(util.Empty), dockerClient.DestroyProject(projectReq.Project, getProjectDir(projectReq), sshClient)
}

func (h *HetznerProvider) GetProjectInfo(projectReq *provider.ProjectRequest) (*project.ProjectInfo, error) {
	logWriter, cleanupFunc := h.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := h.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	return dockerClient.GetProjectInfo(projectReq.Project)
}

func (h *HetznerProvider) getWorkspaceInfo(workspaceReq *provider.WorkspaceRequest) (*workspace.WorkspaceInfo, error) {
	logWriter, cleanupFunc := h.getWorkspaceLogWriter(workspaceReq.Workspace.Id)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to parse target options: " + err.Error() + "\n"))
		return nil, err
	}

	// TODO: call hetzner api to get workspace info
	_ = targetOptions
	jsonMetadata, err := json.Marshal(nil)
	if err != nil {
		return nil, err
	}

	return &workspace.WorkspaceInfo{
		Name:             workspaceReq.Workspace.Name,
		ProviderMetadata: string(jsonMetadata),
	}, nil
}

func (h *HetznerProvider) getWorkspaceLogWriter(workspaceId string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if h.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(h.LogsDir, nil)
		wsLogWriter := loggerFactory.CreateWorkspaceLogger(workspaceId, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, wsLogWriter)
		cleanupFunc = func() { wsLogWriter.Close() }
	}

	return logWriter, cleanupFunc
}

func (h *HetznerProvider) getProjectLogWriter(workspaceId string, projectName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if h.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(h.LogsDir, nil)
		projectLogWriter := loggerFactory.CreateProjectLogger(workspaceId, projectName, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, projectLogWriter)
		cleanupFunc = func() { projectLogWriter.Close() }
	}

	return logWriter, cleanupFunc
}

func getWorkspaceDir(workspaceId string) string {
	return fmt.Sprintf("/home/daytona/%s", workspaceId)
}

func getProjectDir(projectReq *provider.ProjectRequest) string {
	return path.Join(
		getWorkspaceDir(projectReq.Project.WorkspaceId),
		fmt.Sprintf("%s-%s", projectReq.Project.WorkspaceId, projectReq.Project.Name),
	)
}
