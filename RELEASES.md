# Release Process

This document describes how to create releases for elasticat.

## Prerequisites

- All changes committed to the main branch
- Tests passing (`make test`)
- Code formatted (`make fmt`)
- License headers present (`make license-add`)

## Creating a Release

### 1. Create Release Notes

Create a release notes file in `release-notes/` following the naming convention `v<version>.md`:

```bash
# Example for version 0.0.5-alpha
touch release-notes/v0.0.5-alpha.md
```

Use the template in `release-notes/TEMPLATE.md` as a starting point:

```markdown
## What's New

- Feature description

## Bug Fixes

- Fix description

## Breaking Changes

- None
```

### 2. Commit and Push Your Changes

Ensure all changes including release notes are committed and pushed:

```bash
git add .
git commit -m "Prepare release v0.0.5-alpha"
git push origin main
```

**Important:** You must push your commits before creating the release. The `make release` command only pushes the tag, not the commits themselves.

### 3. Run the Release

Use the `make release` command with the version (including the `v` prefix):

```bash
make release VERSION=v0.0.5-alpha
```

This command will:
1. Verify the release notes file exists
2. Run the test suite
3. Check code formatting
4. Verify license headers
5. Create an annotated git tag
6. Push the tag to GitHub

### 4. GitHub Actions

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
# Prepare code for release
make prep

# Create and push a release
make release VERSION=v0.0.5-alpha
```
