# Implementation roadmap: git-brx v1.3.0

## Target milestone & scope

- **Milestone:** `v1.3.0` — Workflow hardening, parent base tracking, and collaboration safety
- **Epic:** `#91` (integration branch `epic/91`, merged to `main` after all slices complete and this file is removed)
- **Non-goals:** No new subcommands (`abort`, `status` deferred to keep the CLI surface minimal); no Bitbucket/GitLab provider implementation beyond existing `SCMProvider` interface contracts.

## Ordered slices

| Slice | Proposed issue title | Depends on | Spec refs | Verification milestone | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `S1` | `fix(conflict): add safe abort in reset and headless marker resolution` | `—` | `global.md` §4; `reset.md` §1, §3–§6 (`TC-RST-01`–`04`); `resolve.md` §1, §3–§6 (`TC-RSV-01`–`03`) | Safe two-step `reset` & headless/marker-aware `resolve` pass unit + integration tests; `go test -race ./...` green | Done (#92) |
| `S2` | `feat(branch): track parent base and add create and select guardrails` | `—` | `global.md` §4–§5, §6.8; `create.md` §1, §3–§6 (`TC-CRT-01`–`04`); `select.md` §1, §3, §6 (`TC-SEL-01`–`04`); `history.md` §1, §3, §6 (`TC-HIS-01`–`02`) | `branch.<name>.brx-base` persistence, `create` auto-switch, `select` `--ff-only`, and `history --topic` pass unit + integration tests | Done (#94) |
| `S3` | `fix(collab): harden sync, update, and publish for shared and worktree flows` | `S2` | `global.md` §4, §6.8–§6.9; `sync.md` §3, §6 (`TC-SYN-01`–`04`); `update.md` §3, §6 (`TC-UPD-01`–`03`); `publish.md` §1–§3, §5–§6 (`TC-PUB-01`–`02`) | Recorded-base & worktree-safe `sync`, `--rebase-merges` `update`, and FF-only `epic/*` `publish` pass unit + integration tests | Done (#96) |
| `S4` | `feat(review): add remote checks, draft mode, and reviewer fixes` | `S2`, `S3` | `global.md` §4–§5, §6.8–§6.9; `review.md` §1, §3–§6 (`TC-REV-01`–`07`) | Remote SHA/target checks, multi-`-r`, `--draft`, `409` URL, and `github.md` snippet fix pass unit + integration tests | Done (#98) |
| `S5` | `feat(delete): support parent return, worktree safety, and --delete-remote` | `S2`, `S3` | `global.md` §4–§5, §6.8–§6.9; `delete.md` §1, §3, §5–§6 (`TC-DEL-01`–`04`) | Multi-PR priority, parent-base return, linked-worktree detach, and `--delete-remote` pass unit + integration tests | Pending |

## Slice scope notes

- **`S1` — Conflict safety & headless resolution:** Owns `internal/cli/reset.go`, `internal/cli/resolve.go`, conflict-marker scanning in `internal/git/`, and matching unit/integration tests + `skills/git-brx/SKILL.md` / `docs/user-guide/` conflict notes. Decouples `reset` abort-only behavior from hard reset without adding flags to `reset`.
- **`S2` — Parent base tracking & branch ergonomics:** Owns `branch.<name>.brx-base` plumbing in `internal/git/inspection.go` & `operations.go`, `domain.BranchRecord.BaseBranch`, `internal/cli/create.go`, `internal/cli/select.go`, and `internal/cli/history.go`. Establishes shared `ResolveBaseBranch` helper for `S3`–`S5`.
- **`S3` — Collaboration & worktree hardening:** Owns `git worktree list --porcelain` inspection in `internal/git/`, `internal/cli/sync.go`, `internal/cli/update.go`, and `internal/cli/publish.go`. Reuses `S2` parent base resolution and enforces FF-only pushes on `epic/*` and `release/*`.
- **`S4` — Review preflight, draft mode & reviewer routing:** Owns `domain.PullRequestRequest.Draft`, `internal/github/client.go` (`CreatePullRequest` draft payload & reviewer warning), `internal/cli/review.go`, and `docs/user-guide/platforms/github.md` reference snippet (`review.merge.strategies.targets` alignment).
- **`S5` — Safe deletion & multi-PR resolution:** Owns `domain.PullRequestDetail.Number` & `OPEN > MERGED > CLOSED` selection in `internal/github/client.go`, `internal/cli/delete.go` (`--delete-remote`, parent-base return, linked-worktree detach fallback), and `docs/user-guide/` deletion notes.

## Spec-coverage matrix

| Spec document | Sections & verification scenarios | Owner slices |
| :--- | :--- | :--- |
| `docs/spec/global.md` | §4 (`Conflict Marker Scanning`) | `S1` |
| `docs/spec/global.md` | §4 (`Parent Base Metadata`), §5 (`BranchRecord.BaseBranch`), §6.8 | `S2` |
| `docs/spec/global.md` | §4 (`Linked Worktree Detection`), §6.8, §6.9 | `S3`, `S5` |
| `docs/spec/global.md` | §4 (`Remote HEAD SHA Verification`), §5 (`PullRequestRequest.Draft`), §6.8, §6.9 | `S4` |
| `docs/spec/global.md` | §5 (`PullRequestDetail` multi-PR priority) | `S5` |
| `docs/spec/reset.md` | §1, §3–§6 (`TC-RST-01`, `TC-RST-02`, `TC-RST-03`, `TC-RST-04`) | `S1` |
| `docs/spec/resolve.md` | §1, §3–§6 (`TC-RSV-01`, `TC-RSV-02`, `TC-RSV-03`) | `S1` |
| `docs/spec/create.md` | §1, §3–§6 (`TC-CRT-01`, `TC-CRT-02`, `TC-CRT-03`, `TC-CRT-04`) | `S2` |
| `docs/spec/select.md` | §1, §3, §6 (`TC-SEL-01`, `TC-SEL-02`, `TC-SEL-03`, `TC-SEL-04`) | `S2` |
| `docs/spec/history.md` | §1, §3, §6 (`TC-HIS-01`, `TC-HIS-02`) | `S2` |
| `docs/spec/sync.md` | §3, §6 (`TC-SYN-01`, `TC-SYN-02`, `TC-SYN-03`, `TC-SYN-04`) | `S3` |
| `docs/spec/update.md` | §3, §6 (`TC-UPD-01`, `TC-UPD-02`, `TC-UPD-03`) | `S3` |
| `docs/spec/publish.md` | §1–§3, §5–§6 (`TC-PUB-01`, `TC-PUB-02`) | `S3` |
| `docs/spec/review.md` | §1, §3–§6 (`TC-REV-01`, `TC-REV-02`, `TC-REV-03`, `TC-REV-04`, `TC-REV-05`, `TC-REV-06`, `TC-REV-07`) | `S4` |
| `docs/spec/delete.md` | §1, §3, §5–§6 (`TC-DEL-01`, `TC-DEL-02`, `TC-DEL-03`, `TC-DEL-04`) | `S5` |
