package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	gfgithub "github.com/KubeRocketCI/gitfusion/pkg/github"
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
	"github.com/KubeRocketCI/gitfusion/pkg/xiter"
	"github.com/google/go-github/v72/github"
	"golang.org/x/sync/errgroup"
)

type GitHubProvider struct{}

func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{}
}

func (g *GitHubProvider) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
) (*models.Repository, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

	repository, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		ghErr := &github.ErrorResponse{}
		if errors.As(err, &ghErr) {
			if ghErr.Response.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("repository %s/%s: %w", owner, repo, gferrors.ErrNotFound)
			}
		}

		return nil, fmt.Errorf("failed to get repository %s/%s: %w", owner, repo, err)
	}

	return convertGitHubRepoToRepository(repository), nil
}

func (g *GitHubProvider) ListRepositories(
	ctx context.Context,
	owner string,
	settings krci.GitServerSettings,
	listOptions models.ListOptions,
) ([]models.Repository, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

	it := filterRepositoriesByName(
		g.listRepositories(ctx, owner, client),
		listOptions,
	)

	result := make([]models.Repository, 0)

	for repo, err := range it {
		if err != nil {
			ghErr := &github.ErrorResponse{}
			if errors.As(err, &ghErr) {
				if ghErr.Response.StatusCode == http.StatusNotFound {
					return nil, fmt.Errorf("organization or user %s: %w", owner, gferrors.ErrNotFound)
				}
			}

			return nil, fmt.Errorf("failed to list repositories for org %s: %w", owner, err)
		}

		result = append(result, *convertGitHubRepoToRepository(repo))
	}

	return result, nil
}

func filterRepositoriesByName(
	scan xiter.Scan[*github.Repository],
	opt models.ListOptions,
) xiter.Scan[*github.Repository] {
	return func(yield func(*github.Repository, error) bool) {
		scan(func(repo *github.Repository, err error) bool {
			if err != nil {
				return yield(nil, err)
			}

			if opt.Name == nil {
				return yield(repo, nil)
			}

			if repo == nil {
				return true
			}

			if strings.Contains(
				strings.ToLower(repo.GetName()),
				strings.ToLower(pointer.ValueOrEmpty(opt.Name)),
			) {
				return yield(repo, nil)
			}

			return true
		})
	}
}

func (g *GitHubProvider) listRepositories(
	ctx context.Context,
	owner string,
	client *github.Client,
) xiter.Scan[*github.Repository] {
	_, _, orgErr := client.Organizations.Get(ctx, owner)
	if orgErr == nil {
		return gfgithub.ScanGitHubList(
			func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
				return client.Repositories.ListByOrg(
					ctx,
					owner,
					&github.RepositoryListByOrgOptions{
						ListOptions: opt,
					},
				)
			},
		)
	}

	return gfgithub.ScanGitHubList(
		func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
			return client.Repositories.ListByUser(
				ctx,
				owner,
				&github.RepositoryListByUserOptions{
					ListOptions: opt,
				},
			)
		},
	)
}

func convertGitHubRepoToRepository(repo *github.Repository) *models.Repository {
	if repo == nil {
		return nil
	}

	var name, owner, url string
	if repo.Name != nil {
		name = *repo.Name
	}

	if repo.Owner != nil && repo.Owner.Login != nil {
		owner = *repo.Owner.Login
	}

	if repo.HTMLURL != nil {
		url = *repo.HTMLURL
	}

	return &models.Repository{
		DefaultBranch: repo.DefaultBranch,
		Description:   repo.Description,
		Id:            strconv.FormatInt(repo.GetID(), 10),
		Name:          name,
		Owner:         &owner,
		Url:           &url,
		Visibility:    convertVisibility(repo.GetPrivate()),
	}
}

func convertVisibility(isPrivate bool) *models.RepositoryVisibility {
	if isPrivate {
		visibility := models.RepositoryVisibilityPrivate

		return &visibility
	}

	visibility := models.RepositoryVisibilityPublic

	return &visibility
}

// ListUserOrganizations returns organizations for the authenticated user.
// Also it adds the current user as an organization.
func (g *GitHubProvider) ListUserOrganizations(
	ctx context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)
	eg, ctx := errgroup.WithContext(ctx)

	var userOrg *models.Organization
	// Goroutine 1: Get current user
	eg.Go(func() error {
		user, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}

		userOrg = &models.Organization{
			Id:        strconv.FormatInt(user.GetID(), 10),
			Name:      user.GetLogin(),
			AvatarUrl: user.AvatarURL,
		}

		return nil
	})

	result := make([]models.Organization, 0, 10)

	// Goroutine 2: Get organizations
	eg.Go(func() error {
		it := gfgithub.ScanGitHubList(
			func(opt github.ListOptions) ([]*github.Membership, *github.Response, error) {
				return client.Organizations.ListOrgMemberships(
					ctx,
					&github.ListOrgMembershipsOptions{
						State:       "active",
						ListOptions: opt,
					},
				)
			},
		)

		for membership, err := range it {
			if err != nil {
				return fmt.Errorf("failed to list organizations: %w", err)
			}

			org := membership.Organization
			if org == nil {
				continue
			}

			orgModel := models.Organization{
				Id:        strconv.FormatInt(org.GetID(), 10),
				Name:      org.GetLogin(),
				AvatarUrl: org.AvatarURL,
			}

			result = append(result, orgModel)
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if userOrg != nil {
		result = append(result, *userOrg)
	}

	return result, nil
}

// ListBranches implements BranchesProvider for GitHubService.
// Returns all branches for the given repository. Pagination fields in the response reflect the full result.
func (g *GitHubProvider) ListBranches(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	_ models.ListOptions,
) ([]models.Branch, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

	it := gfgithub.ScanGitHubList(
		func(opt github.ListOptions) ([]*github.Branch, *github.Response, error) {
			branchOpts := &github.BranchListOptions{
				ListOptions: opt,
			}

			return client.Repositories.ListBranches(ctx, owner, repo, branchOpts)
		},
	)

	branches := make([]models.Branch, 0)

	for b, err := range it {
		if err != nil {
			return nil, fmt.Errorf("failed to list branches: %w", err)
		}

		branches = append(branches, models.Branch{
			Name: b.GetName(),
		})
	}

	return branches, nil
}
