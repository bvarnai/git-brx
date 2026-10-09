# Contributing to git-brx

Thank you for your interest in contributing to `git-brx`! We welcome bug reports, documentation improvements, and pull requests that align with the project's core philosophy.

---

## 🎯 Guiding Principles

Before proposing new features, please review our [Core Philosophy](docs/user-guide/philosophy.md):
1. **Opinionated and Simple:** We favor guardrails and sensible defaults over endless configuration options.
2. **Issue-First Development:** Every non-trivial change should be linked to an issue tracker task.
3. **Zero Runtime Dependencies:** `git-brx` compiles into a single, static Go binary that relies only on the system `git` executable. We do not import heavy Git C-bindings or bloated third-party frameworks.

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

# Test the local binary
./bin/git-brx --version
```

---

## 🧪 Testing Guidelines

We enforce high test coverage and strict architectural boundaries:

1. **Run Unit & Integration Tests:**
   ```bash
   go test -v -race ./...
   ```
2. **Check Code Formatting & Go Vet:**
   ```bash
   # Code formatting
   gofmt -s -w .
   
   # Go static analysis
   go vet ./...
   ```
3. **Integration Tests:**
   All commands include disposable integration tests in `test/integration/` using `t.TempDir()`. Tests must never touch the host machine's `~/.gitconfig` or global Git state.

---

## 📝 Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat(scope): ...` for new features or capabilities
- `fix(scope): ...` for bug fixes
- `docs(scope): ...` for documentation updates
- `refactor(scope): ...` for code refactoring without behavior change
- `test(scope): ...` for adding or improving tests
- `ci: ...` for GitHub Actions workflow changes

---

## 🚀 Submitting a Pull Request

1. Fork the repository and create a topic branch from `main`.
2. Ensure all tests pass (`go test -v -race ./...`) and `gofmt` produces no diffs.
3. Keep commits atomic and clearly described.
4. Open a Pull Request referencing any related issues.

Thank you for helping make `git-brx` better!
