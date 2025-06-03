# GitFusion Project Custom Instructions for GitHub Copilot

This file provides guidance to GitHub Copilot to ensure generated content aligns with the GitFusion project standards.

## Project Context
GitFusion is a service providing a unified interface for interacting with various Git hosting providers (GitHub, GitLab, Bitbucket). It abstracts provider differences and offers a consistent API for common operations.

## Core Functionality
- Repository management (create, list, update, delete)
- User/org management
- Branch management
- Pull/merge request operations
- Webhook management

## Technology Stack
- Go for backend services (Go 1.24)
- Kubernetes for deployment
- Helm charts for packaging
- OpenAPI for API specification
- `github.com/oapi-codegen/oapi-codegen` for API generation

## Project structure
- Follow project structure conventions:
  - `/cmd` - Application executables
  - `/internal/api` - API handlers, and OpenAPI definitions
  - `/internal/service` - Business logic and service implementations
  - `/internal/models` - Domain models and data structures
  - `/internal/errors` - Custom error types
  - `/pkg` - Public libraries that can be imported by other projects
  - `/deploy-templates` - Deployment configuration (Helm charts)

## API Design Guidelines
- Follow RESTful principles
- Document all endpoints in OpenAPI specification
- Implement consistent error responses
- Support pagination for list operations
- Use proper HTTP status codes

## Documentation Guidelines
- Document all public APIs
- Include examples for complex functionality
- Keep README.md files up to date
- Use proper Markdown formatting

