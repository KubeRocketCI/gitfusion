---
dependencies:
  data:
    - go-coding-standards.md
    - feature-development-guide.md
---

# Task: Add New Feature

Implement a new feature in GitFusion following project patterns, coding standards, and API-first design principles.

## Instructions

<instructions>
Read and understand [Go Coding Standards](./.krci-ai/data/go-coding-standards.md) and [Feature Development Guide](./.krci-ai/data/feature-development-guide.md) to apply all development standards, API design principles, error handling patterns, and provider integration guidelines. Ensure dependencies declared in the YAML frontmatter are readable before proceeding.

Follow the feature implementation workflow: API design → Service layer → Provider implementations → Handler → Tests. Ensure consistency with existing patterns and comprehensive test coverage.
</instructions>

## Implementation Steps

<implementation_steps>

### 1. API Specification

- Update `internal/api/oapi.yaml` with new endpoints
- Define request/response models
- Specify error responses
- Run `make generate`

### 2. Create Service Interface

- Define provider interface in `internal/services/{feature}/{feature}_provider.go`
- Create service layer in `internal/services/{feature}/{feature}_service.go`
- Implement provider lookup and routing logic
- Add caching if needed (expensive operations)

### 3. Implement Providers

- Implement for each Git provider (GitHub, GitLab, Bitbucket)
- Create provider files: `internal/services/{github,gitlab,bitbucket}/{provider}.go`
- Map provider-specific models to GitFusion models
- Handle provider-specific errors
- Add comprehensive error context

### 4. Create API Handler

- Implement handler in `internal/api/{feature}_handler.go`
- Validate request parameters
- Call service layer methods
- Map errors to HTTP responses
- Register handler in `internal/api/server.go`

### 5. Write Tests

- Create test file: `internal/services/{provider}/{provider}_test.go`
- Use table-driven tests
- Mock external dependencies
- Test success and error scenarios
- Verify error handling

### 6. Documentation

- Document all exported types and functions
- Update CHANGELOG.md
- Add examples for complex features
</implementation_steps>

## Output Format

<output_format>

### Implementation Summary

Brief description of what was implemented and how it fits into the system.

### Files Created/Modified

List of all files changed with brief description of changes.

### Testing Approach

Description of test coverage and scenarios tested.

### Next Steps

Any follow-up work or improvements needed.
</output_format>

## Quality Checklist

<quality_checklist>

- OpenAPI spec updated and code regenerated
- Provider interface defined with clear contract
- All providers implemented (GitHub, GitLab, Bitbucket)
- Error handling with proper context wrapping
- Caching added for expensive operations
- API handler validates inputs
- Consistent error responses
- Table-driven tests written
- All tests passing
- Code formatted with gofmt
- Exported functions documented
- CHANGELOG.md updated
</quality_checklist>
