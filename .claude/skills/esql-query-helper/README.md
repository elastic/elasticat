# ES|QL Query Helper Skill

This is a Claude Code skill that provides expert assistance with ES|QL (Elasticsearch Query Language) queries for the turbodevlog/telasticat project.

## What This Skill Does

- Generates ES|QL queries for traces, logs, and metrics
- Validates query syntax before execution
- Helps debug ES|QL errors
- Suggests performance optimizations
- Provides examples from the telasticat codebase

## How to Use

Once Claude Code is restarted, this skill will be automatically available. You can invoke it by asking questions like:

- "Write an ES|QL query to find slow transactions in the past hour"
- "How do I count spans per transaction name using ES|QL?"
- "Optimize this ES|QL query for better performance"
- "What's wrong with my ES|QL syntax?"

Claude will automatically use this skill when it detects ES|QL-related tasks.

## Files

- **SKILL.md** - Main skill definition with syntax reference and best practices
- **examples.md** - Real-world query examples from the telasticat project
- **README.md** - This file

## Integration with Telasticat

This skill references the actual ES|QL implementation in `internal/es/client.go`:
- `executeESQLQuery()` - Generic ES|QL query executor
- `GetTransactionNamesESQL()` - Real-world 3-query correlation example

## Sharing with Your Team

This skill is in the project directory (`.claude/skills/`), so anyone who clones the turbodevlog repo will automatically get it when they use Claude Code.

To share:
```bash
git add .claude/skills/esql-query-helper/
git commit -m "Add ES|QL query helper skill"
git push
```

## Making It Personal

To make this available across ALL your projects (not just turbodevlog):
```bash
cp -r /home/andrewvc/projects/turbodevlog/.claude/skills/esql-query-helper \
     ~/.claude/skills/
```

Then restart Claude Code.

## Allowed Tools

This skill has access to:
- Read - Read project files
- Bash(curl:*) - Execute ES|QL queries via curl
- Bash(./telasticat:*) - Run telasticat commands
- WebFetch(domain:www.elastic.co) - Fetch ES documentation
- Grep, Glob - Search for code examples

These match the permissions already configured in your project's settings.

## Next Steps

1. **Restart Claude Code** to load the skill
2. **Test it**: Ask "What ES|QL skills do I have?"
3. **Try it**: "Write an ES|QL query to find errors in traces from the past hour"

## License

Same as the turbodevlog project.
