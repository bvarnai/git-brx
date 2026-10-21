# Contributing to git-brx

Thank you for your interest in contributing to `git-brx`! We welcome bug reports, documentation improvements, and pull requests that adhere to our architectural standards and workflow rules.

---

## 🎯 Guiding Principles

Before proposing new features or modifying existing code, please review our core architectural rules:

1. **Spec-Driven Parity:** All subcommands, flags, and error codes must strictly adhere to the technical specifications in [`docs/spec/`](docs/spec/global.md). Any change in CLI behavior must be accompanied by updates to the corresponding specification files.
2. **Deterministic Plumbing over Porcelain:** External `git` invocations must use deterministic plumbing commands (`rev-parse`, `symbolic-ref`, `show-ref`, `for-each-ref`, `diff-index`, `merge-base`, `rev-list`) rather than porcelain commands.
3. **Stream Separation:**
   - `stdout` is strictly reserved for primary command results, machine-parseable data (`name`, `config`), and `--help`/`--version`.
   - `stderr` is used exclusively for operational progress logs (`[git-brx]`), warnings, hints, and error diagnostics.
4. **Zero Heavyweight Dependencies:** We use the Go standard library (`os`, `strings`, `bufio`, `regexp`, `context`) and standard CLI dependencies (`github.com/spf13/cobra`, `gopkg.in/yaml.v3`). We never import heavyweight Git libraries (such as `go-git`) or shell out to legacy external script utilities.

---

## 🛠️ Development Setup

### Prerequisites
- [Go](https://golang.org/) >= 1.22
- [Git](https://git-scm.com/) >= 2.20

### Clone and Build
```bash
git clone https://github.com/bvarnai/git-brx.git
cd git-brx

# Build local development binary to bin/
mkdir -p bin
go build -o ./bin/git-brx ./cmd/git-brx

# Verify the build
./bin/git-brx --version
```

---

## 🧪 Testing Guidelines

We enforce high test coverage and strict architectural boundaries:

1. **Run Unit & Integration Tests:**
   ```bash
   go test -v -race ./...
   ```

2. **Verify Code Formatting & Static Analysis:**
   ```bash
   # Code formatting (all files must be formatted with standard gofmt)
   gofmt -s -w .

   # Verify zero unformatted files exist
   test -z "$(gofmt -l .)"

   # Go static analysis
   go vet ./...
   ```

3. **Disposable Integration Tests:**
   All commands include disposable integration tests in `test/integration/` using `t.TempDir()`. Tests must never touch the host machine's `~/.gitconfig` or global Git state.

4. **Contract Schema & Compatibility Matrix Testing:**
   `git-brx` validates API payloads against official REST schemas using a schema-first validation engine (`github.com/santhosh-tekuri/jsonschema/v5`):
   - **Provenance Sources:** Jira schemas are extracted directly from official Atlassian WADL descriptors; Bitbucket schemas are derived from official HTML references.
   - **Running Compatibility Tests:**
     ```bash
     go test -v ./test/integration/... -run "CompatibilityMatrix|Rejection"
     ```
   - **Adding a New Server Version:**
     1. Create the version directory: `test/schemas/<product>/<version>` (e.g. `test/schemas/jira/v2-server-9.x`).
     2. Add the corresponding schema files (`issue.json`, `error_collection.json`, `pull_request.json`, `errors.json`).
     3. Append the version string to the target slice in `test/integration/wiremock_schema_test.go`.
     4. Execute `go test -v ./test/integration/... -run "CompatibilityMatrix"` to verify zero contract drift.

---

## 🌿 Git Branch & PR Workflow

We follow the opinionated branch workflow managed by `git-brx`:

1. **Create an Issue:** Every bug fix or feature must have a corresponding GitHub issue.
2. **Create Branch:**
   ```bash
   git brx create issue/<number> -y
   ```
3. **Synchronize & Rebase:**
   ```bash
   git brx sync
   ```
4. **Publish & Review:**
   ```bash
   git brx publish
   git brx review
   ```
5. **Clean Up After Merge:**
   ```bash
   git brx select main
   git brx update
   git brx delete issue/<number>
   ```

---

## 📝 Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat(scope): ...` for new features or capabilities
- `fix(scope): ...` for bug fixes
- `docs(scope): ...` for documentation updates
- `refactor(scope): ...` for code refactoring without behavior change
- `test(scope): ...` for adding or improving tests
- `ci: ...` for GitHub Actions workflow changes
- `chore: ...` for maintenance or dependency updates

Thank you for helping make `git-brx` better!
