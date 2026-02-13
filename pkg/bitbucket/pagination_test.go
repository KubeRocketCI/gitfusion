package bitbucket

import (
	"errors"
	"slices"
	"testing"

	bitbucketcl "github.com/ktrysmt/go-bitbucket"
	"github.com/stretchr/testify/assert"

	"github.com/KubeRocketCI/gitfusion/pkg/xiter"
)

// helper to create a bitbucketcl.RepositoryBranch with a given name
func branch(name string) bitbucketcl.RepositoryBranch {
	return bitbucketcl.RepositoryBranch{Name: name}
}

// helper to create a *bitbucketcl.RepositoryBranches response
func branchesResponse(branches []bitbucketcl.RepositoryBranch, next string, page int) *bitbucketcl.RepositoryBranches {
	return &bitbucketcl.RepositoryBranches{
		Branches: branches,
		Next:     next,
		Page:     page,
	}
}

func TestScanBitbucketBranches(t *testing.T) {
	tests := []struct {
		name      string
		fetchPage func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error)
		rbo       *bitbucketcl.RepositoryBranchOptions
		want      []*bitbucketcl.RepositoryBranch
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name: "single page",
			fetchPage: func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error) {
				return branchesResponse([]bitbucketcl.RepositoryBranch{
					branch("main"),
					branch("develop"),
				}, "", 1), nil
			},
			rbo: &bitbucketcl.RepositoryBranchOptions{},
			want: []*bitbucketcl.RepositoryBranch{
				{Name: "main"},
				{Name: "develop"},
			},
			wantErr: assert.NoError,
		},
		{
			name: "multiple pages",
			fetchPage: func() func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error) {
				pages := map[int]*bitbucketcl.RepositoryBranches{
					1: branchesResponse([]bitbucketcl.RepositoryBranch{branch("main")}, "next", 1),
					2: branchesResponse([]bitbucketcl.RepositoryBranch{branch("develop")}, "", 2),
				}
				return func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error) {
					if rbo.PageNum == 0 {
						rbo.PageNum = 1
					}
					if resp, ok := pages[rbo.PageNum]; ok {
						return resp, nil
					}
					return branchesResponse([]bitbucketcl.RepositoryBranch{}, "", rbo.PageNum), nil
				}
			}(),
			rbo: &bitbucketcl.RepositoryBranchOptions{},
			want: []*bitbucketcl.RepositoryBranch{
				{Name: "main"},
				{Name: "develop"},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error on first page",
			fetchPage: func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error) {
				return nil, errors.New("fetch failed")
			},
			rbo:     &bitbucketcl.RepositoryBranchOptions{},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "error on second page",
			fetchPage: func() func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error) {
				calls := 0
				return func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error) {
					calls++
					if calls == 1 {
						return branchesResponse([]bitbucketcl.RepositoryBranch{branch("main")}, "next", 1), nil
					}
					return nil, errors.New("fetch failed on second page")
				}
			}(),
			rbo: &bitbucketcl.RepositoryBranchOptions{},
			want: []*bitbucketcl.RepositoryBranch{
				{Name: "main"},
			},
			wantErr: assert.Error,
		},
		{
			name: "empty response",
			fetchPage: func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error) {
				return branchesResponse([]bitbucketcl.RepositoryBranch{}, "", 1), nil
			},
			rbo:     &bitbucketcl.RepositoryBranchOptions{},
			want:    []*bitbucketcl.RepositoryBranch{},
			wantErr: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := xiter.CollectFromScan(ScanBitbucketBranches(tt.fetchPage, tt.rbo))

			// Compare the results using slices.EqualFunc for proper comparison
			assert.True(t, slices.EqualFunc(got, tt.want, func(a, b *bitbucketcl.RepositoryBranch) bool {
				return a.Name == b.Name
			}))

			tt.wantErr(t, err)
		})
	}
}
