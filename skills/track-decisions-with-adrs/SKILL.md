---
name: track-decisions-with-adrs
description: Use when architecture or process choices are made (especially under time pressure or via chat) to ensure every consequential decision is captured in an ADR with context, decision, alternatives, and follow-up actions.
version: '0.5.0'
---

# Track Decisions with ADRs

## Overview
Keep the historical record ahead of the work. Every consequential technical or process choice gets logged in an Architecture Decision Record (ADR) within 24 hours, even if the implementation is still in flight.

## When to Use
- New architecture, compliance, or tooling direction is chosen (including “temporary” stop-gaps)
- A decision happens in meetings, chat, or tickets and isn’t already written down
- A change meaningfully affects standards, interoperability, compliance, or long-term maintenance
- Someone says “we’ll jot this down later” or “this is obvious”

Do **not** use for trivial refactors, formatting tweaks, or choices already codified in existing ADRs/standards.

## Core Pattern

1. **Identify the decision trigger**
   - Scope, constraint, or standard changed
   - Approval given or blocked
2. **Open (or update) ADR promptly**
   - Create new ADR file (`docs/adrs/YYYY-MM-DD-title.md`) or amend relevant one
3. **Capture the decision triangle**
   - **Context:** Why action was needed, pressures, stakeholders
   - **Decision:** Chosen option, owner, date
   - **Consequences:** Immediate impact, follow-up tasks, revisit triggers
   - Include alternatives considered + why rejected
4. **Link and broadcast**
   - Reference ADR from commits, issues, PR description
   - Drop link in the conversation where decision occurred
5. **Revisit schedule**
   - Add explicit review checkpoints (e.g., “Re-evaluate Q1 FY26”), create follow-up issue as needed

## Quick Reference

| Trigger | Action | Notes |
|---------|--------|-------|
| New architecture/compliance direction | Create ADR within 24h | Use `docs/adrs/YYYY-MM-DD-title.md` |
| “Temporary” workaround | Document as ADR + revisit date | Temporary ≠ exempt |
| Decision via chat/meeting | Summarize + ADR link in same thread | Prevents context loss |
| Reversing previous ADR | Amend original + reference superseding ADR | Maintain chain of custody |

## Implementation Checklist
- [ ] Capture decision in ADR matrix: context, decision, alternatives, consequences
- [ ] Record stakeholders, date, reviewer/approver
- [ ] Reference relevant standards or external mandates (FedRAMP, in-toto, etc.)
- [ ] Note follow-up tasks and assign owners (issues, tickets)
- [ ] Link ADR in commit/PR/meeting notes
- [ ] Update path filters or workflows if tooling/process changes

## Common Mistakes (and Counters)
- **“We’ll document after launch.”** → Decisions harden silently; mandate ADR before merge/deploy.
- **“It’s temporary.”** → Temporary solutions persist; ADR must state exit criteria and review date.
- **“Chat history is enough.”** → Chat scrollback disappears; ADR centralizes ratified context.
- **“Everyone knows this.”** → Teams change; make implicit norms explicit.
- **“Writing ADRs slows us down.”** → Lost context and rework cost more; aim for lightweight but timely entries.

## Red Flags
- No ADR exists for major infrastructure, compliance, or standards shifts.
- Decision rationale only lives in Slack/Teams or an oral conversation.
- ADR backlog >1 day; deadlines cited to skip documentation.
- Pull requests referencing “per conversation” without link.

## Rationalization Table

| Pressure | Likely Excuse | Skill Counter |
|----------|---------------|---------------|
| Imminent deadline | “We’ll note it later.” | ADR must exist before merge/deploy; capture skeleton now, refine later. |
| Temporary fix | “This is just for now.” | ADR documents stop-gap, exit criteria, and review date. |
| Meeting-only decision | “Everyone heard it.” | Document meeting summary + ADR link; oral knowledge decays. |
| Busy stakeholders | “No time to review.” | Record decision + mark approvers; asynchronous review still needed. |
| Small change creep | “It’s just a minor tweak.” | If it affects standards, compliance, or cross-team interfaces, it’s consequential. |

## Integration with Standards
- Reference in-toto/SLSA, OSCAL, or other open standards within ADR context to maintain interoperability.
- Point ADRs to relevant skills (e.g., this one, compliance checklists) to keep practices discoverable.

## Example ADR Skeleton
```markdown
# 2025-10-30-adopt-in-toto-attestations.md

- **Status:** Accepted
- **Context:** Need interoperable supply-chain evidence across AWS/Azure.
- **Decision:** Use in-toto attestations (SLSA v1 + vulnerability predicate) for all build artifacts.
- **Consequences:** Update verifier, evidence schema, GitHub Actions; document standards mapping; revisit Q1 FY26.
- **Alternatives:** Custom JSON bundle (rejected: vendor lock-in, low interoperability); minimal metadata (rejected: fails compliance).
- **Follow-ups:** #123 configure cosign, #124 update promotion workflow.
```

## Verification
- Review recent commits/issues to confirm every major decision links to an ADR.
- Spot-check a time-pressured change to ensure ADR exists with revisit date.
- During retros, audit ADR log for stale follow-up items.
