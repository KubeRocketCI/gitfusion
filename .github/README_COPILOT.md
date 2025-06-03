# GitHub Copilot Configuration

This directory contains configuration files that customize GitHub Copilot's behavior when working with the GitFusion codebase.

## Configuration Files

### Main Instructions

- `.github/copilot-instructions.md`: Primary instructions file with project context and coding standards

### Specialized Instruction Files

- `.github/instructions/go-development.instructions.md`: Go-specific development guidelines
- `.github/instructions/api-development.instructions.md`: API development best practices
- `.github/instructions/git-provider-integration.instructions.md`: Guidelines for integrating with Git providers
- `.github/instructions/pr-generation.instructions.md`: Pull request title and description formatting

### Prompt Files (Task-Specific Assistants)

- `.github/prompts/create-github-pr.prompt.md`: Pull request creation assistant
- `.github/prompts/create-github-issue.prompt.md`: Issue creation assistant
- `.github/prompts/code-review.prompt.md`: Code review assistant
- `.github/prompts/implement-feature.prompt.md`: Feature implementation assistant
- `.github/prompts/documentation-helper.prompt.md`: Documentation generation assistant

## Usage

### Using Prompt Files

Trigger a prompt file in the chat interface by typing `/` followed by the prompt name:

```text
/create-github-pr
/code-review
/implement-feature
/documentation-helper
```

### VS Code Settings

The `.vscode/settings.json` file configures VS Code to use these instructions and prompt files with GitHub Copilot.
