# Prototype Guidelines

> Standards for creating interactive prototypes with mock data.

---

## Prototype Standards

### Must Be Runnable

Prototypes are NOT static mockups. They must:
- Render without errors in the browser
- Include all user interactions (click, input, navigation)
- Use inline mock data that simulates real API responses

### Mock Data Convention

```tsx
// CORRECT: Inline mock with clear TODO marker
const mockUser = {
  id: "1",
  email: "test@example.com",
  name: "Test User",
};

const handleLogin = async () => {
  // TODO: replace with real API call
  await new Promise((r) => setTimeout(r, 800));
  setUser(mockUser);
};
```

```tsx
// WRONG: Separate mock service (makes it harder for Frontend to find and replace)
import { mockLoginService } from "../mocks/auth";
```

**Rules**:
- All mock data inline in the component file
- Every mock marked with `// TODO: replace with real API`
- Use realistic but clearly fake data

### Component Structure

- One component per file
- Props interface defined for every component
- Clear parent-child hierarchy
- Responsive layout using the project's design system

### File Naming

- Components: `PascalCase.tsx` (e.g., `LoginForm.tsx`)
- Hooks: `use{Name}.ts` (e.g., `useFormValidation.ts`)
- Utilities: `camelCase.ts` (e.g., `formatDate.ts`)

## Handoff Checklist

Before running `/trellis:handoff`, verify:

- [ ] All components render without console errors
- [ ] Mock data locations documented with file:line references
- [ ] Component hierarchy described
- [ ] "Needs frontend implementation" items listed
- [ ] Responsive behavior verified at common breakpoints
- [ ] Accessibility basics: labels, alt text, keyboard navigation
