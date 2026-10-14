package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/httplog/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouterRequestLogging(t *testing.T) {
	const apiPath = "/api/v1/repositories"

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantLogged []string
	}{
		{name: "GET heartbeat is answered and not logged", method: http.MethodGet, path: healthPath,
			wantStatus: http.StatusOK, wantLogged: nil},
		{name: "HEAD heartbeat is answered and not logged", method: http.MethodHead, path: healthPath,
			wantStatus: http.StatusOK, wantLogged: nil},
		{name: "API request is logged", method: http.MethodGet, path: apiPath,
			wantStatus: http.StatusNoContent, wantLogged: []string{apiPath}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := httplog.NewLogger("gitfusion-api-test", httplog.Options{JSON: true, Concise: true, Writer: &buf})

			r := newRouter(logger)
			r.Get(apiPath, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantLogged, loggedPaths(t, &buf))
		})
	}
}

func loggedPaths(t *testing.T, buf *bytes.Buffer) []string {
	t.Helper()

	var paths []string

	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		var line struct {
			HTTPRequest struct {
				Path string `json:"path"`
			} `json:"httpRequest"`
		}

		require.NoError(t, json.Unmarshal(scanner.Bytes(), &line))

		paths = append(paths, line.HTTPRequest.Path)
	}

	require.NoError(t, scanner.Err())

	return paths
}
