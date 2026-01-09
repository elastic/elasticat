# Release Process

This document describes how to create releases for elasticat.

## PR Title Convention

All PR titles **must** follow [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>: <description>
```

**Types:**
- `feat:` - New features (appears in "Features" section)
- `fix:` - Bug fixes (appears in "Bug Fixes" section)
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Adding/updating tests
- `chore:` - Maintenance tasks
- `perf:` - Performance improvements
- `ci:` - CI/CD changes

**Breaking changes:** Add `!` after the type (e.g., `feat!: breaking API change`)

**Examples:**
```
feat: add catseye TUI binary
fix: resolve panic on empty log entries
feat!: change default keybinding for quit
chore: update dependencies
```

PR titles are validated by CI and must pass before merging. When PRs are squash-merged, the PR title becomes the commit message, which feeds into automated changelog generation.

## Prerequisites

- All changes committed to the main branch
- Tests passing (`make test`)
- Code formatted (`make fmt`)
- License headers present (`make license-add`)
- [git-cliff](https://git-cliff.org/) installed (`cargo install git-cliff` or `brew install git-cliff`)

## Creating a Release

### 1. Generate Changelog

Use git-cliff to auto-generate release notes from conventional commits:

```bash
make changelog VERSION=v0.0.6-alpha
```

This creates `release-notes/v0.0.6-alpha.md` with all changes since the last tag, grouped by type (Features, Bug Fixes, etc.).

### 2. Review and Edit Release Notes

Review the generated file and make any edits:
- Add context to important changes
- Highlight breaking changes
- Remove trivial entries if desired

### 3. Commit and Push

```bash
git add release-notes/v0.0.6-alpha.md
git commit -m "chore(release): prepare v0.0.6-alpha"
git push origin main
```

### 4. Run the Release

```bash
make release VERSION=v0.0.6-alpha
```

This command will:
1. Verify the release notes file exists
2. Run the test suite
3. Check code formatting
4. Verify license headers
5. Create an annotated git tag
6. Push the tag to GitHub

### 5. GitHub Actions

Once the tag is pushed, GitHub Actions automatically:
- Builds binaries for all supported platforms (Linux, macOS, Windows)
- Creates a GitHub Release with the binaries attached
- Uses the release notes from `release-notes/v<version>.md`

## Version Naming Convention

We use semantic versioning with optional pre-release identifiers:

- `v0.0.1-alpha` - Alpha releases (early development)
- `v0.1.0-beta` - Beta releases (feature complete, testing)
- `v1.0.0` - Stable releases

## Quick Reference

```bash
# Generate changelog from conventional commits
make changelog VERSION=v0.0.6-alpha

# Review/edit release-notes/v0.0.6-alpha.md

# Prepare code for release
make prep

# Create and push release
make release VERSION=v0.0.6-alpha
```
