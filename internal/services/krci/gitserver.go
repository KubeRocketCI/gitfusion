package krci

import (
	"context"
	"errors"
	"fmt"

	codebaseApi "github.com/epam/edp-codebase-operator/v2/api/v1"
	gitprovider "github.com/epam/edp-codebase-operator/v2/pkg/gitprovider"
	codebaseUtil "github.com/epam/edp-codebase-operator/v2/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GitServerService struct {
	k8sClinet client.Client
	namespace string
}

type GitServerSettings struct {
	Url           string
	Token         string
	GitProvider   string
	GitServerName string
}

func NewGitServerService(k8sClinet client.Client, namespace string) *GitServerService {
	return &GitServerService{
		k8sClinet: k8sClinet,
		namespace: namespace,
	}
}

func (g *GitServerService) GetGitProviderSettings(
	ctx context.Context,
	gitServerName string,
) (GitServerSettings, error) {
	gitServer := &codebaseApi.GitServer{}
	if err := g.k8sClinet.Get(
		ctx,
		client.ObjectKey{Name: gitServerName, Namespace: g.namespace},
		gitServer,
	); err != nil {
		return GitServerSettings{}, err
	}

	return g.getGitProviderSettingsForServer(ctx, gitServer)
}

// GetGitProviderSettingsList returns a list of GitProviderSettings for all GitServers in the namespace.
func (g *GitServerService) GetGitProviderSettingsList(ctx context.Context) ([]GitServerSettings, error) {
	gitServerList := &codebaseApi.GitServerList{}
	if err := g.k8sClinet.List(ctx, gitServerList, client.InNamespace(g.namespace)); err != nil {
		return nil, fmt.Errorf("failed to list GitServers: %w", err)
	}

	settingsList := make([]GitServerSettings, 0, len(gitServerList.Items))

	for i := range gitServerList.Items {
		gs := &gitServerList.Items[i]
		settings, err := g.getGitProviderSettingsForServer(ctx, gs)

		if err != nil {
			return nil, fmt.Errorf("failed to get settings for GitServer %s: %w", gs.Name, err)
		}

		settingsList = append(settingsList, settings)
	}

	return settingsList, nil
}

func (g *GitServerService) getGitProviderSettingsForServer(
	ctx context.Context,
	gitServer *codebaseApi.GitServer,
) (GitServerSettings, error) {
	secret := &corev1.Secret{}
	if err := g.k8sClinet.Get(
		ctx,
		client.ObjectKey{Name: gitServer.Spec.NameSshKeySecret, Namespace: g.namespace},
		secret,
	); err != nil {
		return GitServerSettings{}, err
	}

	token := string(secret.Data[codebaseUtil.GitServerSecretTokenField])
	if token == "" {
		return GitServerSettings{}, errors.New("git provider token is empty")
	}

	return GitServerSettings{
		Url:           gitprovider.GetGitProviderAPIURL(gitServer),
		Token:         token,
		GitProvider:   gitServer.Spec.GitProvider,
		GitServerName: gitServer.Name,
	}, nil
}
