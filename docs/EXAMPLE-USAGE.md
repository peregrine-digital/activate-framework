# Example Usage

This document illustrates how the file hierarchy works in practice across a cross-functional team.

## File Hierarchy

| Tier | Type | Location | Scope | Invocation |
|:------|:------|:----------|:-------|:------------|
| 1 | AGENTS.md | Plugin root | Project-wide | Always active |
| 2 | Instruction files | `plugins/{name}/instructions/` | Context-specific | Automatic via glob patterns |
| 2 | Prompt files | `plugins/{name}/prompts/` | Task-specific | On-demand via `/command` |
| 3 | Skills | `plugins/{name}/skills/{skill}/SKILL.md` | Procedural | Explicit, on-demand |
| 4 | Agent definitions | `plugins/{name}/agents/` | Persona + capabilities | Explicit |

- **AGENTS.md** defines how we work: commit conventions, branching strategy, TDD expectations
- **Instruction files** provide context-specific guidance: language conventions, code review checklists
- **Prompt files** are reusable tasks invoked as `/commands`: code reviews, accessibility checks, ADR scaffolding
- **Skills** are reusable procedures: creating an ADR, scaffolding a component
- **Agent definitions** combine persona, capabilities, and which skills/instructions to use

For full details, see the [plugin file hierarchy](plugin-file-hierarchy.md) documentation.

---

## Scenario: Cross-Functional Team Building a UserProfile Feature

A cross-functional team is building a new UserProfile feature. Each role uses the file hierarchy differently, but all share the same baseline expectations from AGENTS.md.

### Product Manager: Writing the User Story

| Tier | File | What it contributes |
|:------:|:------|:---------------------|
| 1 | `AGENTS.md` | "Link work to GitHub issues", "Maintain traceability" |
| 2 | `product.instructions.md` | "Use Given/When/Then for acceptance criteria", "Include accessibility requirements" |
| 3 | `write-user-story/SKILL.md` | Procedure for creating well-formed stories with examples |

**Result:** A user story in GitHub Issues with structured acceptance criteria

---

### Designer: Creating Component Specs

| Tier | File | What it contributes |
|:------|:------|:---------------------|
| 1 | `AGENTS.md` | "Document decisions in session logs" |
| 2 | `design.instructions.md` | "Reference USWDS tokens", "Include responsive breakpoints", "Document accessibility considerations" |
| 3 | `design-component-spec/SKILL.md` | Procedure for documenting component states, variants, and interactions |

**Result:** A component spec with design tokens, states, and a11y notes

---

### Developer: Implementing the Component

| Tier | File | What it contributes |
|:------|:------|:---------------------|
| 1 | `AGENTS.md` | "Use TDD", "Commit atomically", "Use conventional commits" |
| 2 | `javascript.instructions.md` | "Use TypeScript strict mode", "Prefer named exports" |
| 2 | `react.instructions.md` | "Functional components only", "Use design system tokens" |
| — | `/code-review` | On-demand structured code review with priority-based checklist |
| — | `/accessibility-check` | On-demand WCAG 2.1 AA audit |
| 3 | `create-component/SKILL.md` | Step-by-step TDD workflow for new components |
| 4 | `code-reviewer.agent.md` | Agent that reviews code before PR, using `code-review.instructions.md` and `accessibility.instructions.md` |

**Example Workflow:**

```text
1. Developer implements component using create-component.md skill
2. Quick self-check: type /code-review in chat to run the code review prompt
3. Fix any critical or important issues identified
4. Type /accessibility-check to verify WCAG 2.1 AA compliance
5. For a deeper review, invoke: "@code-reviewer review my changes"
6. code-reviewer agent:
   - Loads code-review.instructions.md (review priorities, checklist)
   - Loads accessibility.instructions.md (a11y checks)
   - Loads react.instructions.md (React-specific patterns)
   - Provides feedback following all three instruction sets
7. Developer addresses feedback, then opens PR
```

> **Prompts vs Agents:** Use `/code-review` for a quick, focused review of specific code.
> Use `@code-reviewer` when you want a thorough, multi-file review that orchestrates
> multiple instruction sets and skills together.

**Result:** A tested, self-reviewed component ready for human code review

---

### QA: Writing Test Cases

| Tier | File | What it contributes |
|:------|:------|:---------------------|
| 1 | `AGENTS.md` | "Maintain traceability to acceptance criteria" |
| 2 | `testing.instructions.md` | "Cover happy path, edge cases, and error states", "Include a11y checks" |
| 2 | `accessibility.instructions.md` | "Test with screen reader", "Verify WCAG 2.1 AA compliance" |
| 3 | `write-test-plan/SKILL.md` | Procedure for creating test cases from acceptance criteria |

**Result:** Test cases linked to acceptance criteria with a11y coverage

---

### Tech Writer: Documenting the Feature

| Tier | File | What it contributes |
|:------|:------|:---------------------|
| 1 | `AGENTS.md` | "Follow established conventions" |
| 2 | `documentation.instructions.md` | "Use active voice", "Include code examples", "Follow style guide" |
| 3 | `document-component/SKILL.md` | Procedure for creating component documentation with props table |

**Result:** User-facing docs with examples and API reference

---

## How It All Connects

```text
plugins/{plugin-name}/
┌─────────────────────────────────────────────────────────────────────┐
│                         AGENTS.md                                   │
│        TDD • Atomic commits • Traceability • Session logs           │
├─────────────┬─────────────┬─────────────┬─────────────┬─────────────┤
│  Product.   │  Design     │    Dev      │    QA.      │    Docs     │
│instructions │instructions │instructions │instructions │instructions │
├─────────────┴──────┬──────┴─────────────┴──────┬──────┴─────────────┤
│                    │      Prompt Files          │                    │
│                    │ /code-review               │                    │
│                    │ /accessibility-check       │                    │
│                    │ /create-adr                │                    │
├─────────────┬──────┴─────────────┬─────────────┬──────┴─────────────┤
│  write-     │  design-    │  create-    │  write-     │   document- │
│  user-      │  component/ │  component/ │  test-      │   component/│
│  story/     │  spec/      │  SKILL.md   │  plan/      │   SKILL.md  │
│  SKILL.md   │  SKILL.md   │  (skill)    │  SKILL.md   │             │
├─────────────┴─────────────┼─────────────┼─────────────┴─────────────┤
│                           │  @code-     │                           │
│                           │  reviewer   │ ◄── Tier 4 Agent          │
│                           │  (agent)    │     orchestrates          │
│                           │             │     instructions + skills │
└───────────────────────────┴─────────────┴───────────────────────────┘
            │           │           │           │           │
            ▼           ▼           ▼           ▼           ▼
       User Story → Design Spec → Component → Test Cases → Docs
       (GitHub)     (Figma/MD)    (Code+Test)  (Test Plan)  (README)
```

## Core Concept

Agents don't replace other tiers—they orchestrate them. The `code-reviewer` agent knows which instruction files and skills to apply for its specialized task, while still respecting the baseline expectations from AGENTS.md.

This composable approach means:

- Teams can share common practices (AGENTS.md)
- Roles get tailored guidance (instruction files)
- Common tasks become one-command shortcuts (prompt files)
- Complex workflows are codified (skills)
- Specialized personas combine it all (agent definitions)

## Activate Core Receiver Template Example

For downstream repositories consuming cross-org updates, use `templates/activate-core-receiver.workflow.yml` to receive `repository_dispatch` events of type `activate-core-update`, verify checksum, run quality gates, and open/update a labeled PR.
