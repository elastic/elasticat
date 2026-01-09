## Description

<!-- Describe your changes here -->

## PR Title Format

**Your PR title must follow conventional commit format:**

```
<type>: <description>
```

**Types:**
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation only
- `refactor:` - Code refactoring
- `test:` - Adding/updating tests
- `chore:` - Maintenance (deps, build, etc.)
- `perf:` - Performance improvement
- `ci:` - CI/CD changes

**Breaking changes:** Add `!` after the type (e.g., `feat!: breaking change`)

**Examples:**
- `feat: add metrics dashboard`
- `fix: resolve panic on empty input`
- `docs: update installation guide`
- `feat!: change default port`

## Checklist

- [ ] PR title follows conventional commit format
- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
