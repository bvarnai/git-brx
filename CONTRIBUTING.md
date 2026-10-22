# Contributing to git-brx

Bug reports, documentation improvements, and pull requests are welcome. Before opening a PR, review the architectural rules and test workflow below.

---

## 🎯 Architecture Rules

1. **Spec-Driven Parity:** Subcommands, flags, and exit codes must match the specifications in [`docs/spec/`](docs/spec/global.md). Update the corresponding spec file whenever CLI behavior changes.
2. **Plumbing over Porcelain:** Invoke Git through deterministic plumbing commands (`rev-parse`, `symbolic-ref`, `show-ref`, `for-each-ref`, `diff-index`, `merge-base`, `rev-list`) rather than porcelain output parsers.
3. **Stream Separation:**
   - `stdout` is reserved for primary command output, machine-readable data (`name`, `config`), and `--help`/`--version`.
   - `stderr` is used for progress logs (`[git-brx]`), warnings, hints, and errors.
4. **Minimal Dependencies:** Stick to the Go standard library (`os`, `strings`, `bufio`, `regexp`, `context`, `net/http`) and approved CLI packages (`github.com/spf13/cobra`, `gopkg.in/yaml.v3`). Do not import heavyweight Git libraries (`go-git`) or shell out to external script runtimes.

---

## 🛠️ Development Setup

### Prerequisites
- [Go](https://golang.org/) >= 1.22
- [Git](https://git-scm.com/) >= 2.20

### Clone and Build
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

## 🧪 Running Tests

1. **Unit & Integration Tests:**
   ```bash
   go test -v -race ./...
   ```

2. **Formatting & Static Analysis:**
   ```bash
   # Format code with standard gofmt
   gofmt -s -w .

   # Verify no unformatted files remain
   test -z "$(gofmt -l .)"

   # Run go vet
   go vet ./...
   ```

3. **Isolated Integration Tests:**
   Integration tests in `test/integration/` create disposable repositories in `t.TempDir()`. Tests must never read or modify the host machine's `~/.gitconfig` or global Git state.

4. **Contract Schema & Compatibility Matrix Tests:**
   `git-brx` validates API payloads against REST schemas using `github.com/santhosh-tekuri/jsonschema/v5`:
   - **Schema Sources:** Jira schemas are extracted from official Atlassian WADL descriptors; Bitbucket schemas come from official REST documentation.
   - **Run Compatibility Tests:**
     ```bash
     go test -v ./test/integration/... -run "CompatibilityMatrix|Rejection"
     ```
   - **Add a Server Version:**
     1. Create `test/schemas/<product>/<version>` (e.g., `test/schemas/jira/v2-server-9.x`).
     2. Add the schema files (`issue.json`, `error_collection.json`, `pull_request.json`, `errors.json`).
     3. Append the version string to the target slice in `test/integration/wiremock_schema_test.go`.
     4. Run `go test -v ./test/integration/... -run "CompatibilityMatrix"`.

---

## 🌿 Branch & PR Workflow

Use `git-brx` itself to manage topic branches and pull requests:

1. **Create an Issue:** Open a GitHub issue for the bug fix or feature.
2. **Create a Topic Branch:**
   ```bash
   git brx create issue/<number> -y
   ```
3. **Sync with `main`:**
   ```bash
   git brx sync
   ```
4. **Publish & Open a PR:**
   ```bash
   git brx publish
   git brx review
   ```
5. **Clean Up After Merge:**
   ```bash
   git brx delete issue/<number>
   git brx update
   ```

---

## 📝 Commit Conventions

Use [Conventional Commits](https://www.conventionalcommits.org/):
- `feat(#<issue>): ...` for new features
- `fix(#<issue>): ...` for bug fixes
- `docs(#<issue>): ...` for documentation changes
- `refactor(#<issue>): ...` for refactoring without behavior changes
- `test(#<issue>): ...` for test additions or updates
- `ci: ...` for GitHub Actions workflow changes
- `chore(#<issue>): ...` for maintenance or dependency updates
