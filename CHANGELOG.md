<a name="unreleased"></a>
## [Unreleased]


<a name="v0.6.0"></a>
## v0.6.0 - 2026-06-28
### Features

- Add GitLab pipeline jobs and job trace endpoints
- implement pipeline listing API across multiple git providers
- extend PullRequest model with description, draft, and commit_sha fields
- add pull/merge request listing API across GitHub, GitLab, and Bitbucket
- Enhance GithubProvider ListUserOrganizations to include current user ([#35](https://github.com/KubeRocketCI/gitfusion/issues/35))
- Implement cache invalidation endpoint ([#33](https://github.com/KubeRocketCI/gitfusion/issues/33))
- Add /branches endpoint and services structure refactoring ([#27](https://github.com/KubeRocketCI/gitfusion/issues/27))
- Implement caching for repositories and organizations ([#21](https://github.com/KubeRocketCI/gitfusion/issues/21))
- Add endpoint to list organizations ([#19](https://github.com/KubeRocketCI/gitfusion/issues/19))
- Unify repository endpoints and add multi-provider support ([#15](https://github.com/KubeRocketCI/gitfusion/issues/15))
- Add user repositories for GitHub endpoint ([#13](https://github.com/KubeRocketCI/gitfusion/issues/13))
- Add repository name filter to repository listing endpoints ([#11](https://github.com/KubeRocketCI/gitfusion/issues/11))
- Add Bitbucket repository endpoints ([#9](https://github.com/KubeRocketCI/gitfusion/issues/9))
- Add GitLab repository endpoints ([#8](https://github.com/KubeRocketCI/gitfusion/issues/8))
- Adds Copilot instructions and PR generation guidelines
- GitHub repository endpoints

### Bug Fixes

- remove unused GitLab InsecureSkipVerify
- Return 400 for unsupported provider in pipeline dispatch
- Replace deprecated Bitbucket workspaces endpoint with direct HTTP call
- Add proper 404/401 error handling
- Use Path instead of Name for GitLab repository ([#50](https://github.com/KubeRocketCI/gitfusion/issues/50))
- Incomplete branch list for Bitbucket ([#41](https://github.com/KubeRocketCI/gitfusion/issues/41))

### Routine

- Add caching for pipeline jobs/traces and git-server settings
- Update current development version
- support Jira-prefixed CHANGELOG format
- remove PR title length validation
- Update current development version
- Align Chart.yaml ([#58](https://github.com/KubeRocketCI/gitfusion/issues/58))
- Disable regcred usage by default ([#58](https://github.com/KubeRocketCI/gitfusion/issues/58))
- Update current development version ([#58](https://github.com/KubeRocketCI/gitfusion/issues/58))
- validate commit title length and format
- Update current development version ([#54](https://github.com/KubeRocketCI/gitfusion/issues/54))
- Update KubeRocketAI ([#44](https://github.com/KubeRocketCI/gitfusion/issues/44))
- Setup KubeRocketAI ([#44](https://github.com/KubeRocketCI/gitfusion/issues/44))
- Update current development version ([#37](https://github.com/KubeRocketCI/gitfusion/issues/37))
- Enable CHANGELOG.md generation([#37](https://github.com/KubeRocketCI/gitfusion/issues/37))
- Add multi-arch build support ([#31](https://github.com/KubeRocketCI/gitfusion/issues/31))
- Bump CodeQL version ([#26](https://github.com/KubeRocketCI/gitfusion/issues/26))
- Align github templates ([#26](https://github.com/KubeRocketCI/gitfusion/issues/26))
- Align github templates ([#26](https://github.com/KubeRocketCI/gitfusion/issues/26))
- Align README file to reflect proper changes ([#17](https://github.com/KubeRocketCI/gitfusion/issues/17))
- Update .gitignore, add VSCode settings
- Update repository community artifacts

### Documentation

- add CLAUDE.md with repository guidance
- Improve GitHub Copilot configuration ([#17](https://github.com/KubeRocketCI/gitfusion/issues/17))
- Add GitHub Copilot configuration and assistance tools ([#17](https://github.com/KubeRocketCI/gitfusion/issues/17))


[Unreleased]: https://github.com/KubeRocketCI/gitfusion/compare/v0.6.0...HEAD
