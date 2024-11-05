package provider

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	hetznerutil "github.com/daytonaio/daytona-provider-hetzner/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-hetzner/pkg/types"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/workspace"
)

var (
	apiToken = os.Getenv("HETZNER_API_TOKEN")

	hetznerProvider = &HetznerProvider{}
	targetOptions   = &types.TargetOptions{
		Location:   "fsn1",
		DiskImage:  "ubuntu-22.04",
		DiskSize:   20,
		ServerType: "cpx11",
		APIToken:   apiToken,
	}

	workspaceReq *provider.WorkspaceRequest
)

func TestCreateWorkspace(t *testing.T) {
	_, err := hetznerProvider.CreateWorkspace(workspaceReq)
	if err != nil {
		t.Errorf("Error creating workspace: %s", err)
	}

	_, err = hetznerutil.GetServer(workspaceReq.Workspace, targetOptions)
	if err != nil {
		t.Fatalf("Error getting server: %s", err)
	}
}

func TestWorkspaceInfo(t *testing.T) {
	workspaceInfo, err := hetznerProvider.GetWorkspaceInfo(workspaceReq)
	if err != nil {
		t.Fatalf("Error getting workspace info: %s", err)
	}

	var workspaceMetadata types.WorkspaceMetadata
	err = json.Unmarshal([]byte(workspaceInfo.ProviderMetadata), &workspaceMetadata)
	if err != nil {
		t.Fatalf("Error unmarshalling workspace metadata: %s", err)
	}

	server, err := hetznerutil.GetServer(workspaceReq.Workspace, targetOptions)
	if err != nil {
		t.Fatalf("Error getting server: %s", err)
	}

	expectedMetadata := types.ToWorkspaceMetadata(server)

	if expectedMetadata.ServerID != expectedMetadata.ServerID {
		t.Fatalf("Expected vm id %d, got %d",
			expectedMetadata.ServerID,
			expectedMetadata.ServerID,
		)
	}

	if expectedMetadata.ServerName != expectedMetadata.ServerName {
		t.Fatalf("Expected server name %s, got %s",
			expectedMetadata.ServerName,
			expectedMetadata.ServerName,
		)
	}

	if expectedMetadata.ServerMemory != workspaceMetadata.ServerMemory {
		t.Fatalf("Expected server memory %f, got %f",
			expectedMetadata.ServerMemory,
			workspaceMetadata.ServerMemory,
		)
	}

	if expectedMetadata.Architecture != workspaceMetadata.Architecture {
		t.Fatalf("Expected server architecture %s, got %s",
			expectedMetadata.Architecture,
			workspaceMetadata.Architecture,
		)
	}

	if expectedMetadata.Location != workspaceMetadata.Location {
		t.Fatalf("Expected server location %s, got %s",
			expectedMetadata.Location,
			workspaceMetadata.Location,
		)
	}

	if expectedMetadata.Created != workspaceMetadata.Created {
		t.Fatalf("Expected server created at %s, got %s",
			expectedMetadata.Created,
			workspaceMetadata.Created,
		)
	}
}

func TestDestroyWorkspace(t *testing.T) {
	_, err := hetznerProvider.DestroyWorkspace(workspaceReq)
	if err != nil {
		t.Fatalf("Error destroying workspace: %s", err)
	}
	time.Sleep(3 * time.Second)

	_, err = hetznerutil.GetServer(workspaceReq.Workspace, targetOptions)
	if err == nil {
		t.Fatalf("Error destroyed workspace still exists")
	}
}

func init() {
	_, err := hetznerProvider.Initialize(provider.InitializeProviderRequest{
		BasePath:           "/tmp/workspaces",
		DaytonaDownloadUrl: "https://download.daytona.io/daytona/install.sh",
		DaytonaVersion:     "latest",
		ServerUrl:          "",
		ApiUrl:             "",
		LogsDir:            "/tmp/logs",
	})
	if err != nil {
		panic(err)
	}

	opts, err := json.Marshal(targetOptions)
	if err != nil {
		panic(err)
	}

	workspaceReq = &provider.WorkspaceRequest{
		TargetOptions: string(opts),
		Workspace: &workspace.Workspace{
			Id:   "123",
			Name: "workspace",
		},
	}
}
