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

func requireUnsupported(t *testing.T, err error, gitProvider, operation string) {
	t.Helper()

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.EqualError(t, err, "provider "+gitProvider+" does not support "+operation+": unsupported operation")
}

func TestMultiProviderBranchesService_ListBranches_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	service := NewMultiProviderBranchesService()

	result, err := service.ListBranches(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{GitProvider: "gerrit"},
		models.ListOptions{},
	)

	requireUnsupported(t, err, "gerrit", "listBranches")
	assert.Nil(t, result)
}

func TestMultiProviderBranchesService_CheckListBranches(t *testing.T) {
	service := NewMultiProviderBranchesService()

	for _, provider := range []string{"github", "gitlab", "bitbucket"} {
		assert.NoError(t, service.CheckListBranches(provider), provider)
	}

	requireUnsupported(t, service.CheckListBranches("gerrit"), "gerrit", "listBranches")
}
