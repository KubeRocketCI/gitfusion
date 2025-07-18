package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitlabProvider struct{}

func NewGitlabProvider() *GitlabProvider {
	return &GitlabProvider{}
}

func (g *GitlabProvider) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
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

func (g *GitlabProvider) ListRepositories(
	ctx context.Context,
	owner string,
	settings krci.GitServerSettings,
	listOptions models.ListOptions,
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

// ListUserOrganizations returns organizations for the authenticated user
func (g *GitlabProvider) ListUserOrganizations(
	ctx context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
	if err != nil {
		return nil, err
	}

	it := gitlab.Scan2(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Group, *gitlab.Response, error) {
		return client.Groups.ListGroups(&gitlab.ListGroupsOptions{}, gitlab.WithContext(ctx), p)
	})

	result := make([]models.Organization, 0)

	for group, err := range it {
		if err != nil {
			return nil, fmt.Errorf("failed to list groups: %w", err)
		}

		org := models.Organization{
			Id:   strconv.Itoa(group.ID),
			Name: group.FullPath,
		}
		if group.AvatarURL != "" {
			org.AvatarUrl = &group.AvatarURL
		}

		result = append(result, org)
	}

	return result, nil
}

// ListBranches implements BranchesProvider for GitlabService.
func (g *GitlabProvider) ListBranches(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	_ models.ListOptions,
) ([]models.Branch, error) {
	client, err := gitlab.NewClient(settings.Token, gitlab.WithBaseURL(settings.Url))
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	it := gitlab.Scan2(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Branch, *gitlab.Response, error) {
		return client.Branches.ListBranches(
			fmt.Sprintf("%s/%s", owner, repo),
			&gitlab.ListBranchesOptions{},
			gitlab.WithContext(ctx),
			p,
		)
	})

	result := make([]models.Branch, 0)

	for b, err := range it {
		if err != nil {
			return nil, fmt.Errorf("failed to list branches for %s/%s: %w", owner, repo, err)
		}

		result = append(result, models.Branch{
			Name: b.Name,
		})
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

func newGitlabRepositoryListByOrgOptions(listOptions models.ListOptions) *gitlab.ListGroupProjectsOptions {
	return &gitlab.ListGroupProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
		Search: listOptions.Name,
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
