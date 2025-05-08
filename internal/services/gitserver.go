package services

import (
	"context"
	"errors"

	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	codebaseUtil "github.com/epam/edp-codebase-operator/v2/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GitServerService struct {
	k8sClinet client.Client
	namespace string
}

func NewGitServerService(k8sClinet client.Client, namespace string) *GitServerService {
	return &GitServerService{
		k8sClinet: k8sClinet,
		namespace: namespace,
	}
}

func (g *GitServerService) GetGitProviderToken(ctx context.Context, gitServerName string) (string, error) {
	gitServer := &codebaseApi.GitServer{}
	if err := g.k8sClinet.Get(
		ctx,
		client.ObjectKey{Name: gitServerName, Namespace: g.namespace},
		gitServer,
	); err != nil {
		return "", err
	}

	secret := &corev1.Secret{}
	if err := g.k8sClinet.Get(
		ctx,
		client.ObjectKey{Name: gitServer.Spec.NameSshKeySecret, Namespace: g.namespace},
		secret,
	); err != nil {
		return "", err
	}

	token := string(secret.Data[codebaseUtil.GitServerSecretTokenField])

	if token == "" {
		return "", errors.New("git provider token is empty")
	}

	return token, nil
}
