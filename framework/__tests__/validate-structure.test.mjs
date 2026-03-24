/**
 * Tests for validate-structure.mjs
 */

import { describe, it, before, after } from 'node:test';
import assert from 'node:assert';
import { mkdir, writeFile, rm } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execSync } from 'node:child_process';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const SCRIPT_PATH = path.join(__dirname, '..', 'validate-structure.mjs');
const TEST_PLUGINS_DIR = path.join(__dirname, '..', '..', 'plugins', '__test-plugin__');

/**
 * Run the validation script and capture output.
 */
function runValidator(pluginName = null) {
  const args = pluginName ? `"${pluginName}"` : '';
  try {
    const output = execSync(`node "${SCRIPT_PATH}" ${args}`, {
      cwd: path.join(__dirname, '..', '..'),
      encoding: 'utf-8',
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    return { exitCode: 0, output };
  } catch (err) {
    return { exitCode: err.status, output: err.stdout + err.stderr };
  }
}

describe('validate-structure.mjs', () => {
  before(async () => {
    // Create a test plugin directory
    await mkdir(TEST_PLUGINS_DIR, { recursive: true });
  });

  after(async () => {
    // Clean up test plugin
    await rm(TEST_PLUGINS_DIR, { recursive: true, force: true });
  });

  it('should warn when AGENTS.md is missing', async () => {
    const result = runValidator('__test-plugin__');
    assert.strictEqual(result.exitCode, 0);
    assert.ok(result.output.includes('Missing AGENTS.md'));
  });

  it('should pass when AGENTS.md exists', async () => {
    await writeFile(path.join(TEST_PLUGINS_DIR, 'AGENTS.md'), '# AGENTS.md\n');
    const result = runValidator('__test-plugin__');
    assert.strictEqual(result.exitCode, 0);
    assert.ok(result.output.includes('✅ __test-plugin__'));
  });

  it('should warn when instruction file missing frontmatter', async () => {
    await mkdir(path.join(TEST_PLUGINS_DIR, 'instructions'), { recursive: true });
    await writeFile(
      path.join(TEST_PLUGINS_DIR, 'instructions', 'test.instructions.md'),
      '# Test instruction\nNo frontmatter here.'
    );
    const result = runValidator('__test-plugin__');
    assert.strictEqual(result.exitCode, 1);
    assert.ok(result.output.includes('Missing frontmatter'));
  });

  it('should error when instruction missing applyTo', async () => {
    await writeFile(
      path.join(TEST_PLUGINS_DIR, 'instructions', 'test.instructions.md'),
      '---\ndescription: "Test"\n---\n# Test'
    );
    const result = runValidator('__test-plugin__');
    assert.strictEqual(result.exitCode, 1);
    assert.ok(result.output.includes("Missing 'applyTo'"));
  });

  it('should pass with valid instruction file', async () => {
    await writeFile(
      path.join(TEST_PLUGINS_DIR, 'instructions', 'test.instructions.md'),
      '---\ndescription: "Test"\napplyTo: "**/*.js"\n---\n# Test'
    );
    const result = runValidator('__test-plugin__');
    assert.strictEqual(result.exitCode, 0);
  });

  it('should error when skill folder missing SKILL.md', async () => {
    await mkdir(path.join(TEST_PLUGINS_DIR, 'skills', 'test-skill'), { recursive: true });
    const result = runValidator('__test-plugin__');
    assert.strictEqual(result.exitCode, 1);
    assert.ok(result.output.includes('Missing SKILL.md'));
  });

  it('should error when SKILL.md missing name/description', async () => {
    await writeFile(
      path.join(TEST_PLUGINS_DIR, 'skills', 'test-skill', 'SKILL.md'),
      '---\nauthor: someone\n---\n# Test skill'
    );
    const result = runValidator('__test-plugin__');
    assert.strictEqual(result.exitCode, 1);
    assert.ok(result.output.includes("Missing 'name'"));
    assert.ok(result.output.includes("Missing 'description'"));
  });

  it('should pass with valid skill', async () => {
    await writeFile(
      path.join(TEST_PLUGINS_DIR, 'skills', 'test-skill', 'SKILL.md'),
      '---\nname: test-skill\ndescription: "A test skill"\n---\n# Test skill'
    );
    const result = runValidator('__test-plugin__');
    assert.strictEqual(result.exitCode, 0);
  });

  it('should validate real plugins successfully', () => {
    const result = runValidator('adhoc');
    assert.strictEqual(result.exitCode, 0);
    assert.ok(result.output.includes('✅ adhoc'));
  });

  it('should validate ironarch successfully', () => {
    const result = runValidator('ironarch');
    assert.strictEqual(result.exitCode, 0);
    assert.ok(result.output.includes('✅ ironarch'));
  });
});
