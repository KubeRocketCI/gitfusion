---
applyTo: "**/*.go"
---
# Go Development Guidelines for GitFusion

## Coding Style
- Follow Go best practices and idioms
- Use latest Go 1.24 features
- Split function parameters into separate lines if they exceed 120 characters
- Use meaningful variable and function names
- Keep functions concise and focused
- Properly handle errors with appropriate context
- Add comments for complex logic, but prefer self-documenting code

## Error Handling
- Always check error returns
- Add context to errors using `fmt.Errorf("operation failed: %w", err)`
- Return early on errors instead of nesting conditionals
- Use custom error types from `internal/errors` for domain-specific errors
- Log errors with appropriate context and severity

## API Development
- All API handlers should be in the `internal/api` package
- Use the OpenAPI specification in `internal/api/oapi.yaml` as the source of truth
- Generate API server stubs and models using `oapi-codegen` by running `make generate`
- Implement proper request validation and error responses
- Include appropriate status codes and error messages in responses

## Testing
- Write unit tests for all functions with meaningful assertions
- Use table-driven tests for functions with multiple input/output combinations
- Mock external dependencies with interfaces
- Aim for high test coverage, especially for critical business logic
- Use testify for assertions: `github.com/stretchr/testify/assert`

## Documentation
- Document all exported functions, types, and constants
- Include examples for complex APIs
- Keep comments up-to-date with code changes
