---
applyTo: "**/api/**/*.go"
---
# API Development Guidelines for GitFusion

## API Design Principles
- Follow RESTful principles (consistent resource naming, proper HTTP methods)
- Support pagination, filtering, and sorting with consistent patterns
- Version APIs appropriately and maintain backward compatibility
- Use standard error formats and status codes

## Request/Response Structure
- Use consistent JSON response formats
- Include appropriate HTTP status codes (200, 201, 400, 401, 403, 404, 500)
- Provide meaningful error messages with error codes
- Follow JSON naming conventions (camelCase)

## OpenAPI Specification
- All API changes must be reflected in the OpenAPI specification
- Generate server stubs from the OpenAPI spec
- Keep the spec and implementation in sync

## Security
- Validate all input data
- Protect against common API vulnerabilities (injection, XSS, etc.)
- Ensure all external communications are secured via HTTPS
- Apply proper CORS policies

## Testing
- Write comprehensive tests for all endpoints
- Include positive and negative test cases
