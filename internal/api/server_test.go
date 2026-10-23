package api

import (
	"context"
	"testing"

	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildServerReturnsWarmUp(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, codebaseApi.AddToScheme(scheme))

	server, warmUp := buildServer(fake.NewClientBuilder().WithScheme(scheme).Build(), "krci")

	assert.NotNil(t, server)
	require.NotNil(t, warmUp)
	warmUp(context.Background())
}
