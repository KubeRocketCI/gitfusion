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

func TestMultiProviderRepositoryService_GetRepository_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	service := NewMultiProviderRepositoryService()

	result, err := service.GetRepository(
		context.Background(),
		"owner",
		"repo",
		krci.GitServerSettings{GitProvider: "gerrit"},
	)

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported provider gerrit")
}

func TestMultiProviderRepositoryService_ListRepositories_UnknownProviderReturnsUnsupportedOperation(t *testing.T) {
	service := NewMultiProviderRepositoryService()

	result, err := service.ListRepositories(
		context.Background(),
		"owner",
		krci.GitServerSettings{GitProvider: "gerrit"},
		models.ListOptions{},
	)

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported provider gerrit")
}
