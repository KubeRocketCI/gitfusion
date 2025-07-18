package bitbucket

import (
	"github.com/KubeRocketCI/gitfusion/pkg/xiter"
	bitbucketcl "github.com/ktrysmt/go-bitbucket"
)

// ScanBitbucketBranches scans all branches for a given repository.
func ScanBitbucketBranches(
	fetchPage func(rbo *bitbucketcl.RepositoryBranchOptions) (*bitbucketcl.RepositoryBranches, error),
	rbo *bitbucketcl.RepositoryBranchOptions,
) xiter.Scan[*bitbucketcl.RepositoryBranch] {
	return func(yield func(*bitbucketcl.RepositoryBranch, error) bool) {
		for {
			branchesResp, err := fetchPage(rbo)
			if err != nil {
				yield(nil, err)

				return
			}

			for _, branch := range branchesResp.Branches {
				if !yield(&branch, nil) {
					return
				}
			}

			if branchesResp.Next == "" {
				break
			}

			rbo.PageNum = branchesResp.Page + 1
		}
	}
}
