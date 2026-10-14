package organizations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/gitfusion/internal/cache"
	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestMultiProviderOrganizationsService_ListUserOrganizations_UnknownProviderReturnsUnsupportedOperation(
	t *testing.T,
) {
	// Built without NewMultiProviderOrganizationsService: its cache warm-up goroutine makes coverage
	// depend on scheduling.
	service := &MultiProviderOrganizationsService{
		providers: map[string]OrganizationsProvider{},
		cache:     cache.NewOrganizationCache(),
	}

	result, err := service.ListUserOrganizations(
		context.Background(),
		krci.GitServerSettings{GitProvider: "gerrit"},
	)

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported provider gerrit")
}
