package repositories

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func requireUnsupportedForGerrit(t *testing.T, err error, operation string) {
	t.Helper()

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.EqualError(t, err, "provider gerrit does not support "+operation+": unsupported operation")
}

func TestMultiProviderRepositoryService_GetRepository_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	service := NewMultiProviderRepositoryService()

	result, err := service.GetRepository(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{GitProvider: "gerrit"},
	)

	requireUnsupportedForGerrit(t, err, "getRepository")
	assert.Nil(t, result)
}

func TestMultiProviderRepositoryService_ListRepositories_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	service := NewMultiProviderRepositoryService()

	result, err := service.ListRepositories(
		context.Background(),
		"owner",
		krci.GitServerSettings{GitProvider: "gerrit"},
		models.ListOptions{},
	)

	requireUnsupportedForGerrit(t, err, "listRepositories")
	assert.Nil(t, result)
}

func TestMultiProviderRepositoryService_Checks(t *testing.T) {
	service := NewMultiProviderRepositoryService()

	for _, provider := range []string{"github", "gitlab", "bitbucket"} {
		assert.NoError(t, service.CheckGetRepository(provider), provider)
		assert.NoError(t, service.CheckListRepositories(provider), provider)
	}

	requireUnsupportedForGerrit(t, service.CheckGetRepository("gerrit"), "getRepository")
	requireUnsupportedForGerrit(t, service.CheckListRepositories("gerrit"), "listRepositories")
}
