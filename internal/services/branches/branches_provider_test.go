package branches

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestMultiProviderBranchesService_ListBranches_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	service := NewMultiProviderBranchesService()

	result, err := service.ListBranches(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{GitProvider: "gerrit"},
		models.ListOptions{},
	)

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported provider gerrit")
}
