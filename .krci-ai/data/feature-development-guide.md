# Feature Development Guide

Quick reference for implementing new features in GitFusion following project patterns and best practices.

## Core Principles

- Write clean, idiomatic Go code following established patterns
- Use OpenAPI spec as source of truth for API changes
- Abstract provider-specific details behind common interfaces
- Implement comprehensive error handling with context
- Add tests for all new functionality

## Feature Implementation Workflow

### 1. API Design

- Update `internal/api/oapi.yaml` with new endpoints/models
- Run `make generate` to regenerate server stubs and models
- Keep JSON responses consistent (camelCase)
- Use standard HTTP status codes (200, 201, 400, 401, 404, 500)

### 2. Error Handling

- Use custom errors from `internal/errors` package
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Return early on errors (minimize nesting)
- Map provider errors to domain errors

### 3. Service Layer

- Create provider interface in `internal/services/{feature}/`
- Implement providers for each Git provider (GitHub, GitLab, Bitbucket)
- Use caching with `github.com/viccon/sturdyc` for expensive operations
- Keep providers focused on single responsibility

### 4. Handler Implementation

- Place handlers in `internal/api/`
- Validate input parameters
- Call service layer methods
- Return consistent error responses
- Document exported functions

### 5. Testing

- Write table-driven tests for multiple scenarios
- Mock external dependencies
- Test both success and error paths
- Use `github.com/stretchr/testify/assert` for assertions
- Aim for meaningful test coverage

## Code Style

- Keep line length readable (split parameters if >120 chars)
- Use `gofmt` and `goimports`
- Add comments for complex logic
- Prefer self-documenting code
- Follow Go naming conventions (mixedCaps)

## Git Provider Integration

- Maintain consistent interfaces across providers
- Use official client libraries when available
- Add logging with redacted sensitive data
- Handle provider-specific errors gracefully
- Document provider limitations

## Security

- Validate all input data
- Sanitize user-provided values
- Use HTTPS for external communications
- Apply proper authentication/authorization
- Handle tokens securely
