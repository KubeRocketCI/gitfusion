# GitFusion Project Custom Instructions for GitHub Copilot

This file provides guidance to GitHub Copilot to ensure generated content aligns with the GitFusion project standards.

## Project Context
GitFusion is a service providing a unified interface for interacting with various Git hosting providers (GitHub, GitLab, Bitbucket). It abstracts provider differences and offers a consistent API for common operations.

## Core Functionality
- Repository management (create, list, update, delete)
- User/org management
- Branch protection rules
- Pull/merge request operations
- Webhook management
- Authentication/authorization

## Technology Stack
- Go for backend services (Go 1.24)
- Kubernetes for deployment
- Helm charts for packaging
- OpenAPI for API specification
- github.com/oapi-codegen/oapi-codegen for API generation

## Pull Request Guidelines
When generating PR titles and descriptions, please refer to the [PR generation guidelines](./instructions/pr-generation.instructions.md).

## Code Style Guidelines
- Follow Go best practices and idioms
- Use meaningful variable and function names
- Keep functions concise and focused
- Properly handle errors with appropriate context
- Add comments for complex logic, but prefer self-documenting code
- Follow project structure conventions:
  - `/cmd` - Main applications
  - `/internal` - Private application and library code
  - `/pkg` - Public libraries that can be imported by other projects
  - `/deploy-templates` - Deployment configuration (Helm charts)

## Error Handling Guidelines
- Always check error returns
- Add context to errors using `fmt.Errorf("operation failed: %w", err)`
- Return early on errors instead of nesting conditionals
- Use custom error types from `internal/errors` for domain-specific errors
- Log errors with appropriate context and severity

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

## Testing Guidelines
- Write unit tests for all functions
- Include integration tests for API endpoints
- Aim for high test coverage
- Mock external dependencies
