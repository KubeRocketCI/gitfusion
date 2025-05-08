package services

import (
	"context"

	"github.com/KubeRocketCI/gitfusion/internal/models"
)

type GitHubService struct{}

func (g *GitHubService) GetRepository(ctx context.Context, token, repositoryID string) (models.Repository, error) {
	//TODO: Implement the logic to get a GitHub repository using the provided token and repository ID.
	return models.Repository{}, nil
}
