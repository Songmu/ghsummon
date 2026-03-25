# Changelog

## [v0.0.3](https://github.com/Songmu/ghsummon/compare/v0.0.2...v0.0.3) - 2026-03-25
- Add test case for new file detection in diff parser by @Songmu in https://github.com/Songmu/ghsummon/pull/26

## [v0.0.2](https://github.com/Songmu/ghsummon/compare/v0.0.1...v0.0.2) - 2026-03-21
- go fix ./... by @Songmu in https://github.com/Songmu/ghsummon/pull/22
- Remove user lookup fallback for Copilot agent ID by @Songmu in https://github.com/Songmu/ghsummon/pull/23
- Use current branch as PR base instead of repository default branch by @Songmu in https://github.com/Songmu/ghsummon/pull/24
- Fix token masking by @Songmu in https://github.com/Songmu/ghsummon/pull/25

## [v0.0.1](https://github.com/Songmu/ghsummon/commits/v0.0.1) - 2026-03-21
- Fix parseDiffOutput: larger scanner buffer and propagate scan errors by @Copilot in https://github.com/Songmu/ghsummon/pull/4
- Implement core logic for @copilot prompt detection and PR creation by @Songmu in https://github.com/Songmu/ghsummon/pull/2
- Add ghsummon and copilot-setup-steps workflows by @Songmu in https://github.com/Songmu/ghsummon/pull/5
- Fix Copilot assignee: use GraphQL API instead of REST by @Songmu in https://github.com/Songmu/ghsummon/pull/8
- add-mask encoded by @Songmu in https://github.com/Songmu/ghsummon/pull/9
- Add GraphQL-Features header for Copilot assignment by @Songmu in https://github.com/Songmu/ghsummon/pull/11
- Add fallback for Copilot agent ID lookup by @Songmu in https://github.com/Songmu/ghsummon/pull/13
- Switch from GitHub App token to PAT for Copilot assignment by @Songmu in https://github.com/Songmu/ghsummon/pull/15
- Update token docs: cite official GitHub docs by @Songmu in https://github.com/Songmu/ghsummon/pull/17
- Update action.yml description and branding by @Songmu in https://github.com/Songmu/ghsummon/pull/18
- Add CodeQL security scanning workflow by @Songmu in https://github.com/Songmu/ghsummon/pull/19
- Fix ghalint issues across all workflows by @Songmu in https://github.com/Songmu/ghsummon/pull/20
