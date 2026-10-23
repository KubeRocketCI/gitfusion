package organizations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func requireUnsupported(t *testing.T, err error, gitProvider, operation string) {
	t.Helper()

	require.ErrorIs(t, err, gferrors.ErrUnsupportedOperation)
	assert.EqualError(t, err, "provider "+gitProvider+" does not support "+operation+": unsupported operation")
}

func TestMultiProviderOrganizationsService_ListUserOrganizations_UnknownProviderReturnsUnsupportedOperation(
	t *testing.T,
) {
	service := NewMultiProviderOrganizationsService()

	result, err := service.ListUserOrganizations(
		context.Background(),
		krci.GitServerSettings{GitProvider: "gerrit"},
	)

	requireUnsupported(t, err, "gerrit", "listUserOrganizations")
	assert.Nil(t, result)
}

func TestMultiProviderOrganizationsService_CheckListUserOrganizations(t *testing.T) {
	service := NewMultiProviderOrganizationsService()

	for _, provider := range []string{"github", "gitlab", "bitbucket"} {
		assert.NoError(t, service.CheckListUserOrganizations(provider), provider)
	}

	requireUnsupported(t, service.CheckListUserOrganizations("gerrit"), "gerrit", "listUserOrganizations")
}
