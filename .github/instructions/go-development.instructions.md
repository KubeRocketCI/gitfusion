---
applyTo: "**/*.go"
---
# Go Development Guidelines for GitFusion

## Coding Style
- Follow Go formatting conventions (enforced by golangci-lint, run `make lint-fix` to format code)
- Organize imports in three groups: standard library, external libraries, and internal packages
- Use meaningful variable and function names that describe their purpose
- Prefer shorter variable names for limited scopes, longer names for wider scopes

## Error Handling
- Always check errors; never ignore them without a comment explaining why
- Use custom error types from the `internal/errors` package when appropriate
- Include context when wrapping errors: `fmt.Errorf("failed to fetch repositories: %w", err)`
- Return early on errors instead of using deep nesting

## API Development
- All API handlers should be in the `internal/api` package
- Use the OpenAPI specification in `internal/api/oapi.yaml` as the source of truth
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

## Performance Considerations
- Avoid unnecessary memory allocations
- Use pointer receivers only when necessary
- Consider using sync.Pool for frequently allocated objects
- Profile code for performance bottlenecks before optimizing
