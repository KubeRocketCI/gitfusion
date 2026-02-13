package github

import (
	"slices"
	"testing"

	"github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"

	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/pkg/xiter"
)

func Test_filterProjectsByName(t *testing.T) {
	type args struct {
		it  xiter.Scan[*github.Repository]
		opt models.ListOptions
	}

	tests := []struct {
		name    string
		args    args
		want    []*github.Repository
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "returns all when Name is nil",
			args: args{
				it: xiter.Scan[*github.Repository](func(yield func(*github.Repository, error) bool) {
					if !yield(&github.Repository{Name: github.Ptr("repo1")}, nil) {
						return
					}
					yield(&github.Repository{Name: github.Ptr("repo2")}, nil)
				}),
				opt: models.ListOptions{Name: nil},
			},
			want: []*github.Repository{
				{Name: github.Ptr("repo1")},
				{Name: github.Ptr("repo2")},
			},
			wantErr: assert.NoError,
		},
		{
			name: "filters by substring (case-insensitive)",
			args: args{
				it: xiter.Scan[*github.Repository](func(yield func(*github.Repository, error) bool) {
					if !yield(&github.Repository{Name: github.Ptr("with a")}, nil) {
						return
					}
					if !yield(&github.Repository{Name: github.Ptr("with b")}, nil) {
						return
					}
					yield(&github.Repository{Name: github.Ptr("with A")}, nil)
				}),
				opt: models.ListOptions{Name: github.Ptr("a")},
			},
			want: []*github.Repository{
				{Name: github.Ptr("with a")},
				{Name: github.Ptr("with A")},
			},
			wantErr: assert.NoError,
		},
		{
			name: "returns empty when no match",
			args: args{
				it: xiter.Scan[*github.Repository](func(yield func(*github.Repository, error) bool) {
					if !yield(&github.Repository{Name: github.Ptr("foo")}, nil) {
						return
					}
					yield(&github.Repository{Name: github.Ptr("bar")}, nil)
				}),
				opt: models.ListOptions{Name: github.Ptr("baz")},
			},
			want:    []*github.Repository{},
			wantErr: assert.NoError,
		},
		{
			name: "handles error from iterator",
			args: args{
				it: xiter.Scan[*github.Repository](func(yield func(*github.Repository, error) bool) {
					yield(nil, assert.AnError)
				}),
				opt: models.ListOptions{Name: nil},
			},
			want:    []*github.Repository{},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := xiter.CollectFromScan(filterRepositoriesByName(tt.args.it, tt.args.opt))
			assert.True(t, slices.EqualFunc(got, tt.want, func(a, b *github.Repository) bool {
				return a.GetName() == b.GetName()
			}))
			tt.wantErr(t, err)
		})
	}
}
