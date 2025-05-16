package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
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
				return nil, fmt.Errorf("repository %s/%s %w", owner, repo, gferrors.ErrNotFound)
			}
		}

		return nil, err
	}

	return convertGitHubRepoToRepository(repository), nil
}

type ListOptions struct {
	PerPage *int
	Page    *int
}

func (g *GitHubService) ListRepositories(
	ctx context.Context,
	org string,
	settings GitProviderSettings,
	listOptions ListOptions,
) ([]models.Repository, error) {
	client := github.NewClient(nil).WithAuthToken(settings.Token)

	opt := newRepositoryListByOrgOptions(listOptions)

	repositories, _, err := client.Repositories.ListByOrg(ctx, org, opt)
	if err != nil {
		ghErr := &github.ErrorResponse{}
		if errors.As(err, &ghErr) {
			if ghErr.Response.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("organization %s %w", org, gferrors.ErrNotFound)
			}
		}

		return nil, err
	}

	result := make([]models.Repository, 0, len(repositories))
	for _, repo := range repositories {
		result = append(result, *convertGitHubRepoToRepository(repo))
	}

	return result, nil
}

func newRepositoryListByOrgOptions(listOptions ListOptions) *github.RepositoryListByOrgOptions {
	opt := &github.RepositoryListByOrgOptions{
		Type: "all",
	}

	if listOptions.PerPage != nil {
		opt.PerPage = *listOptions.PerPage
	}

	if listOptions.Page != nil {
		opt.Page = *listOptions.Page
	}

	return opt
}

func convertGitHubRepoToRepository(repo *github.Repository) *models.Repository {
	if repo == nil {
		return nil
	}

	return &models.Repository{
		DefaultBranch: repo.DefaultBranch,
		Description:   repo.Description,
		Id:            strconv.FormatInt(repo.GetID(), 10),
		Name:          *repo.Name,
		Owner:         repo.Owner.Login,
		Url:           repo.HTMLURL,
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
