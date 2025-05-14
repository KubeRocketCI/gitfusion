---
mode: 'agent'
description: 'Generate a pull request title and description based on your changes'
---
# PR Title and Description Generator

Generate a professional pull request title and description for my changes based on the project's PR guidelines.

Use the PR style defined in the [PR generation guidelines](../instructions/pr-generation.instructions.md).

## Instructions:
1. Analyze the git changes in the repository to understand what has been modified.
2. Generate a concise, descriptive PR title following the conventional commit format.
3. Create a comprehensive PR description with all required sections.
4. Ensure the description explains the problem being solved, the approach taken, lists key changes, and describes testing performed.

If there are no changes detected, please ask me to describe the changes I've made so you can generate an appropriate PR title and description.

Here's what I'm trying to accomplish with this PR:
${input:description:Please briefly describe the purpose of your changes}
