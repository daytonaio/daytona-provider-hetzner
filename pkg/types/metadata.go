package types

import (
	"github.com/hetznercloud/hcloud-go/hcloud"
)

type WorkspaceMetadata struct {
	ServerID     int
	ServerName   string
	ServerMemory float32
	Architecture string
	Location     string
	Created      string
}

// ToWorkspaceMetadata converts and maps values from an *hcloud.Server to a WorkspaceMetadata.
func ToWorkspaceMetadata(server *hcloud.Server) WorkspaceMetadata {
	return WorkspaceMetadata{
		ServerID:     server.ID,
		ServerName:   server.Name,
		ServerMemory: server.ServerType.Memory,
		Architecture: string(server.ServerType.Architecture),
		Created:      server.Created.String(),
	}
}
