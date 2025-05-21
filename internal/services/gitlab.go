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

func (g *GitlabService) ListRepositories(
	ctx context.Context,
	owner string,
	settings GitProviderSettings,
	listOptions ListOptions,
) ([]models.Repository, error) {
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
	if err != nil {
		return nil, err
	}

	it := gitlab.Scan2(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Project, *gitlab.Response, error) {
		return client.Groups.ListGroupProjects(
			owner,
			newGitlabRepositoryListByOrgOptions(listOptions),
			gitlab.WithContext(ctx),
		)
	})

	result := make([]models.Repository, 0)

	for repo, err := range it {
		if err != nil {
			if errors.Is(err, gitlab.ErrNotFound) {
				return nil, fmt.Errorf("owner %s %w", owner, gferrors.ErrNotFound)
			}

			return nil, err
		}

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

func newGitlabRepositoryListByOrgOptions(listOptions ListOptions) *gitlab.ListGroupProjectsOptions {
	return &gitlab.ListGroupProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
		Search: listOptions.Name,
	}
}
