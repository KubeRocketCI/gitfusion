package bitbucket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
)

func TestBitbucketProviderDoesNotShareCookiesAcrossCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Cookies())
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "first-caller", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(bitbucketWorkspacesResponse{}))
	}))
	t.Cleanup(server.Close)

	svc := NewBitbucketProvider()
	svc.httpClient.SetTransport(&redirectTransport{target: server.URL, wrapped: http.DefaultTransport})

	for range 2 {
		_, err := svc.ListUserOrganizations(context.Background(), krci.GitServerSettings{Token: testBitbucketToken()})
		require.NoError(t, err)
	}
}
