# Contributing to git-brx

Bug reports, documentation improvements, and pull requests are welcome. Before opening a PR, review the architectural rules and test workflow below.

---

## 🎯 Architecture rules

1. **Spec-driven parity:** Subcommands, flags, and exit codes must match the specifications in [`docs/spec/`](docs/spec/global.md). Update the corresponding spec file whenever CLI behavior changes.
2. **Plumbing over porcelain:** Invoke Git through deterministic plumbing commands (`rev-parse`, `symbolic-ref`, `show-ref`, `for-each-ref`, `diff-index`, `merge-base`, `rev-list`) rather than porcelain output parsers.
3. **Stream separation:**
   - `stdout` is reserved for primary command output, machine-readable data (`name`, `config`), and `--help`/`--version`.
   - `stderr` is used for progress logs (`[git-brx]`), warnings, hints, and errors.
4. **Minimal dependencies:** Stick to the Go standard library (`os`, `strings`, `bufio`, `regexp`, `context`, `net/http`) and approved CLI/test packages (`github.com/spf13/cobra`, `github.com/stretchr/testify`, `gopkg.in/yaml.v3`). Do not import heavyweight Git libraries (`go-git`) or shell out to external script runtimes.

---

## 🛠️ Development setup

### Prerequisites
- [Go](https://golang.org/) >= 1.22
- [Git](https://git-scm.com/) >= 2.20

### Clone and build
```bash
git clone https://github.com/bvarnai/git-brx.git
cd git-brx

# Build local binary into ./bin/
mkdir -p bin
go build -o ./bin/git-brx ./cmd/git-brx

# Verify the build
./bin/git-brx --version
```

---

## 🧪 Running tests

1. **Unit & integration tests:**
   ```bash
   go test -v -race ./...
   ```

2. **Formatting & static analysis:**
   ```bash
   # Format code with standard gofmt
   gofmt -s -w .

   # Verify no unformatted files remain
   test -z "$(gofmt -l .)"

   # Run go vet
   go vet ./...
   ```

3. **Isolated integration tests:**
   Integration tests in `test/integration/` create disposable repositories in `t.TempDir()`. Tests must never read or modify the host machine's `~/.gitconfig` or global Git state.

4. **Contract schema & compatibility matrix tests:**
   `git-brx` validates API payloads against REST schemas using `github.com/santhosh-tekuri/jsonschema/v5`:
   - **Schema sources:** Jira schemas are extracted from official Atlassian WADL descriptors; Bitbucket schemas come from official REST documentation.
   - **Run compatibility tests:**
     ```bash
     go test -v ./test/integration/... -run "CompatibilityMatrix|Rejection"
     ```
   - **Add a server version:**
     1. Create `test/schemas/<product>/<version>` (e.g., `test/schemas/jira/v2-server-9.x`).
     2. Add the schema files (`issue.json`, `error_collection.json`, `pull_request.json`, `errors.json`).
     3. Append the version string to the target slice in `test/integration/wiremock_schema_test.go`.
     4. Run `go test -v ./test/integration/... -run "CompatibilityMatrix"`.

---

## 🌿 Branch & PR workflow

Use `git-brx` itself to manage work branches and pull requests:

1. **Create an issue:** Open a GitHub issue for the bug fix or feature.
2. **Create a work branch:**
   ```bash
   git brx create issue/<number> -y
   ```
3. **Sync with `main`:**
   ```bash
   git brx sync
   ```
4. **Publish & open a PR:**
   ```bash
   git brx publish
   git brx review
   ```
5. **Clean up after merge:**
   ```bash
   git brx delete issue/<number>
   git brx update
   ```

---

## 📝 Commit conventions

Use [Conventional Commits](https://www.conventionalcommits.org/) in the format `<type>(<component>): <description>` (lowercase imperative, <= 72 characters, no trailing period, and no issue numbers in commit subjects—issue references belong in the pull request):
- `feat(<component>): ...` for new features (`!` suffix + `BREAKING CHANGE:` footer for breaking changes)
- `fix(<component>): ...` for bug fixes
- `docs(<component>): ...` for documentation changes
- `refactor(<component>): ...` for refactoring without behavior changes
- `test(<component>): ...` for test additions or updates
- `ci(<component>): ...` for GitHub Actions workflow changes
- `chore(<component>): ...` for maintenance or dependency updates
