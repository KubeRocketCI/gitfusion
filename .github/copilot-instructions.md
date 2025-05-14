# GitFusion Project Custom Instructions for GitHub Copilot

This file provides guidance to GitHub Copilot to ensure generated content aligns with the GitFusion project standards.

## Project Context
GitFusion is a service designed to provide a unified interface for interacting with various Git hosting providers (GitHub, GitLab, etc.).

## Technology Stack
- Go for backend services
- Kubernetes for deployment
- Helm charts for packaging

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
  - `/deploy-templates` - Deployment configuration

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
