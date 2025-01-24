package types

import (
	"github.com/hetznercloud/hcloud-go/hcloud"
)

type TargetMetadata struct {
	ServerID     int
	ServerName   string
	ServerMemory float32
	Architecture string
	Location     string
	Created      string
}

// ToTargetMetadata converts and maps values from an *hcloud.Server to a WorkspaceMetadata.
func ToTargetMetadata(server *hcloud.Server) TargetMetadata {
	return TargetMetadata{
		ServerID:     server.ID,
		ServerName:   server.Name,
		ServerMemory: server.ServerType.Memory,
		Architecture: string(server.ServerType.Architecture),
		Created:      server.Created.String(),
	}
}
