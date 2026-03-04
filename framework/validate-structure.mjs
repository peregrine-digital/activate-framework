/**
 * Validate plugin structure against ADR-001 file hierarchy.
 * 
 * Usage:
 *   node framework/validate-structure.mjs [plugin-name]
 * 
 * If no plugin name is provided, validates all plugins.
 */

import { readdir, readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const PLUGINS_DIR = path.join(__dirname, '..', 'plugins');

/**
 * Extract YAML frontmatter from markdown content.
 * @param {string} content - Markdown file content
 * @returns {object|null} Parsed frontmatter or null if none found
 */
function extractFrontmatter(content) {
  const match = content.match(/^---\n([\s\S]*?)\n---/);
  if (!match) return null;
  
  const fmContent = match[1].trim();
  if (!fmContent) return {}; // Empty frontmatter block
  
  const fm = {};
  const lines = fmContent.split('\n');
  for (const line of lines) {
    const colonIdx = line.indexOf(':');
    if (colonIdx > 0) {
      const key = line.slice(0, colonIdx).trim();
      let value = line.slice(colonIdx + 1).trim();
      // Remove quotes if present
      if ((value.startsWith("'") && value.endsWith("'")) || 
          (value.startsWith('"') && value.endsWith('"'))) {
        value = value.slice(1, -1);
      }
      // Only set non-empty values
      if (value) {
        fm[key] = value;
      }
    }
  }
  return fm;
}

/**
 * Check if a path exists and is a directory.
 */
async function isDirectory(p) {
  try {
    const s = await stat(p);
    return s.isDirectory();
  } catch {
    return false;
  }
}

/**
 * Check if a path exists and is a file.
 */
async function isFile(p) {
  try {
    const s = await stat(p);
    return s.isFile();
  } catch {
    return false;
  }
}

/**
 * Validate a single plugin against ADR-001 structure.
 * @param {string} pluginPath - Absolute path to plugin directory
 * @returns {object} Validation result with errors and warnings
 */
async function validatePlugin(pluginPath) {
  const pluginName = path.basename(pluginPath);
  const errors = [];
  const warnings = [];

  // Tier 1: AGENTS.md recommended
  const agentsMdPath = path.join(pluginPath, 'AGENTS.md');
  if (!(await isFile(agentsMdPath))) {
    warnings.push('Missing AGENTS.md at plugin root (Tier 1 recommended)');
  }

  // Tier 2: instructions/ directory
  const instructionsDir = path.join(pluginPath, 'instructions');
  if (await isDirectory(instructionsDir)) {
    const files = await readdir(instructionsDir);
    const instructionFiles = files.filter(f => f.endsWith('.instructions.md'));
    
    for (const file of instructionFiles) {
      const content = await readFile(path.join(instructionsDir, file), 'utf-8');
      const fm = extractFrontmatter(content);
      
      if (!fm) {
        errors.push(`instructions/${file}: Missing frontmatter`);
      } else if (!fm.applyTo) {
        errors.push(`instructions/${file}: Missing 'applyTo' in frontmatter`);
      }
      if (fm && !fm.description) {
        warnings.push(`instructions/${file}: Missing 'description' in frontmatter`);
      }
    }
  }

  // Tier 2: prompts/ directory
  const promptsDir = path.join(pluginPath, 'prompts');
  if (await isDirectory(promptsDir)) {
    const files = await readdir(promptsDir);
    const promptFiles = files.filter(f => f.endsWith('.prompt.md'));
    
    for (const file of promptFiles) {
      const content = await readFile(path.join(promptsDir, file), 'utf-8');
      const fm = extractFrontmatter(content);
      
      if (!fm) {
        errors.push(`prompts/${file}: Missing frontmatter`);
      } else if (!fm.description) {
        warnings.push(`prompts/${file}: Missing 'description' in frontmatter`);
      }
    }
  }

  // Tier 3: skills/ directory
  const skillsDir = path.join(pluginPath, 'skills');
  if (await isDirectory(skillsDir)) {
    const entries = await readdir(skillsDir);
    
    for (const entry of entries) {
      const skillPath = path.join(skillsDir, entry);
      if (!(await isDirectory(skillPath))) continue;
      
      const skillMdPath = path.join(skillPath, 'SKILL.md');
      if (!(await isFile(skillMdPath))) {
        errors.push(`skills/${entry}/: Missing SKILL.md`);
        continue;
      }
      
      const content = await readFile(skillMdPath, 'utf-8');
      const fm = extractFrontmatter(content);
      
      if (!fm) {
        errors.push(`skills/${entry}/SKILL.md: Missing frontmatter`);
      } else {
        if (!fm.name) {
          errors.push(`skills/${entry}/SKILL.md: Missing 'name' in frontmatter`);
        }
        if (!fm.description) {
          errors.push(`skills/${entry}/SKILL.md: Missing 'description' in frontmatter`);
        }
      }
    }
  }

  // Tier 4: agents/ directory
  const agentsDir = path.join(pluginPath, 'agents');
  if (await isDirectory(agentsDir)) {
    const files = await readdir(agentsDir);
    const agentFiles = files.filter(f => f.endsWith('.agent.md'));
    
    for (const file of agentFiles) {
      const content = await readFile(path.join(agentsDir, file), 'utf-8');
      const fm = extractFrontmatter(content);
      
      if (!fm) {
        errors.push(`agents/${file}: Missing frontmatter`);
      } else {
        if (!fm.name) {
          errors.push(`agents/${file}: Missing 'name' in frontmatter`);
        }
        if (!fm.description) {
          errors.push(`agents/${file}: Missing 'description' in frontmatter`);
        }
      }
    }
  } else {
    warnings.push('No agents/ directory found');
  }

  return {
    plugin: pluginName,
    valid: errors.length === 0,
    errors,
    warnings,
  };
}

/**
 * Discover and validate all plugins or a specific one.
 * @param {string|null} targetPlugin - Plugin name to validate, or null for all
 */
async function main(targetPlugin = null) {
  const plugins = await readdir(PLUGINS_DIR);
  const results = [];

  for (const plugin of plugins) {
    const pluginPath = path.join(PLUGINS_DIR, plugin);
    if (!(await isDirectory(pluginPath))) continue;
    if (targetPlugin && plugin !== targetPlugin) continue;

    const result = await validatePlugin(pluginPath);
    results.push(result);
  }

  // Output results
  let hasErrors = false;
  
  for (const result of results) {
    const status = result.valid ? '✅' : '❌';
    console.log(`\n${status} ${result.plugin}`);
    
    if (result.errors.length > 0) {
      hasErrors = true;
      console.log('  Errors:');
      for (const err of result.errors) {
        console.log(`    ❌ ${err}`);
      }
    }
    
    if (result.warnings.length > 0) {
      console.log('  Warnings:');
      for (const warn of result.warnings) {
        console.log(`    ⚠️  ${warn}`);
      }
    }
    
    if (result.valid && result.warnings.length === 0) {
      console.log('  All checks passed');
    }
  }

  console.log('\n' + '─'.repeat(40));
  const passed = results.filter(r => r.valid).length;
  const failed = results.filter(r => !r.valid).length;
  console.log(`Summary: ${passed} passed, ${failed} failed`);

  process.exit(hasErrors ? 1 : 0);
}

// CLI entry point
const targetPlugin = process.argv[2] || null;
main(targetPlugin).catch(err => {
  console.error('Validation failed:', err.message);
  process.exit(1);
});
