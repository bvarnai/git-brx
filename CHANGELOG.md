# Changelog

All notable changes to `git-brx` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.3.0] - 2026-11-25

### Added
- Parent base branch tracking (`branch.<name>.brx-base`) on `create` (with `--base`, fork/dirty-worktree warnings, and collision auto-switch) resolved automatically across `sync`, `review`, `history --delta`, and `delete` (`#94`, `#95`).
- Automatic fast-forward of clean behind-origin branches on `select` with `--no-ff` opt-out (`#94`, `#95`).
- Linked-worktree safety across `sync`, `review`, `history --delta`, and `delete` using `origin/<base>` and detached-`HEAD` fallback (`#96`, `#97`, `#100`, `#101`).
- Pull request `--draft` mode (skipping automated reviewer assignment unless `-r` is passed), repeatable/comma-separated `-r`/`--reviewer`, and remote sync/target validation in `review` (`#98`, `#99`).
- `delete --delete-remote` to remove merged remote branches while blocking open PRs and prioritizing `OPEN` > `MERGED` > `CLOSED` across reused branch names (`#100`, `#101`).
- Automatic aggregation of merged child closing references (`Closes #<N>`) on `epic/*` pull requests in `review`, with a non-fatal warning when exceeding GitHub's 10-issue auto-close limit (`#105`, `#106`).
- `epic(<scope>):` PR title prefix inference and descriptive merge commit instructions (`Merge branch '<branch>' into <target>: <summary>`) for `epic/*` pull requests (`#103`, `#104`).
- Antigravity plugin packaging (`plugin.json`, `skills/git-brx/SKILL.md`) for AI coding agent integration (`#83`, `#84`, `#85`, `#86`).
- Canonical `internal/term`, `internal/errcode`, and `internal/command` packages with thread-safe `Logger`, `ResolveColorPolicy` (`GIT_BRX_COLOR`, `NO_COLOR`, `TERM=dumb`), magenta `Debug` logging on `-v`/`--verbose`, mutual exclusivity between `--verbose` and `--quiet`, `SanitizedEnv`, and graceful `SIGTERM` cancellation (`#66`, `#70`, `#79`, `#80`, `#81`, `#82`).
- Shell completion specification `docs/spec/completion.md`, cross-platform CI matrix (`ubuntu-latest`, `macos-latest`, `windows-latest`, plus Go `1.22` minimum check), and stable tagless release archive aliases (`#65`, `#67`, `#69`, `#71`).

### Changed
- **Breaking:** Renamed `git-brx history --topic` to `--delta` (no compatibility alias) and replaced "topic branch" terminology across CLI diagnostics, specifications, and guides (`#109`, `#110`).
- Enforced fast-forward-only pushes on shared `epic/*` and `release/*` branches in `publish`, and preserved merge commits (`--rebase-merges`) on `feature/*` and `epic/*` in `update` (`#96`, `#97`).
- Defaulted child `issue/*` and `feature/*` pull requests targeting `epic/*` to **Squash and merge**, reserving merge-commit instructions for `epic/*` and `release/*` source branches (`#89`, `#90`).
- Updated `reset` to abort active merges/rebases while preserving unpushed local commits and reset to `origin/<branch>` only when idle (`#92`, `#93`).
- Updated `resolve` to detect cleared conflict markers before invoking `git mergetool` and added `--no-tool` for headless sessions (`#92`, `#93`).
- Aligned `git-brx --version` output format to `git-brx version <semver> (<commit> <os>/<arch>)` and mapped context cancellation to exit code `130` (`ExitInterrupted`) (`#66`, `#70`).
- Refined `README.md`, `CONTRIBUTING.md`, and user guides for `v1.3.0` behavior, optional `release/*` guidance, and GitHub-first examples (`#68`, `#75`, `#77`, `#87`, `#107`).

### Removed
- **Breaking:** Removed the unused `--dry-run` (`-n`) global flag across CLI, specifications, and documentation (`#73`, `#74`).
- Removed legacy `internal/ui` package, legacy source/docs, and unused repository PR template (`#63`, `#64`, `#66`).

### Fixed
- Queried `origin` directly in `update` so branches published from another clone or worktree are discovered cleanly (`#96`, `#97`).
- Hardened reviewer author exclusion and surfaced existing PR URLs on HTTP 409 conflicts in `review` (`#98`, `#99`).

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

[Unreleased]: https://github.com/bvarnai/git-brx/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/bvarnai/git-brx/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/bvarnai/git-brx/compare/v1.0.3...v1.2.0
[1.0.3]: https://github.com/bvarnai/git-brx/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/bvarnai/git-brx/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/bvarnai/git-brx/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/bvarnai/git-brx/releases/tag/v1.0.0
