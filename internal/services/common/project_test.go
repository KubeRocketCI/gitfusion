package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
)

func TestSplitProject(t *testing.T) {
	tests := []struct {
		name       string
		project    string
		wantFirst  string
		wantSecond string
		wantErr    bool
	}{
		{name: "valid owner/repo", project: "owner/repo", wantFirst: "owner", wantSecond: "repo"},
		{name: "valid with nested path", project: "owner/repo/extra", wantFirst: "owner", wantSecond: "repo/extra"},
		{name: "empty string", project: "", wantErr: true},
		{name: "no slash", project: "ownerrepo", wantErr: true},
		{name: "empty owner", project: "/repo", wantErr: true},
		{name: "empty repo", project: "owner/", wantErr: true},
		{name: "just slash", project: "/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, second, err := SplitProject(tt.project)

			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, gferrors.ErrBadRequest), "error should wrap ErrBadRequest")
				assert.Empty(t, first)
				assert.Empty(t, second)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantFirst, first)
				assert.Equal(t, tt.wantSecond, second)
			}
		})
	}
}
