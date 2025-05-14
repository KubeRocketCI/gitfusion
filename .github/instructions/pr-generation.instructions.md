---
applyTo: "**"
---
# Pull Request Title and Description Generation Guidelines

## PR Title Guidelines
- Keep titles concise (under 72 characters) but descriptive
- Start with a verb in the present tense (e.g., "Add", "Fix", "Update", "Remove")
- Include the component or area affected (e.g., "Add repository pagination to GitHub API")
- For bug fixes, include "fix:" prefix
- For features, include "feat:" prefix
- For documentation changes, include "docs:" prefix
- For refactoring, include "refactor:" prefix
- For performance improvements, include "perf:" prefix
- For tests, include "test:" prefix
- For chores, include "chore:" prefix

## PR Description Guidelines

### Structure
- Begin with a clear summary of the changes
- Use markdown formatting for better readability
- Include the following sections defined in [PR Template](../PULL_REQUEST_TEMPLATE.md)

### Language and Style
- Use clear, concise language
- Avoid technical jargon when possible
- Write in the present tense
- Use bullet points for lists
- Use code blocks for code snippets with language specification for syntax highlighting
- Link to relevant documentation when necessary

### PR Size Guidelines
- Focus on a single logical change
- Suggest breaking down PRs with more than 500 lines of changes
- Consider separating refactoring commits from functional changes

### Example PR Description Format

```markdown
This PR introduces a new feature that allows users to paginate through GitHub repositories.

Fixes #(333)

## Type of change

Please delete options that are not relevant.

- [ ] Bug fix (non-breaking change which fixes an issue)
- [x] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] This change requires a documentation update

## How Has This Been Tested?

Unit tests were added to cover the new pagination feature.

- [x] Test Pagination
- [x] Test B

## Checklist:

- [x] My code follows the style guidelines of this project
- [x] I have performed a self-review of my own code
- [x] I have commented my code, particularly in hard-to-understand areas
- [x] I have made corresponding changes to the documentation
- [x] My changes generate no new warnings
- [x] I have added tests that prove my fix is effective or that my feature works
- [x] New and existing unit tests pass locally with my changes
- [x] Any dependent changes have been merged and published in downstream modules
- [x] I have squashed my commits
```
