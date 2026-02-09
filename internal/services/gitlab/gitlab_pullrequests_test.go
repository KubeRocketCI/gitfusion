package gitlab

import (
	"testing"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/stretchr/testify/assert"
)

func Test_mapPullRequestStateToGitLab(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{
			name:  "open maps to opened",
			state: "open",
			want:  "opened",
		},
		{
			name:  "closed maps to closed",
			state: "closed",
			want:  "closed",
		},
		{
			name:  "merged maps to merged",
			state: "merged",
			want:  "merged",
		},
		{
			name:  "all maps to all",
			state: "all",
			want:  "all",
		},
		{
			name:  "unknown state defaults to opened",
			state: "unknown",
			want:  "opened",
		},
		{
			name:  "empty state defaults to opened",
			state: "",
			want:  "opened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPullRequestStateToGitLab(tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_normalizeGitLabMRState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  models.PullRequestState
	}{
		{
			name:  "opened maps to open",
			state: "opened",
			want:  models.PullRequestStateOpen,
		},
		{
			name:  "merged maps to merged",
			state: "merged",
			want:  models.PullRequestStateMerged,
		},
		{
			name:  "closed maps to closed",
			state: "closed",
			want:  models.PullRequestStateClosed,
		},
		{
			name:  "locked defaults to open",
			state: "locked",
			want:  models.PullRequestStateOpen,
		},
		{
			name:  "unknown defaults to open",
			state: "unknown",
			want:  models.PullRequestStateOpen,
		},
		{
			name:  "empty defaults to open",
			state: "",
			want:  models.PullRequestStateOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitLabMRState(tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}
