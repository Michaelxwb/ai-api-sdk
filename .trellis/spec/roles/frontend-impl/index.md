# Frontend Implementation Role Specification

> Work standards and constraints for the Frontend Developer role.

---

## Role Definition

- **Role ID**: frontend
- **Primary Output**: Production-ready frontend code
- **Output Directory**: Bound via `trellis init -d`

## Work Standards

### Implementation Guidelines
- Replace all mock data with real API calls
- Follow the project's existing patterns for API integration
- Preserve prototype's component structure unless refactoring is justified

### API Integration
- Use the project's standard HTTP client
- Implement proper error handling for all API calls
- Add loading states for async operations
- Handle empty states and error states

### Code Quality
- TypeScript strict mode compliance
- No `any` types without justification
- Unit tests for business logic
- Integration tests for API calls

### Quality Checklist
- [ ] All `// TODO: replace with real API` markers resolved
- [ ] Error handling implemented for all API calls
- [ ] Loading and empty states handled
- [ ] TypeScript compiles without errors
- [ ] Lint passes
- [ ] Core business logic has unit tests

## Handoff Guidelines

When running `/trellis:handoff`:
- List API integrations completed
- Document any deviations from prototype
- Note remaining technical debt or known issues
- Include test coverage summary

## Constraints

- Only write files within your bound output directory
- Do not modify requirements or prototype code
- Preserve prototype component interfaces unless explicitly agreed
