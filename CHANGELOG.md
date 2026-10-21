# Changelog

All notable changes to `git-brx` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Canonical `internal/term` package with thread-safe `Logger`, `ResolveColorPolicy` (`GIT_BRX_COLOR`, `NO_COLOR`, `TERM=dumb`), `Confirm`, `Spinner`, and `Timer` (`#66`).
- Domain-level diagnostic debug logging (`Debug` / `Debugf` in magenta `\x1b[35m`) across all CLI subcommands on `-v` / `--verbose` (`#66`).
- Subprocess environment sanitization (`BuildSanitizedEnv`) stripping conflicting environment variables, `WaitDelay = 3 * time.Second`, and graceful `SIGTERM` cancellation (`#66`).
- Mutual exclusivity enforcement between `--verbose` (`-v`) and `--quiet` (`-q`) with exit code `2` (`#66`).
- Shell completion specification `docs/spec/completion.md` and global specification harmonization (`#67`).
- Cross-platform CI matrix across `ubuntu-latest`, `macos-latest`, and `windows-latest` (`#65`).
- Stable tagless release archive aliases (`git-brx_<os>_<arch>.tar.gz` / `.zip`) and PR template (`#65`).

### Changed
- Migrated all commands and tests from `internal/ui` to `internal/term` and removed legacy `internal/ui` (`#66`).
- Aligned `git-brx --version` output format to `git-brx version <semver> (<commit> <os>/<arch>)` (`#66`).
- Mapped process context cancellation to standard exit code `130` (`ExitInterrupted`) (`#66`).

## [1.2.0] - 2026-10-15

### Added
- Layered 4-tier cascading configuration (`User Global -> $GIT_BRX_CONFIG -> Repo Local -> Remote Auto-Discovery`) and `config` subcommand (`#43`, `#45`, `#46`).
- Zero-config auto-detection of SCM provider and repository owner/name from `remote.origin.url` (`#28`, `#29`).
- Structured reviewer routing in `review` matching path globs, labels, branch patterns, and author exclusion (`#34`, `#35`).
- Branch-aware merge strategy resolution (`squash`, `merge`, `rebase`) and configurable merge instructions (`#30`, `#32`, `#33`).
- Automatic fast-forwarding of clean base branch during `sync` (`#24`, `#25`).
- Multi-branch semantic mapping and flexible enhancement routing in `branch.mapping` (`#26`, `#52`).
- Enhanced `update` and `sync` reporting with commit deltas and strategy status (`#54`, `#57`, `#58`).
- Designate `main` as primary default branch (`#60`, `#61`).
- Coworker and reviewer simulation test playbooks (`#22`, `#36`, `#49`).

### Changed
- Streamlined pull request description layout and eliminated redundant boilerplate (`#41`, `#42`).
- Removed legacy backward-compatibility code and JSON config tags (`#47`, `#48`).

### Fixed
- Prevent release binaries from being flagged as dirty by compiler VCS detection (`#20`, `#21`).
- Align squash merge instruction commit title with composed pull request title (`#39`, `#40`).
- Eliminate redundant PR URL output in `review` command (`#55`, `#56`).

## [1.0.3] - 2026-10-12

### Added
- Inject issue key as conventional commit scope in pull request titles (`#12`, `#13`).
- Automatic linking of GitHub issues in pull requests and published branches (`#8`, `#9`).
- Support optional branch argument in `delete` subcommand (`#5`).
- Fine-grained GitHub token setup and branch protection documentation (`#16`, `#17`).

### Fixed
- Harmonize CLI diagnostic messages, remove duplicate errors, and fix misleading hints (`#18`, `#19`).
- Align pull request title and merge instructions with GitHub default squash commit format (`#11`).

## [1.0.2] - 2026-10-09

### Fixed
- Handle `--help` gracefully on root command across all execution environments (`#2`, `#3`).

## [1.0.1] - 2026-10-09

### Added
- Contributing guide, GitHub issue templates, and AI disclosure notice.

### Fixed
- Prefix interactive confirmation prompts with `[git-brx]` and standardize error prefixes to `Error:` instead of `!`.
- Contextual branch creation prompts defaulting to `[Y/n]` and warn on closed issues.
- Fix Scoop manifest checksum download in release workflow.

## [1.0.0] - 2026-10-09

### Added
- Initial Go implementation of `git-brx` (`create`, `select`, `sync`, `update`, `publish`, `delete`, `review`, `reset`, `resolve`, `name`, `history`, `completion`).
- GitHub and Jira/Bitbucket issue tracker and SCM integration.
- Direct Git execution boundary (`internal/git`) with `LC_ALL=C` and non-interactive safeguards.

[Unreleased]: https://github.com/bvarnai/git-brx/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/bvarnai/git-brx/compare/v1.0.3...v1.2.0
[1.0.3]: https://github.com/bvarnai/git-brx/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/bvarnai/git-brx/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/bvarnai/git-brx/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/bvarnai/git-brx/releases/tag/v1.0.0
