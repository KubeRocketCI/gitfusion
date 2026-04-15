package bitbucket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestBitbucketServiceListUserOrganizationsSuccess(t *testing.T) {
	responseBody := bitbucketWorkspacesResponse{
		Size:    2,
		Page:    1,
		Pagelen: 10,
		Values: []bitbucketWorkspaceAccess{
			{Workspace: bitbucketWorkspace{UUID: "{aaaaaaaa-1111-2222-3333-bbbbbbbbbbbb}", Slug: "my-workspace"}},
			{Workspace: bitbucketWorkspace{UUID: "{cccccccc-4444-5555-6666-dddddddddddd}", Slug: "another-workspace"}},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/2.0/user/workspaces", r.URL.Path)
		assert.NotEmpty(t, r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		body, _ := json.Marshal(responseBody)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListUserOrganizations(context.Background(), krci.GitServerSettings{
		Token: testBitbucketToken(),
	})

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "{aaaaaaaa-1111-2222-3333-bbbbbbbbbbbb}", result[0].Id)
	assert.Equal(t, "my-workspace", result[0].Name)
	assert.Equal(t, "{cccccccc-4444-5555-6666-dddddddddddd}", result[1].Id)
	assert.Equal(t, "another-workspace", result[1].Name)
}

func TestBitbucketServiceListUserOrganizationsEmpty(t *testing.T) {
	responseBody := bitbucketWorkspacesResponse{
		Size:    0,
		Page:    1,
		Pagelen: 10,
		Values:  []bitbucketWorkspaceAccess{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		body, _ := json.Marshal(responseBody)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListUserOrganizations(context.Background(), krci.GitServerSettings{
		Token: testBitbucketToken(),
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestBitbucketServiceListUserOrganizationsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListUserOrganizations(context.Background(), krci.GitServerSettings{
		Token: testBitbucketToken(),
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gferrors.ErrUnauthorized))
}

func TestBitbucketServiceListUserOrganizationsForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListUserOrganizations(context.Background(), krci.GitServerSettings{
		Token: testBitbucketToken(),
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.True(t, errors.Is(err, gferrors.ErrUnauthorized))
}

func TestBitbucketServiceListUserOrganizationsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	svc := &BitbucketService{
		httpClient: resty.New().SetTransport(&redirectTransport{
			target:  server.URL,
			wrapped: http.DefaultTransport,
		}),
	}

	result, err := svc.ListUserOrganizations(context.Background(), krci.GitServerSettings{
		Token: testBitbucketToken(),
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestBitbucketServiceListUserOrganizationsInvalidToken(t *testing.T) {
	svc := &BitbucketService{
		httpClient: resty.New(),
	}

	result, err := svc.ListUserOrganizations(context.Background(), krci.GitServerSettings{
		Token: "not-valid-base64!!!",
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode token")
}
