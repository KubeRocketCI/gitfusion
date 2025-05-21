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
	org string,
	settings GitProviderSettings,
	listOptions ListOptions,
) ([]models.Repository, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

	it := filterrepositoriesByName(
		gfgithub.ScanGitHubList(
			func(opt github.ListOptions) ([]*github.Repository, *github.Response, error) {
				return client.Repositories.ListByOrg(
					ctx,
					org,
					&github.RepositoryListByOrgOptions{
						ListOptions: github.ListOptions{PerPage: 100},
					},
				)
			},
		),
		listOptions,
	)

	result := make([]models.Repository, 0)

	for repo, err := range it {
		if err != nil {
			ghErr := &github.ErrorResponse{}
			if errors.As(err, &ghErr) {
				if ghErr.Response.StatusCode == http.StatusNotFound {
					return nil, fmt.Errorf("organization %s: %w", org, gferrors.ErrNotFound)
				}
			}

			return nil, fmt.Errorf("failed to list repositories for org %s: %w", org, err)
		}

		result = append(result, *convertGitHubRepoToRepository(repo))
	}

	return result, nil
}

func filterrepositoriesByName(
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
