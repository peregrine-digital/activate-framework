# AGENTS.md
<!-- 
This is the DISTRIBUTABLE AGENTS.md template.
Teams installing this starter kit will receive this file.
Customize the content below for their project.
-->

Guidelines for AI agents and human contributors working in this repository.
For agents developing in Activate Framework
Legend (from RFC2119): !=MUST, ~=SHOULD, ≉=SHOULD NOT, ⊗=MUST NOT, ?=MAY.

## Working principles

**Do**
- !: Trunk-based development best practices
- !: Implement small, well scoped changes that follow existing conventions for a single task at a time.
- !: Keep all tests and policy checks green. Test prior to PR
- !: Maintain quality and automation and ADR discipline.
- ~: Create simple, maintainable designs over clever abstractions.
- ~: Follow TDD. 
- !: Atomic commits as you go, following conventional commit format
- !: After finishing work (all todos done, create pr), review session and propose recommended improvements to AGENTS.md, custom agents, and skills.
- !: Push branch changes to remote, open PR

**Do Not**

- ⊗: introduce secrets, tokens, credentials, or private keys in any form.
- ⊗: Redesign the architecture without explicit instruction or approval
- ⊗: Introduce new tools or services without explicit instruction or approval
- ⊗: Make large sweeping changes across many apps or modules without explicit approval

<!-- 
## Code Map
**For Agents:** Replace this with concise, high-level code-map and link to more detailed map in docs/REPO-STRUCTURE.md
-->

## Commit Message Conventions

Use [Conventional Commits](https://www.conventionalcommits.org/):

```text
type: description

[optional body]
```

**Types:** `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

## Code Quality

<!-- Customize these principles for your project -->

- Write tests for new functionality
- Follow language-specific conventions (see instruction files)
- Keep commits atomic and reviewable

## Agent Workflow

### Session Logging

<!-- 
Configure your log location below. Options:
- `docs/dev/logs/` - Version-controlled logs (good for team visibility)
- `logs/` or `.logs/` - Add to .gitignore if logs shouldn't be committed
-->

When starting a new feature or branch, create a session log to track work:

1. **Verify log location on first use**
   - Confirm with the user where session logs should be stored
   - If the directory doesn't exist, ask before creating it
   - Check if logs should be added to `.gitignore` or, if that file is not appropriate, another exclude approach like `.git/info/exclude`

2. **Create the log file** before any other work
   - Format: `<log-directory>/YYYY-MM-DD-<branch-name>.md`
   - Include: Objective, Related (issue/PR links), empty Work Completed section
s
3. **Update incrementally** after each commit:
   - Add entry to Work Completed with timestamp
   - Document your reasoning: why this approach? what alternatives were considered?
   - Capture what you learned or discovered during implementation
   - Include the commit message for traceability

4. **Capture decisions and lessons as they happen**
   - Don't wait until session end to record insights
   - Document "why" not just "what"—future contributors need context

Logs should contain:

- **Objective** – What the session aims to accomplish
- **Related** – Links to issues and PRs
- **Work completed** – Summary of each task with timestamps and commit references
- **Decisions made** – Choices, alternatives considered, and rationale
- **Lessons learned** – What would you do differently? What should be improved?

## Proactive Self-Improvement

At the end of each session (or when prompted), agents must:

1. Review the conversation for lessons learned
2. Identify gaps or friction in the current workflows
3. Propose or implement improvements to AGENTS.md, instructions, or skills

This ensures the repository continuously evolves based on real usage.

## Session Completion

**When ending a work session**, complete ALL steps below. Work is NOT complete until changes are pushed.

1. **File issues for remaining work** – Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) – Tests, linters, builds
3. **Update issue status** – Close finished work, update in-progress items
4. **Push to remote**:

   ```bash
   git pull --rebase
   git push
   git status  # Should show "up to date with origin"
   ```

5. **Verify** – All changes committed and pushed
6. **Hand off** – Provide context for next session

## Discovering Available Guidance

To see what guidance is included in your installation:

```bash
# List instruction files (context-specific rules)
ls ./instructions/

# List prompt files (reusable slash commands)
ls ./prompts/

# List skills (procedural workflows)
ls ./skills/

# List agents (specialized personas)
ls ./agents/
```

To use a skill or agent, reference it in your prompt:

```text
# Reference a prompt
Type /code-review in the chat input to invoke a prompt

# Reference a skill
Use the skill in .github/skills/[skill-name]/SKILL.md to...

# Reference an agent
Follow the guidance in .github/agents/[agent-name].agent.md to...
```
