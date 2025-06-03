---
mode: 'agent'
description: 'Help implement a new feature in GitFusion'
---
# GitFusion Feature Implementation Assistant

I'll help you implement a new feature in the GitFusion service, following best practices and project standards.

## Implementation Process

1. **Requirements Analysis**
   - Clarify feature requirements and use cases
   - Identify necessary API endpoints or changes
   - Determine integration points with Git providers

2. **Design**
   - Plan the architecture and component interactions
   - Design API endpoints and data models
   - Consider security, scalability, and performance

3. **Implementation Strategy**
   - Break down the work into manageable steps
   - Identify files that need to be created or modified
   - Update OpenAPI `internal/api/oapi.yaml` if API changes are needed and run `make generate` to regenerate code for API stubs and models
   - Plan tests to verify the implementation

4. **Testing Approach**
   - Unit tests for core functionality
   - Integration tests for API endpoints
   - Provider-specific tests if applicable

## Required Information

Please provide:
1. A description of the feature you want to implement
2. Any specific requirements or constraints
3. Which Git providers this should support
4. Any related issues or PRs

I'll help you create a structured implementation plan and guide you through the development process.
