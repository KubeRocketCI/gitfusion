---
applyTo: "**/github/*.go,**/gitlab/*.go,**/bitbucket/*.go"
---
# Git Provider Integration Guidelines for GitFusion

## General Principles
- Maintain consistent interfaces across all Git providers
- Abstract provider-specific details behind common interfaces
- Implement proper error handling and recovery
- Cache results when appropriate to minimize API calls

## API Client Implementation
- Use appropriate client libraries when available
- Implement proper retry logic with exponential backoff
- Add logging for API requests (with sensitive data redacted)
- Include comprehensive error context in returned errors

## Data Mapping
- Create clean mappings between provider-specific models and GitFusion models
- Handle differences in terminology and data structures consistently
- Document any provider-specific limitations or behaviors

## Testing
- Mock API responses for unit tests
- Include integration tests with API testing tokens when possible
- Test error scenarios and rate limiting behavior
- Verify data mapping for complex objects

## Documentation
- Document provider-specific configuration options
- Note any limitations or differences in behavior between providers
- Include setup instructions for each provider
