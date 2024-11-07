package types

import (
	"reflect"
	"testing"
)

func TestGetTargetManifest(t *testing.T) {
	targetManifest := GetTargetManifest()
	if targetManifest == nil {
		t.Fatalf("Expected target manifest but got nil")
	}

	fields := [5]string{"Location", "Disk Image", "Disk Size", "Server Type", "API Token"}
	for _, field := range fields {
		if _, ok := (*targetManifest)[field]; !ok {
			t.Errorf("Expected field %s in target manifest but it was not found", field)
		}
	}
}

func TestParseTargetOptions(t *testing.T) {
	tests := []struct {
		name        string
		optionsJson string
		envVars     map[string]string
		want        *TargetOptions
		wantErr     bool
	}{
		{
			name: "Valid JSON with all fields",
			optionsJson: `{
				"Location":"fsn1",
				"Disk Image":"ubuntu-22.04",
				"Disk Size":20,
				"Server Type":"cpx11",
				"API Token":"token"
			}`,
			want: &TargetOptions{
				Location:   "fsn1",
				DiskImage:  "ubuntu-22.04",
				DiskSize:   20,
				ServerType: "cpx11",
				APIToken:   "token",
			},
			wantErr: false,
		},
		{
			name: "Valid JSON with missing fields, using env vars",
			optionsJson: `{
				"Location":"fsn1",
				"Disk Image":"ubuntu-22.04",
				"Disk Size":20,
				"Server Type":"cpx11"
			}`,
			envVars: map[string]string{
				"HETZNER_API_TOKEN": "token",
			},
			want: &TargetOptions{
				Location:   "fsn1",
				DiskImage:  "ubuntu-22.04",
				DiskSize:   20,
				ServerType: "cpx11",
				APIToken:   "token",
			},
			wantErr: false,
		},
		{
			name:        "Invalid JSON",
			optionsJson: `{"Location": "hel1", "DiskImage": "debian-11"`,
			wantErr:     true,
		},
		{
			name: "Missing all required fields in both JSON and env vars",
			optionsJson: `{
				"Disk Image":"ubuntu-22.04"
			}`,
			wantErr: true,
		},
		{
			name:        "Empty JSON",
			optionsJson: `{}`,
			envVars: map[string]string{
				"HETZNER_API_TOKEN": "token",
			},
			want: &TargetOptions{
				APIToken: "token",
			},
			wantErr: false,
		},
		{
			name: "Partial JSON with some valid env vars",
			optionsJson: `{
				"Disk Size":30,
				"Server Type":"cpx22"
			}`,
			envVars: map[string]string{
				"HETZNER_API_TOKEN": "token",
			},
			want: &TargetOptions{
				DiskSize:   30,
				ServerType: "cpx22",
				APIToken:   "token",
			},
			wantErr: false,
		},
		{
			name: "JSON with additional non-required fields",
			optionsJson: `{
				"Location":"fsn1",
				"Disk Image":"ubuntu-22.04",
				"Disk Size":20,
				"Server Type":"cpx11",
				"API Token":"token",
				"ExtraField": "extra-value"
			}`,
			want: &TargetOptions{
				Location:   "fsn1",
				DiskImage:  "ubuntu-22.04",
				DiskSize:   20,
				ServerType: "cpx11",
				APIToken:   "token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			got, err := ParseTargetOptions(tt.optionsJson)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTargetOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTargetOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}
