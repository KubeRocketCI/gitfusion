package services

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
	"strings"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	gfgithub "github.com/KubeRocketCI/gitfusion/pkg/github"
	"github.com/KubeRocketCI/gitfusion/pkg/pointer"
	"github.com/google/go-github/v72/github"
)

type GitHubService struct{}

func NewGitHubService() *GitHubService {
	return &GitHubService{}
}

func (g *GitHubService) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings GitProviderSettings,
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

type ListOptions struct {
	Name *string
}

func (g *GitHubService) ListRepositories(
	ctx context.Context,
	owner string,
	settings GitProviderSettings,
	listOptions ListOptions,
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
	it iter.Seq2[*github.Repository, error],
	opt ListOptions,
) iter.Seq2[*github.Repository, error] {
	return func(yield func(*github.Repository, error) bool) {
		it(func(repo *github.Repository, err error) bool {
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

func (g *GitHubService) listRepositories(
	ctx context.Context,
	owner string,
	client *github.Client,
) iter.Seq2[*github.Repository, error] {
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

// ListUserOrganizations returns organizations for the authenticated user
func (g *GitHubService) ListUserOrganizations(
	ctx context.Context,
	settings GitProviderSettings,
) ([]models.Organization, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

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

	result := make([]models.Organization, 0)

	for membership, err := range it {
		if err != nil {
			return nil, fmt.Errorf("failed to list organizations: %w", err)
		}

		org := membership.Organization
		if org == nil {
			continue
		}

		orgModel := models.Organization{
			Id:   strconv.FormatInt(org.GetID(), 10),
			Name: org.GetLogin(),
		}
		if org.AvatarURL != nil {
			orgModel.AvatarUrl = org.AvatarURL
		}

		result = append(result, orgModel)
	}

	return result, nil
}
