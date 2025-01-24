package types

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/daytonaio/daytona/pkg/models"
)

type TargetOptions struct {
	Location   string `json:"Location"`
	DiskImage  string `json:"Disk Image"`
	DiskSize   int    `json:"Disk Size"`
	ServerType string `json:"Server Type"`
	APIToken   string `json:"API Token"`
}

func GetTargetConfigManifest() *models.TargetConfigManifest {
	return &models.TargetConfigManifest{
		"Location": models.TargetConfigProperty{
			Type: models.TargetConfigPropertyTypeString,
			Description: "The locations where the resources will be created. Default is fsn1.\n" +
				"https://docs.hetzner.com/cloud/general/locations",
			DefaultValue: "fsn1",
			Suggestions:  locations,
		},
		"Disk Image": models.TargetConfigProperty{
			Type: models.TargetConfigPropertyTypeString,
			Description: "The Hetzner image to use for the VM. Default is ubuntu-24.04.\n" +
				"https://docs.hetzner.com/robot/dedicated-server/operating-systems/standard-images",
			DefaultValue: "ubuntu-24.04",
			Suggestions:  diskImages,
		},
		"Disk Size": models.TargetConfigProperty{
			Type:         models.TargetConfigPropertyTypeInt,
			Description:  "The size of the instance volume, in GB. Default is 20 GB.",
			DefaultValue: "20",
		},
		"Server Type": models.TargetConfigProperty{
			Type: models.TargetConfigPropertyTypeString,
			Description: "The Hetzner server type to use for the VM. Default is List cpx11.\n" +
				"https://docs.hetzner.com/cloud/servers/overview",
			DefaultValue: "cpx11",
			Suggestions:  serverTypes,
		},
		"API Token": models.TargetConfigProperty{
			Type:        models.TargetConfigPropertyTypeString,
			InputMasked: true,
			Description: "If empty, token will be fetched from the HETZNER_API_TOKEN environment variable.",
		},
	}
}

// ParseTargetOptions parses the target options from the JSON string.
func ParseTargetOptions(optionsJson string) (*TargetOptions, error) {
	var targetOptions TargetOptions
	err := json.Unmarshal([]byte(optionsJson), &targetOptions)
	if err != nil {
		return nil, err
	}

	if targetOptions.APIToken == "" {
		token, ok := os.LookupEnv("HETZNER_API_TOKEN")
		if ok {
			targetOptions.APIToken = token
		}
	}

	if targetOptions.APIToken == "" {
		return nil, fmt.Errorf("auth token not set in env/target options")
	}

	return &targetOptions, nil
}
