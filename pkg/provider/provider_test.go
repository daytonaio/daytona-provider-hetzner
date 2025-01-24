package provider

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	hetznerutil "github.com/daytonaio/daytona-provider-hetzner/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-hetzner/pkg/types"
	"github.com/daytonaio/daytona/pkg/models"
	"github.com/daytonaio/daytona/pkg/provider"
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

	targetReq *provider.TargetRequest
)

func TestCreateTarget(t *testing.T) {
	_, err := hetznerProvider.CreateTarget(targetReq)
	if err != nil {
		t.Errorf("Error creating target: %s", err)
	}

	_, err = hetznerutil.GetServer(targetReq.Target, targetOptions)
	if err != nil {
		t.Fatalf("Error getting server: %s", err)
	}
}

func TestGetTargetProviderMetadata(t *testing.T) {
	targetProviderMetadata, err := hetznerProvider.GetTargetProviderMetadata(targetReq)
	if err != nil {
		t.Fatalf("Error getting target info: %s", err)
	}

	var targetMetadata types.TargetMetadata
	err = json.Unmarshal([]byte(targetProviderMetadata), &targetMetadata)
	if err != nil {
		t.Fatalf("Error unmarshalling target metadata: %s", err)
	}

	server, err := hetznerutil.GetServer(targetReq.Target, targetOptions)
	if err != nil {
		t.Fatalf("Error getting server: %s", err)
	}

	expectedMetadata := types.ToTargetMetadata(server)

	if expectedMetadata.ServerID != targetMetadata.ServerID {
		t.Fatalf("Expected server id %d, got %d",
			expectedMetadata.ServerID,
			targetMetadata.ServerID,
		)
	}

	if expectedMetadata.ServerName != targetMetadata.ServerName {
		t.Fatalf("Expected server name %s, got %s",
			expectedMetadata.ServerName,
			targetMetadata.ServerName,
		)
	}

	if expectedMetadata.ServerMemory != targetMetadata.ServerMemory {
		t.Fatalf("Expected server memory %f, got %f",
			expectedMetadata.ServerMemory,
			targetMetadata.ServerMemory,
		)
	}

	if expectedMetadata.Architecture != targetMetadata.Architecture {
		t.Fatalf("Expected server architecture %s, got %s",
			expectedMetadata.Architecture,
			targetMetadata.Architecture,
		)
	}

	if expectedMetadata.Location != targetMetadata.Location {
		t.Fatalf("Expected server location %s, got %s",
			expectedMetadata.Location,
			targetMetadata.Location,
		)
	}

	if expectedMetadata.Created != targetMetadata.Created {
		t.Fatalf("Expected server created at %s, got %s",
			expectedMetadata.Created,
			targetMetadata.Created,
		)
	}
}

func TestDestroyTarget(t *testing.T) {
	_, err := hetznerProvider.DestroyTarget(targetReq)
	if err != nil {
		t.Fatalf("Error destroying target: %s", err)
	}
	time.Sleep(3 * time.Second)

	_, err = hetznerutil.GetServer(targetReq.Target, targetOptions)
	if err == nil {
		t.Fatalf("Error destroyed target still exists")
	}
}

func init() {
	_, err := hetznerProvider.Initialize(provider.InitializeProviderRequest{
		BasePath:           "/tmp/targets",
		DaytonaDownloadUrl: "https://download.daytona.io/daytona/install.sh",
		DaytonaVersion:     "latest",
		ServerUrl:          "",
		ApiUrl:             "",
		WorkspaceLogsDir:   "/tmp/workspace/logs",
		TargetLogsDir:      "/tmp/target/logs",
	})
	if err != nil {
		panic(err)
	}

	opts, err := json.Marshal(targetOptions)
	if err != nil {
		panic(err)
	}

	targetReq = &provider.TargetRequest{
		Target: &models.Target{
			Id:   "123",
			Name: "target",
			TargetConfig: models.TargetConfig{
				Name: "test",
				ProviderInfo: models.ProviderInfo{
					Name:    "aws-provider",
					Version: "test",
				},
				Options: string(opts),
				Deleted: false,
			},
		},
	}
}
