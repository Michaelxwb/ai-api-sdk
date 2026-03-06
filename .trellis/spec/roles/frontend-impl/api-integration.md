# API Integration Guidelines

> Standards for replacing mock data with real API calls.

---

## Mock Replacement Process

### Step 1: Find All Mocks

Search for TODO markers left by the Designer:

```bash
grep -rn "TODO: replace with real API" deliverables/prototypes/
```

### Step 2: Replace Pattern

```tsx
// BEFORE (Designer's mock)
const handleLogin = async () => {
  // TODO: replace with real API call
  await new Promise((r) => setTimeout(r, 800));
  setUser(mockUser);
};

// AFTER (Frontend implementation)
const handleLogin = async () => {
  try {
    setLoading(true);
    const user = await authApi.login({ email, password });
    setUser(user);
  } catch (error) {
    setError(getErrorMessage(error));
  } finally {
    setLoading(false);
  }
};
```

### Step 3: Required States

Every API call must handle:

| State | Implementation |
|-------|---------------|
| Loading | Show spinner or skeleton |
| Success | Render data |
| Error | Show error message with retry option |
| Empty | Show empty state message |

## Error Handling

- Use the project's standard error handling pattern
- Map API error codes to user-friendly messages
- Log errors for debugging but don't expose internals to users
- Implement retry for transient failures (network errors, 5xx)

## Testing Requirements

- Unit tests for business logic (state management, data transforms)
- Integration tests for API call flows
- Mock API responses in tests (don't hit real endpoints)

## Handoff Checklist

Before running `/trellis:handoff`, verify:

- [ ] All `// TODO: replace with real API` markers resolved
- [ ] Every API call has loading, error, and empty states
- [ ] Error messages are user-friendly
- [ ] TypeScript compiles without errors
- [ ] Lint and format checks pass
- [ ] Core logic has unit tests
