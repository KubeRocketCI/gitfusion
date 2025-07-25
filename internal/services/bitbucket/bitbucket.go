package bitbucket

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	gferrors "github.com/KubeRocketCI/gitfusion/internal/errors"
	"github.com/KubeRocketCI/gitfusion/internal/models"
	"github.com/KubeRocketCI/gitfusion/internal/services/krci"
	bitbucketpkg "github.com/KubeRocketCI/gitfusion/pkg/bitbucket"
	"github.com/ktrysmt/go-bitbucket"
)

type BitbucketService struct{}

func NewBitbucketProvider() *BitbucketService {
	return &BitbucketService{}
}

func (b *BitbucketService) GetRepository(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
) (*models.Repository, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, err
	}

	client := bitbucket.NewBasicAuth(username, password)

	repoOptions := &bitbucket.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repo,
	}

	repository, err := client.Repositories.Repository.Get(repoOptions)
	if err != nil {
		if err.Error() == "404 Not Found" {
			return nil, fmt.Errorf("repository %s/%s %w", owner, repo, gferrors.ErrNotFound)
		}

		return nil, fmt.Errorf("failed to get repository %s/%s: %w", owner, repo, err)
	}

	return convertBitbucketRepoToRepository(repository), nil
}

func (b *BitbucketService) ListRepositories(
	ctx context.Context,
	account string,
	settings krci.GitServerSettings,
	listOptions models.ListOptions,
) ([]models.Repository, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, err
	}

	client := bitbucket.NewBasicAuth(username, password)
	repoOptions := &bitbucket.RepositoriesOptions{
		Owner:   account,
		Keyword: listOptions.Name,
	}

	repositories, err := client.Repositories.ListForAccount(repoOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories for account %s: %w", account, err)
	}

	result := make([]models.Repository, 0, len(repositories.Items))
	for _, repo := range repositories.Items {
		result = append(result, *convertBitbucketRepoToRepository(&repo))
	}

	return result, nil
}

// ListUserOrganizations returns workspaces for the authenticated user using go-bitbucket client
func (b *BitbucketService) ListUserOrganizations(
	_ context.Context,
	settings krci.GitServerSettings,
) ([]models.Organization, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, err
	}

	client := bitbucket.NewBasicAuth(username, password)

	workspaces, err := client.Workspaces.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}

	result := make([]models.Organization, 0, len(workspaces.Workspaces))

	for _, ws := range workspaces.Workspaces {
		org := models.Organization{
			Id:   ws.UUID,
			Name: ws.Name,
		}

		result = append(result, org)
	}

	return result, nil
}

// ListBranches implements BranchesProvider for BitbucketService.
func (b *BitbucketService) ListBranches(
	ctx context.Context,
	owner, repo string,
	settings krci.GitServerSettings,
	_ models.ListOptions,
) ([]models.Branch, error) {
	username, password, err := decodeBitbucketToken(settings.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bitbucket token: %w", err)
	}

	client := bitbucket.NewBasicAuth(username, password)
	branchOptions := &bitbucket.RepositoryBranchOptions{
		Owner:    owner,
		RepoSlug: repo,
		Pagelen: 100,
	}

	scanBranches := bitbucketpkg.ScanBitbucketBranches(
		func(rbo *bitbucket.RepositoryBranchOptions) (*bitbucket.RepositoryBranches, error) {
			return client.Repositories.Repository.ListBranches(rbo)
		},
		branchOptions,
	)

	result := make([]models.Branch, 0)

	for b, err := range scanBranches {
		if err != nil {
			return nil, fmt.Errorf("failed to list branches: %w", err)
		}

		result = append(result, models.Branch{
			Name: b.Name,
		})
	}

	return result, nil
}

func convertBitbucketRepoToRepository(repo *bitbucket.Repository) *models.Repository {
	if repo == nil {
		return nil
	}

	// Extract owner username and URL from the generic map fields
	ownerUsername := ""
	if owner, ok := repo.Owner["username"].(string); ok {
		ownerUsername = owner
	}

	repoUrl := ""

	if html, ok := repo.Links["html"].(map[string]interface{}); ok {
		if href, ok := html["href"].(string); ok {
			repoUrl = href
		}
	}

	return &models.Repository{
		DefaultBranch: &repo.Mainbranch.Name,
		Description:   &repo.Description,
		Id:            repo.Uuid,
		Name:          repo.Name,
		Owner:         &ownerUsername,
		Url:           &repoUrl,
	}
}

func decodeBitbucketToken(token string) (username, password string, err error) {
	decodedToken, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode token: %w", err)
	}

	basicAuth := strings.Split(string(decodedToken), ":")

	if len(basicAuth) != 2 {
		return "", "", fmt.Errorf("invalid token format")
	}

	return basicAuth[0], basicAuth[1], nil
}
