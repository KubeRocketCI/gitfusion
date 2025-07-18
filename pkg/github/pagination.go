package github

import (
	"github.com/KubeRocketCI/gitfusion/pkg/xiter"
	"github.com/google/go-github/v72/github"
)

// ScanGitHubList iterates over paginated GitHub API list results, yielding each item.
// - fetchPage: function that fetches a page of results using github.ListOptions.
// - opts: optional functional options (e.g., WithPerPage).
// Returns an iterator (iter.Seq2) yielding each item and any error encountered.
func ScanGitHubList[T any](
	fetchPage func(opt github.ListOptions) ([]T, *github.Response, error),
	opts ...ScanGitHubListOption,
) xiter.Scan[T] {
	return func(yield func(T, error) bool) {
		opt := github.ListOptions{PerPage: 100}
		for _, o := range opts {
			o(&opt)
		}

		for {
			list, resp, err := fetchPage(opt)
			if err != nil {
				var t T

				yield(t, err)

				return
			}

			for _, item := range list {
				if !yield(item, nil) {
					return
				}
			}

			if resp.NextPage == 0 {
				break
			}

			opt.Page = resp.NextPage
		}
	}
}

// ScanGitHubListOption defines a functional option for ScanGitHubList.
type ScanGitHubListOption func(*github.ListOptions)

// WithPerPage sets the PerPage value for pagination.
func WithPerPage(perPage int) ScanGitHubListOption {
	return func(opt *github.ListOptions) {
		if perPage > 0 {
			opt.PerPage = perPage
		}
	}
}
