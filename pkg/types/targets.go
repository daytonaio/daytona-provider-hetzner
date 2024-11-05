package types

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/daytonaio/daytona/pkg/provider"
)

type TargetOptions struct {
	Location   string `json:"Location"`
	DiskImage  string `json:"Disk Image"`
	DiskSize   int    `json:"Disk Size"`
	ServerType string `json:"Server Type"`
	APIToken   string `json:"API Token"`
}

func GetTargetManifest() *provider.ProviderTargetManifest {
	return &provider.ProviderTargetManifest{
		"Location": provider.ProviderTargetProperty{
			Type: provider.ProviderTargetPropertyTypeString,
			Description: "The locations where the resources will be created. Default is fsn1.\n" +
				"https://docs.hetzner.com/cloud/general/locations",
			DefaultValue: "fsn1",
			Suggestions:  locations,
		},
		"Disk Image": provider.ProviderTargetProperty{
			Type: provider.ProviderTargetPropertyTypeString,
			Description: "The Hetzner image to use for the VM. Default is ubuntu-24.04.\n" +
				"https://docs.hetzner.com/robot/dedicated-server/operating-systems/standard-images",
			DefaultValue: "ubuntu-24.04",
			Suggestions:  diskImages,
		},
		"Disk Size": provider.ProviderTargetProperty{
			Type:         provider.ProviderTargetPropertyTypeInt,
			Description:  "The size of the instance volume, in GB. Default is 20 GB.",
			DefaultValue: "20",
		},
		"Server Type": provider.ProviderTargetProperty{
			Type: provider.ProviderTargetPropertyTypeString,
			Description: "The Hetzner server type to use for the VM. Default is List cpx11.\n" +
				"https://docs.hetzner.com/cloud/servers/overview",
			DefaultValue: "cpx11",
			Suggestions:  serverTypes,
		},
		"API Token": provider.ProviderTargetProperty{
			Type:        provider.ProviderTargetPropertyTypeString,
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
