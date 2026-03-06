# CHANGELOG Convention (Frontend)

> Guidelines for maintaining the production code CHANGELOG.

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
| New | New feature implementation (API integration, business logic) |
| Update | Existing implementation modified |
| Delete | Feature or code removed |
| Refactor | Code restructured without changing behavior |

## Best Practices

- Keep summaries under 80 characters
- Reference API endpoints integrated (e.g., "Integrated POST /api/auth/login")
- Note any deviations from the prototype with rationale
- List remaining technical debt or known issues
- Include test coverage summary when relevant
