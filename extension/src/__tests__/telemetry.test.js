const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const Module = require('node:module');

// Provide a minimal vscode stub so telemetry.js can be loaded outside VS Code.
const originalResolve = Module._resolveFilename;
Module._resolveFilename = function (request, parent, isMain, options) {
  if (request === 'vscode') {
    // Return a sentinel; we override _load below.
    return '__vscode_stub__';
  }
  return originalResolve.call(this, request, parent, isMain, options);
};
const originalLoad = Module._load;
Module._load = function (request, parent, isMain) {
  if (request === '__vscode_stub__' || request === 'vscode') {
    return {
      workspace: {
        getConfiguration: () => ({
          get: (key, def) => def,
        }),
      },
      authentication: {},
      window: {
        showWarningMessage: () => {},
        showInformationMessage: () => {},
        onDidChangeWindowState: () => ({ dispose() {} }),
      },
      commands: { registerCommand: () => ({ dispose() {} }) },
      Uri: { file: (p) => ({ fsPath: p }) },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

// Pure/testable helpers from telemetry.js
const {
  formatDateKey,
  getArchiveDateStamp,
  shouldRunOnFocus,
  archiveActiveLogIfNeeded,
  extractPremiumQuota,
  FOCUS_THROTTLE_MS,
} = require('../telemetry');

// ── formatDateKey ───────────────────────────────────────────────

describe('formatDateKey', () => {
  it('formats a UTC date as YYYY-MM-DD', () => {
    const d = new Date('2026-02-24T14:30:00Z');
    assert.equal(formatDateKey(d), '2026-02-24');
  });

  it('pads single-digit month and day', () => {
    const d = new Date('2026-01-05T00:00:00Z');
    assert.equal(formatDateKey(d), '2026-01-05');
  });

  it('handles year boundary', () => {
    const d = new Date('2025-12-31T23:59:59Z');
    assert.equal(formatDateKey(d), '2025-12-31');
  });
});

// ── getArchiveDateStamp ─────────────────────────────────────────

describe('getArchiveDateStamp', () => {
  const now = new Date('2026-02-24T12:00:00Z');

  it('uses the parsed previous quota key when valid', () => {
    assert.equal(getArchiveDateStamp('2026-02-01T00:00:00Z', now), '2026-02-01');
  });

  it('falls back to now when prevQuotaKey is no-reset', () => {
    assert.equal(getArchiveDateStamp('no-reset', now), '2026-02-24');
  });

  it('falls back to now when prevQuotaKey is invalid', () => {
    assert.equal(getArchiveDateStamp('not-a-date', now), '2026-02-24');
  });
});

// ── shouldRunOnFocus ────────────────────────────────────────────

describe('shouldRunOnFocus', () => {
  it('returns true when no previous run', () => {
    assert.ok(shouldRunOnFocus(0, Date.now(), FOCUS_THROTTLE_MS));
  });

  it('returns false within throttle window', () => {
    const now = Date.now();
    assert.ok(!shouldRunOnFocus(now - 1000, now, FOCUS_THROTTLE_MS));
  });

  it('returns true after throttle window', () => {
    const now = Date.now();
    assert.ok(shouldRunOnFocus(now - FOCUS_THROTTLE_MS - 1, now, FOCUS_THROTTLE_MS));
  });

  it('returns true when exactly at throttle boundary', () => {
    const now = Date.now();
    assert.ok(shouldRunOnFocus(now - FOCUS_THROTTLE_MS, now, FOCUS_THROTTLE_MS));
  });
});

// ── extractPremiumQuota ─────────────────────────────────────────

describe('extractPremiumQuota', () => {
  it('extracts premium_interactions quota', () => {
    const data = {
      quota_snapshots: {
        a: { quota_id: 'premium_interactions', entitlement: 300, remaining: 142 },
        b: { quota_id: 'other_quota', entitlement: 100, remaining: 50 },
      },
    };
    const result = extractPremiumQuota(data);
    assert.equal(result.quota_id, 'premium_interactions');
    assert.equal(result.entitlement, 300);
    assert.equal(result.remaining, 142);
  });

  it('returns null when no snapshots', () => {
    assert.equal(extractPremiumQuota({}), null);
  });

  it('returns null when premium quota is unlimited', () => {
    const data = {
      quota_snapshots: {
        a: { quota_id: 'premium_interactions', unlimited: true },
      },
    };
    assert.equal(extractPremiumQuota(data), null);
  });

  it('returns null when premium_interactions not found', () => {
    const data = {
      quota_snapshots: {
        a: { quota_id: 'other_quota', entitlement: 100, remaining: 50 },
      },
    };
    assert.equal(extractPremiumQuota(data), null);
  });
});

// ── archiveActiveLogIfNeeded ────────────────────────────────────

describe('archiveActiveLogIfNeeded', () => {
  let tmpDir;

  beforeEach(async () => {
    tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'telemetry-test-'));
  });

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true });
  });

  it('does nothing when keys match', async () => {
    const activePath = path.join(tmpDir, 'copilot-telemetry.jsonl');
    await fs.writeFile(activePath, '{"test": true}\n');
    const result = await archiveActiveLogIfNeeded(tmpDir, 'same', 'same', new Date());
    assert.equal(result, null);
    // File should still be there
    await fs.stat(activePath);
  });

  it('does nothing when active log does not exist', async () => {
    const result = await archiveActiveLogIfNeeded(tmpDir, 'old', 'new', new Date());
    assert.equal(result, null);
  });

  it('archives active log when keys differ', async () => {
    const activePath = path.join(tmpDir, 'copilot-telemetry.jsonl');
    await fs.writeFile(activePath, '{"test": true}\n');

    const result = await archiveActiveLogIfNeeded(
      tmpDir,
      '2026-02-01T00:00:00Z',
      '2026-03-01T00:00:00Z',
      new Date('2026-02-24T12:00:00Z'),
    );

    assert.ok(result);
    assert.ok(result.includes('copilot-telemetry-2026-02-01.jsonl'));

    // Active log should be gone
    await assert.rejects(() => fs.stat(activePath));

    // Archive should exist with correct content
    const content = await fs.readFile(result, 'utf8');
    assert.equal(content, '{"test": true}\n');
  });

  it('uses current date when no valid previous reset key', async () => {
    const activePath = path.join(tmpDir, 'copilot-telemetry.jsonl');
    await fs.writeFile(activePath, 'data\n');
    const now = new Date('2026-02-24T12:00:00Z');

    const result = await archiveActiveLogIfNeeded(tmpDir, 'no-reset', 'new-key', now);

    assert.ok(result);
    assert.ok(result.includes('copilot-telemetry-2026-02-24.jsonl'));
  });
});
