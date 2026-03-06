# CHANGELOG Convention (Designer)

> Guidelines for maintaining the prototype CHANGELOG.

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
| New | New component or page prototype created |
| Update | Existing component modified (layout, interaction, mock data) |
| Delete | Component removed |
| Refactor | Component restructured without changing UI behavior |

## Best Practices

- Keep summaries under 80 characters
- Reference component names changed (e.g., "Updated LoginForm validation UX")
- Note mock data changes that affect Frontend replacement work
- Flag any deviations from the PRD with brief rationale
