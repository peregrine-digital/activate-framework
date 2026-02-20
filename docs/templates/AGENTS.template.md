# AGENTS.md

<!-- Update the placeholders inside double braces before distributing -->

Guidelines for AI agents and human contributors working on {{project_name}}.

## Repository Purpose

{{repository_purpose}}

## Repository Structure

<!-- Replace with a mermaid diagram or bullet list that mirrors the target repository -->
{{repository_structure_overview}}

## Guidance Hierarchy

This project uses the four-tier structure defined in ADR-001. Adjust the table if the downstream repository differs.

| Tier | Location | Scope | Invocation |
|------|----------|-------|------------|
| 1 | `AGENTS.md` | Project-wide | Always active |
| 2 | `.github/instructions/*.instructions.md` | Context-specific | Glob pattern match |
| 3 | `.github/skills/[name]/SKILL.md` | Procedural | On-demand |
| 4 | `.github/agents/[name].agent.md` | Persona + capabilities | Explicit selection |

## Commands

- {{command_1}} — {{command_1_purpose}}
- {{command_2}} — {{command_2_purpose}}

## Testing & Quality Gates

- {{test_command}} — {{test_command_purpose}}
- {{additional_gate}} — {{additional_gate_purpose}}

## Project Structure Highlights

- {{path_1}} — {{path_1_description}}
- {{path_2}} — {{path_2_description}}

## Code Style Example

```{{language_hint}}
{{code_style_example}}
```

## Core Principles

### Workflow Expectations

- {{workflow_expectation_1}}
- {{workflow_expectation_2}}
- {{workflow_expectation_3}}

### Quality Guardrails

- {{quality_guardrail_1}}
- {{quality_guardrail_2}}
- {{quality_guardrail_3}}

## Git Workflow

- {{git_expectation_1}}
- {{git_expectation_2}}

## Boundaries

### Always

- {{always_action_1}}
- {{always_action_2}}

### Ask First

- {{ask_action_1}}
- {{ask_action_2}}

### Never

- {{never_action_1}}
- {{never_action_2}}

## Getting Started

1. {{getting_started_step_1}}
2. {{getting_started_step_2}}
3. {{getting_started_step_3}}

## References

- {{reference_link_1}}
- {{reference_link_2}}
