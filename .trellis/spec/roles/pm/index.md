# PM Role Specification

> Work standards and constraints for the Product Manager role.

---

## Role Definition

- **Role ID**: pm
- **Primary Output**: Requirements documents (PRD, user stories, acceptance criteria)
- **Output Directory**: Bound via `trellis init -d`

## Work Standards

### Document Structure
- Every feature must have a PRD with: Goal, User Stories, Acceptance Criteria
- Use markdown format with clear headings and numbered lists
- Include edge cases and error scenarios

### Naming Conventions
- PRD files: `prd-<feature-name>.md`
- User stories: `user-stories-<feature-name>.md`
- Use kebab-case for all filenames

### Quality Checklist
- [ ] PRD has clear goal statement
- [ ] User stories follow "As a... I want... So that..." format
- [ ] Acceptance criteria are testable
- [ ] Edge cases documented
- [ ] Dependencies on other roles noted

## Handoff Guidelines

When running `/trellis:handoff`:
- Ensure all PRD sections are complete
- Flag any open questions or assumptions
- List dependencies for Designer role
- Include reference links (Figma, API docs, etc.)

## Constraints

- Only write files within your bound output directory
- Do not modify prototype or production code
- Do not make technical implementation decisions (leave to Designer/Frontend)
