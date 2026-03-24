# Creating Customization Files

This project uses VS Code's [agent customization](https://code.visualstudio.com/docs/copilot/copilot-customization) primitives to provide AI-assisted development guidance. When you need to create a new instruction file, skill, prompt, or agent definition, VS Code can help.

## Use VS Code's Built-in Skill

VS Code includes a built-in **agent-customization** skill that understands the full set of customization primitives. When you ask Copilot to create a new customization file, this skill automatically:

1. **Selects the appropriate primitive** based on your intent
2. **Places the file** in the correct directory
3. **Generates valid frontmatter** with required fields
4. **Validates** the result

You do not need to memorize file formats or directory conventions — the built-in skill handles this for you.

## Plugin Architecture

Activate Framework uses a plugin-based architecture. Customization files are organized into plugins under `plugins/`:

```
plugins/
├── adhoc/                       # Core framework plugin
│   ├── AGENTS.md
│   ├── instructions/
│   ├── prompts/
│   ├── skills/
│   └── agents/
└── ironarch/                    # VA workflow plugin
    ├── AGENTS.md
    ├── instructions/
    ├── prompts/
    ├── skills/
    └── agents/
```

Each plugin must follow the [four-tier file hierarchy](plugin-file-hierarchy.md):

| Tier | Location | Invocation |
|------|----------|------------|
| 1 | `AGENTS.md` | Always active |
| 2 | `instructions/*.instructions.md` | Auto by glob pattern |
| 2 | `prompts/*.prompt.md` | Manual via `/command` |
| 3 | `skills/{name}/SKILL.md` | On-demand |
| 4 | `agents/*.agent.md` | User selects |

## Creating Files in a Plugin

### 1. Choose the right plugin

- **adhoc** — General-purpose guidance applicable to any project
- **ironarch** — VA-specific workflow with specialized agents
- **New plugin** — Create `plugins/{your-plugin}/` for domain-specific content

### 2. Create the file

In VS Code's Copilot chat, describe what you need:

- *"Create an instruction file for Python code style"*
- *"Add a prompt for generating unit tests"*
- *"Set up a skill for database migration workflows"*
- *"Create an agent that specializes in security reviews"*

Place the file in the appropriate directory within your plugin.

### 3. Add to manifest

Update `manifests/{plugin-name}.json` to include the new file:

```json
{
  "src": "instructions/python.instructions.md",
  "dest": "instructions/python.instructions.md",
  "tier": "core",
  "category": "instructions",
  "description": "Python conventions and project-specific patterns"
}
```

### 4. Validate

Run the validation script to ensure structure compliance:

```bash
npm run validate:plugins
```

## Work Iteratively from Your Code

The most effective way to create customization files is to let Copilot derive them from your actual codebase. Rather than writing guidance from scratch, point the agent at your existing code and let it identify patterns worth codifying.

### Start from what's already there

Open your project in VS Code and ask Copilot to analyze your code for conventions:

- *"Look through the code in this repo and identify common patterns, naming conventions, and style choices that we should document as instruction files"*
- *"Review our API routes and suggest an instruction file that captures our error handling conventions"*
- *"Examine our React components and create an instruction file for the component patterns we follow"*

### Refine through conversation

The first draft is a starting point. Work back and forth with Copilot to refine the output:

1. **Generate** — Ask the agent to analyze your code and draft a customization file
2. **Review** — Read through what it produced and note what's missing or over-specified
3. **Refine** — Ask it to adjust: *"Add our logging conventions"* or *"Remove the section about testing — we'll handle that separately"*
4. **Validate** — Try using the new file in a real task to see if it produces the behavior you want

### Build up over time

You don't need to capture everything at once. Start with the conventions that matter most — the ones new team members get wrong or that cause the most code review friction — and add more files as patterns emerge.

## Quick Decision Guide

| Question | Primitive |
|----------|-----------|
| Applies to most work in the project? | `AGENTS.md` at plugin root |
| Applies to specific file types or contexts? | `instructions/*.instructions.md` with `applyTo` |
| A repeatable single task? | `prompts/*.prompt.md` |
| A multi-step workflow with scripts or templates? | `skills/{name}/SKILL.md` |
| Needs its own persona, tools, or context isolation? | `agents/*.agent.md` |

## Creating a New Plugin

If your customizations don't fit an existing plugin:

1. **Create the plugin directory:**
   ```bash
   mkdir -p plugins/my-plugin/{instructions,prompts,skills,agents}
   ```

2. **Add AGENTS.md** (required):
   ```bash
   touch plugins/my-plugin/AGENTS.md
   ```

3. **Create a manifest** in `manifests/my-plugin.json`:
   ```json
   {
     "name": "My Plugin",
     "description": "What this plugin provides",
     "version": "0.1.0",
     "basePath": "plugins/my-plugin",
     "tiers": [
       { "id": "core", "label": "Core" }
     ],
     "files": [
       {
         "src": "AGENTS.md",
         "dest": "AGENTS.md",
         "tier": "core",
         "category": "other",
         "description": "Plugin guidance"
       }
     ]
   }
   ```

4. **Validate:**
   ```bash
   npm run validate:plugins
   ```

## Further Reading

- [Plugin File Hierarchy](plugin-file-hierarchy.md) — Structure requirements
- [VS Code Copilot Customization docs](https://code.visualstudio.com/docs/copilot/copilot-customization)
