package sqlite

import (
	"context"
	"testing"

	"memdoor/pkg/domain"
)

// A workspace with no slug is not a workspace.
func TestAdoptingNeedsASlug(t *testing.T) {
	if err := (&WorkspaceRepository{}).Create(context.Background(), &domain.Workspace{}); err == nil {
		t.Fatal("a workspace with no slug was created")
	}
}
