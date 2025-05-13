# GitFusion

[![Apache License 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Overview

GitFusion is a unified Git provider integration microservice designed to enhance user experience when working with git repositories across multiple providers (GitHub, GitLab, Bitbucket). It acts as a consistent interface between [portal UI](https://github.com/epam/edp-headlamp) and various git providers, enabling users to easily discover and interact with codebases, branches, pull requests, and other git capabilities.

## Business Value

- Enhance user productivity by providing a seamless experience across different git providers
- Reduce context switching for users when working with multiple git platforms
- Standardize git operations through a consistent API interface
- Improve discoverability of repositories, branches, and other git resources
- Enable future extensibility of git-related features in our portal

## Key Features

GitFusion provides the following features (implemented incrementally):

- Repository discovery and search across providers
- Branch management and information retrieval
- Pull/Merge request handling and visibility
- Unified API for interacting with different git providers

## Architecture

GitFusion is built as a RESTful API service with the following characteristics:

- Integration with multiple git providers (GitHub, GitLab, Bitbucket) via their respective APIs
- Authentication via API keys with Kubernetes secrets
- Kubernetes-native deployment with Helm charts
- Written in Go with a clean, extensible architecture

## Getting Started

### Prerequisites

- Go 1.24+
- Kubernetes cluster with configured GitServer CRDs
- Access to Git provider API tokens

### Building

```bash
# Build the binary
make build

# Run tests
make test

# Generate API code from OpenAPI spec
make generate
```

### Deployment

GitFusion can be deployed using the provided Helm chart:

```bash
# From project root
helm install gitfusion ./deploy-templates -n my-namespace
```

## API Endpoints

GitFusion exposes a RESTful API defined using OpenAPI specification. Key endpoints include:

- `/api/v1/providers/github/{git-server}/{org}/repositories` - List organization repositories
- `/api/v1/providers/github/{git-server}/repositories/{owner}/{repo}` - Get repository details

For full API documentation, refer to the OpenAPI specification at `internal/api/oapi.yaml`.

## Contributing

We welcome contributions to GitFusion! Please see our [Contributing Guide](CONTRIBUTING.md) for more information on how to get started. Also, please note that all contributions are governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Security

For information about security policies and procedures, please refer to our [Security Policy](SECURITY.md).

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
