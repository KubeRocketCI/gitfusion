package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitlabService struct{}

func NewGitlabService() *GitlabService {
	return &GitlabService{}
}

func (g *GitlabService) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings GitProviderSettings,
) (*models.Repository, error) {
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
	if err != nil {
		return nil, err
	}

	repository, _, err := client.Projects.GetProject(
		fmt.Sprintf("%s/%s", owner, repo),
		nil,
		gitlab.WithContext(ctx),
	)
	if err != nil {
		if errors.Is(err, gitlab.ErrNotFound) {
			return nil, fmt.Errorf("repository %s/%s %w", owner, repo, gferrors.ErrNotFound)
		}

		return nil, err
	}

	return convertGitlabRepoToRepository(repository), nil
}

func (g *GitlabService) ListOrganizationsRepositories(
	ctx context.Context,
	org string,
	settings GitProviderSettings,
	listOptions ListOptions,
) ([]models.Repository, error) {
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
	if err != nil {
		return nil, err
	}

	repositories, _, err := client.Groups.ListGroupProjects(
		org,
		&gitlab.ListGroupProjectsOptions{
			ListOptions: *newGitlabRepositoryListByOrgOptions(listOptions),
		},
		gitlab.WithContext(ctx),
	)
	if err != nil {
		if errors.Is(err, gitlab.ErrNotFound) {
			return nil, fmt.Errorf("organization %s %w", org, gferrors.ErrNotFound)
		}

		return nil, err
	}

	result := make([]models.Repository, 0, len(repositories))
	for _, repo := range repositories {
		result = append(result, *convertGitlabRepoToRepository(repo))
	}

	return result, nil
}

func convertGitlabRepoToRepository(repo *gitlab.Project) *models.Repository {
	if repo == nil {
		return nil
	}

	return &models.Repository{
		DefaultBranch: &repo.DefaultBranch,
		Description:   &repo.Description,
		Id:            strconv.Itoa(repo.ID),
		Name:          repo.Name,
		Owner:         &repo.Namespace.FullPath,
		Url:           &repo.WebURL,
		Visibility:    convertVisibility(repo.Visibility == gitlab.PrivateVisibility),
	}
}

func newGitlabRepositoryListByOrgOptions(listOptions ListOptions) *gitlab.ListOptions {
	opt := &gitlab.ListOptions{}

	if listOptions.PerPage != nil {
		opt.PerPage = *listOptions.PerPage
	}

	if listOptions.Page != nil {
		opt.Page = *listOptions.Page
	}

	return opt
}
