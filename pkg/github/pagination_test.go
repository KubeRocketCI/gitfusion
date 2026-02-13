package github

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"

	"github.com/KubeRocketCI/gitfusion/pkg/xiter"
)

// helper to create a *github.Repository with a given name
func repo(name string) *github.Repository {
	return &github.Repository{Name: github.Ptr(name)}
}

func TestScanGitHubList(t *testing.T) {
	tests := []struct {
		name    string
		do      func(opt github.ListOptions) ([]*github.Repository, *github.Response, error)
		opts    ScanGitHubListOption
		want    []*github.Repository
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "single page",
			do: func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
				return []*github.Repository{repo("a"), repo("b")}, &github.Response{NextPage: 0}, nil
			},
			opts:    WithPerPage(10),
			want:    []*github.Repository{repo("a"), repo("b")},
			wantErr: assert.NoError,
		},
		{
			name: "multiple pages",
			do: func() func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
				pages := map[int][]*github.Repository{
					1: {repo("a")},
					2: {repo("b")},
				}
				return func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
					if opt.Page == 0 {
						opt.Page = 1
					}
					if repos, ok := pages[opt.Page]; ok {
						next := 0
						if opt.Page < 2 {
							next = opt.Page + 1
						}
						return repos, &github.Response{NextPage: next}, nil
					}
					return nil, &github.Response{NextPage: 0}, nil
				}
			}(),
			opts:    WithPerPage(10),
			want:    []*github.Repository{repo("a"), repo("b")},
			wantErr: assert.NoError,
		},
		{
			name: "error on first page",
			do: func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
				return nil, nil, errors.New("fail")
			},
			opts:    WithPerPage(10),
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "error on second page",
			do: func() func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
				calls := 0
				return func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
					calls++
					if calls == 1 {
						return []*github.Repository{repo("a")}, &github.Response{NextPage: 2}, nil
					}
					return nil, nil, errors.New("fail")
				}
			}(),
			opts:    WithPerPage(10),
			want:    []*github.Repository{repo("a")},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := xiter.CollectFromScan(ScanGitHubList(tt.do, tt.opts))
			assert.True(t, slices.EqualFunc(got, tt.want, func(a, b *github.Repository) bool {
				return a.GetName() == b.GetName()
			}))
			tt.wantErr(t, err)
		})
	}
}
