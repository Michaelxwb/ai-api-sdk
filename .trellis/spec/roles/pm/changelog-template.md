# CHANGELOG Convention (PM)

> Guidelines for maintaining the requirements CHANGELOG.

---

## Format

Each `/trellis:handoff` appends one row to the CHANGELOG table:

```markdown
| Date | Author | Type | Summary | Files |
|------|--------|------|---------|-------|
| YYYY-MM-DD HH:mm | developer-name | Type | One sentence (max 80 chars) | key files |
```

## Type Classification

| Type | When to Use |
|------|-------------|
| New | New PRD or user story document created |
| Update | Existing requirement modified (scope change, clarification) |
| Delete | Requirement removed or deprecated |
| Refactor | Document restructured without changing requirements |

## Best Practices

- Keep summaries under 80 characters
- Reference specific sections changed (e.g., "Updated US-03 acceptance criteria")
- Note if the change affects downstream roles (Designer/Frontend)
- Include the reason for changes when non-obvious
