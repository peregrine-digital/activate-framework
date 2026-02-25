import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { mkdir, writeFile, readFile, rm } from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';

// We test the internal merge + read/write logic by importing the module
// and pointing it at a temp directory instead of ~/.activate.
// Since the module exports functions that take projectDir, we can test
// the project-level functions directly.
import {
  resolveConfig,
  readProjectConfig,
  writeProjectConfig,
  setFileOverride,
  setSkippedVersion,
  clearSkippedVersion,
  ensureGitExclude,
  writeGlobalConfig,
  readGlobalConfig,
  GLOBAL_CONFIG_PATH,
  PROJECT_CONFIG_FILENAME,
} from '../config.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/** Create a temp directory for each test */
async function makeTempDir() {
  const dir = path.join(os.tmpdir(), `activate-config-test-${Date.now()}-${Math.random().toString(36).slice(2)}`);
  await mkdir(dir, { recursive: true });
  return dir;
}

describe('config.mjs — project config', () => {
  let tmpDir;

  beforeEach(async () => {
    tmpDir = await makeTempDir();
  });

  afterEach(async () => {
    await rm(tmpDir, { recursive: true, force: true });
  });

  it('readProjectConfig returns null when no config exists', async () => {
    const result = await readProjectConfig(tmpDir);
    assert.equal(result, null);
  });

  it('writeProjectConfig creates and reads back config', async () => {
    await writeProjectConfig(tmpDir, { manifest: 'test-manifest', tier: 'advanced' });

    const result = await readProjectConfig(tmpDir);
    assert.equal(result.manifest, 'test-manifest');
    assert.equal(result.tier, 'advanced');
  });

  it('writeProjectConfig merges with existing config', async () => {
    await writeProjectConfig(tmpDir, { manifest: 'first', tier: 'standard' });
    await writeProjectConfig(tmpDir, { tier: 'advanced' });

    const result = await readProjectConfig(tmpDir);
    assert.equal(result.manifest, 'first');
    assert.equal(result.tier, 'advanced');
  });

  it('writeProjectConfig replaces fileOverrides when provided', async () => {
    await writeProjectConfig(tmpDir, {
      fileOverrides: { 'a.md': 'pinned' },
    });
    await writeProjectConfig(tmpDir, {
      fileOverrides: { 'b.md': 'excluded' },
    });

    const result = await readProjectConfig(tmpDir);
    // Second call replaces — only b.md should exist
    assert.equal(result.fileOverrides['a.md'], undefined);
    assert.equal(result.fileOverrides['b.md'], 'excluded');
  });
});

describe('config.mjs — resolveConfig', () => {
  let tmpDir;

  beforeEach(async () => {
    tmpDir = await makeTempDir();
  });

  afterEach(async () => {
    await rm(tmpDir, { recursive: true, force: true });
  });

  it('returns defaults when no config exists', async () => {
    const cfg = await resolveConfig(tmpDir);
    assert.equal(cfg.manifest, 'activate-framework');
    assert.equal(cfg.tier, 'standard');
    assert.deepEqual(cfg.fileOverrides, {});
    assert.deepEqual(cfg.skippedVersions, {});
  });

  it('project config overrides defaults', async () => {
    await writeProjectConfig(tmpDir, { manifest: 'custom', tier: 'minimal' });
    const cfg = await resolveConfig(tmpDir);
    assert.equal(cfg.manifest, 'custom');
    assert.equal(cfg.tier, 'minimal');
  });

  it('explicit overrides beat project config', async () => {
    await writeProjectConfig(tmpDir, { manifest: 'custom', tier: 'minimal' });
    const cfg = await resolveConfig(tmpDir, { tier: 'advanced' });
    assert.equal(cfg.manifest, 'custom');
    assert.equal(cfg.tier, 'advanced');
  });

  it('merges fileOverrides from all layers', async () => {
    await writeProjectConfig(tmpDir, {
      fileOverrides: { 'a.md': 'pinned' },
    });
    const cfg = await resolveConfig(tmpDir, {
      fileOverrides: { 'b.md': 'excluded' },
    });
    assert.equal(cfg.fileOverrides['a.md'], 'pinned');
    assert.equal(cfg.fileOverrides['b.md'], 'excluded');
  });
});

describe('config.mjs — setFileOverride', () => {
  let tmpDir;

  beforeEach(async () => {
    tmpDir = await makeTempDir();
  });

  afterEach(async () => {
    await rm(tmpDir, { recursive: true, force: true });
  });

  it('sets a file override to pinned', async () => {
    await setFileOverride(tmpDir, 'instructions/general.md', 'pinned');
    const cfg = await readProjectConfig(tmpDir);
    assert.equal(cfg.fileOverrides['instructions/general.md'], 'pinned');
  });

  it('sets a file override to excluded', async () => {
    await setFileOverride(tmpDir, 'skills/secret.md', 'excluded');
    const cfg = await readProjectConfig(tmpDir);
    assert.equal(cfg.fileOverrides['skills/secret.md'], 'excluded');
  });

  it('clears a file override when set to null', async () => {
    await setFileOverride(tmpDir, 'a.md', 'pinned');
    await setFileOverride(tmpDir, 'a.md', null);
    const cfg = await readProjectConfig(tmpDir);
    assert.equal(cfg.fileOverrides['a.md'], undefined);
  });
});

describe('config.mjs — skippedVersions', () => {
  let tmpDir;

  beforeEach(async () => {
    tmpDir = await makeTempDir();
  });

  afterEach(async () => {
    await rm(tmpDir, { recursive: true, force: true });
  });

  it('setSkippedVersion records the version', async () => {
    await setSkippedVersion(tmpDir, 'instructions/general.md', '0.5.0');
    const cfg = await readProjectConfig(tmpDir);
    assert.equal(cfg.skippedVersions['instructions/general.md'], '0.5.0');
  });

  it('clearSkippedVersion removes the version', async () => {
    await setSkippedVersion(tmpDir, 'a.md', '1.0.0');
    await clearSkippedVersion(tmpDir, 'a.md');
    const cfg = await readProjectConfig(tmpDir);
    assert.equal(cfg.skippedVersions['a.md'], undefined);
  });
});

describe('config.mjs — ensureGitExclude', () => {
  let tmpDir;

  beforeEach(async () => {
    tmpDir = await makeTempDir();
    // Set up a fake .git/info/ directory
    await mkdir(path.join(tmpDir, '.git', 'info'), { recursive: true });
    await writeFile(path.join(tmpDir, '.git', 'info', 'exclude'), '# default excludes\n');
  });

  afterEach(async () => {
    await rm(tmpDir, { recursive: true, force: true });
  });

  it('adds .activate.json to .git/info/exclude', async () => {
    await ensureGitExclude(tmpDir);
    const content = await readFile(path.join(tmpDir, '.git', 'info', 'exclude'), 'utf8');
    assert.ok(content.includes(PROJECT_CONFIG_FILENAME));
    assert.ok(content.includes('>>> Peregrine Activate config'));
  });

  it('is idempotent — calling twice does not duplicate', async () => {
    await ensureGitExclude(tmpDir);
    await ensureGitExclude(tmpDir);
    const content = await readFile(path.join(tmpDir, '.git', 'info', 'exclude'), 'utf8');
    const count = (content.match(/>>> Peregrine Activate config/g) || []).length;
    assert.equal(count, 1);
  });

  it('handles missing .git directory gracefully', async () => {
    const noGitDir = await makeTempDir();
    // Should not throw
    await ensureGitExclude(noGitDir);
    await rm(noGitDir, { recursive: true, force: true });
  });
});
