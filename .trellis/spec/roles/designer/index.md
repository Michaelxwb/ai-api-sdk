# Designer Role Specification

> Work standards and constraints for the Interaction Designer role.

---

## Role Definition

- **Role ID**: designer
- **Primary Output**: Interactive prototypes with mock data
- **Output Directory**: Bound via `trellis init -d`

## Work Standards

### Prototype Requirements
- All prototypes must be runnable (not static mockups)
- Use inline mock data with `// TODO: replace with real API` markers
- Follow the project's existing component library and design system

### Mock Data Convention
- Inline mock data in component files (not separate mock files)
- Mark every mock with: `// TODO: replace with real API`
- Use realistic but clearly fake data (e.g., "test@example.com")

### Component Structure
- One component per file
- Clear component hierarchy documented in HANDOFF.md
- Props interface defined for each component

### Quality Checklist
- [ ] Prototype renders without errors
- [ ] All mock data marked with TODO comments
- [ ] Component hierarchy documented
- [ ] Responsive layout considered
- [ ] Accessibility basics covered (labels, alt text)

## Handoff Guidelines

When running `/trellis:handoff`:
- List all components created with brief descriptions
- Document mock data locations (file:line)
- Create "needs frontend implementation" checklist
- Note any design decisions that differ from PRD

## Constraints

- Only write files within your bound output directory
- Do not modify requirements or production code
- Mock data must be inline, not in separate services
