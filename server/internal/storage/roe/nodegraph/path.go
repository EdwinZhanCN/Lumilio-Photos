// Package nodegraph owns path projection from the durable repository node
// hierarchy. Projected paths are capabilities for immediate filesystem access,
// never catalog identity.
package nodegraph

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"server/internal/db/repo"
)

type Reader interface {
	GetRepositoryNode(context.Context, repo.GetRepositoryNodeParams) (repo.RepositoryNode, error)
}

var (
	ErrNodeNotActive = errors.New("repository node is not active")
	ErrInvalidRoot   = errors.New("repository node graph has an invalid root")
)

func ProjectPath(ctx context.Context, reader Reader, repositoryID, nodeID uuid.UUID) (string, error) {
	components := make([]string, 0, 16)
	seen := make(map[uuid.UUID]struct{})
	for nodeID != uuid.Nil {
		if _, exists := seen[nodeID]; exists {
			return "", errors.New("repository node graph contains a cycle")
		}
		seen[nodeID] = struct{}{}
		node, err := reader.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{
			RepositoryID: repositoryID, NodeID: nodeID,
		})
		if err != nil {
			return "", err
		}
		if node.Lifecycle != "active" {
			return "", fmt.Errorf("%w: %s", ErrNodeNotActive, node.NodeID)
		}
		if !node.ParentNodeID.Valid {
			if node.Kind != "directory" || node.Name != "" {
				return "", ErrInvalidRoot
			}
			break
		}
		components = append(components, node.Name)
		nodeID = node.ParentNodeID.UUID
		if len(components) > 4096 {
			return "", errors.New("repository node graph exceeds maximum depth")
		}
	}
	for left, right := 0, len(components)-1; left < right; left, right = left+1, right-1 {
		components[left], components[right] = components[right], components[left]
	}
	return strings.Join(components, "/"), nil
}
