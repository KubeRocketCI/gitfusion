package bitbucket

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitbucketProviderDoesNotShareCookiesAcrossCalls(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /2.0/user/workspaces", func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Cookies())
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "first-caller", Path: "/"})
		writeBitbucketJSON(t, w, http.StatusOK, bitbucketWorkspacesResponse{})
	})

	svc := newBitbucketTestService(t, mux)

	for range 2 {
		_, err := svc.ListUserOrganizations(context.Background(), bitbucketTestSettings())
		require.NoError(t, err)
	}
}
