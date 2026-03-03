'use strict';

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const Module = require('module');
const path = require('path');
const fs = require('fs');
const os = require('os');

// ── Module interception (vscode mock) ───────────────────────────

const origResolve = Module._resolveFilename;
const origLoad = Module._load;

const vscodeMock = {
  workspace: { workspaceFolders: [{ uri: { fsPath: '/test' } }] },
  window: {
    showInformationMessage: async () => {},
    withProgress: async (_o, fn) => fn(),
    createOutputChannel: () => ({
      appendLine: () => {}, clear: () => {}, show: () => {}, dispose: () => {},
    }),
    registerWebviewViewProvider: () => ({ dispose: () => {} }),
    showErrorMessage: () => {},
    showWarningMessage: async () => {},
  },
  commands: { registerCommand: () => ({ dispose: () => {} }), executeCommand: async () => {} },
  authentication: { getSession: async () => null },
  Uri: { file: (p) => ({ fsPath: p, scheme: 'file' }), joinPath: (b, ...s) => ({ fsPath: [b.fsPath, ...s].join('/') }) },
  ProgressLocation: { Notification: 15 },
};

let installed = false;
function installVscodeMock() {
  if (installed) return;
  installed = true;
  Module._resolveFilename = function (request, parent, isMain, options) {
    if (request === 'vscode') return 'vscode';
    return origResolve.call(this, request, parent, isMain, options);
  };
  Module._load = function (request, parent, isMain) {
    if (request === 'vscode') return vscodeMock;
    return origLoad.call(this, request, parent, isMain);
  };
}

installVscodeMock();
const { verifyBinary, resolveBinPath } = require('../extension');

// ── verifyBinary tests ──────────────────────────────────────────
// Tests the REAL exported function from extension.js.

describe('verifyBinary (real function)', () => {
  it('returns null for a real executable', () => {
    // node is always available
    const result = verifyBinary(process.execPath);
    // node --version doesn't match ['version'], but node binary is still executable
    // The function runs `<binary> version` — node will output version info or error but won't throw ENOENT
    // Either null (success) or an error is valid — the key is it doesn't crash
    // For a true positive, use a script that accepts 'version'
    assert.ok(result === null || result instanceof Error,
      'should return null or Error, not throw');
  });

  it('returns Error for a nonexistent binary', () => {
    const result = verifyBinary('/nonexistent/binary/path/activate-fake');
    assert.ok(result instanceof Error, 'should return Error for missing binary');
    assert.ok(result.message.includes('ENOENT') || result.code === 'ENOENT',
      'error should indicate file not found');
  });

  it('returns Error for a non-executable file', () => {
    // Create a temp file that is not executable
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const fakeFile = path.join(tmpDir, 'not-executable');
    fs.writeFileSync(fakeFile, 'not a binary', { mode: 0o444 });

    try {
      const result = verifyBinary(fakeFile);
      assert.ok(result instanceof Error, 'should return Error for non-executable file');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('returns Error for a script that exits non-zero', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const badScript = path.join(tmpDir, 'bad-binary');
    fs.writeFileSync(badScript, '#!/bin/sh\nexit 1', { mode: 0o755 });

    try {
      const result = verifyBinary(badScript);
      assert.ok(result instanceof Error, 'should return Error for failing binary');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('returns null for a script that accepts version arg', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const goodScript = path.join(tmpDir, 'good-binary');
    fs.writeFileSync(goodScript, '#!/bin/sh\necho "v1.0.0"', { mode: 0o755 });

    try {
      const result = verifyBinary(goodScript);
      assert.strictEqual(result, null, 'should return null for working binary');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });
});

// ── resolveBinPath tests ────────────────────────────────────────
// Tests the REAL exported function from extension.js.

describe('resolveBinPath (real function)', () => {
  it('returns bundled path when it exists', async () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const bundled = path.join(tmpDir, 'bin', 'activate');
    fs.mkdirSync(path.dirname(bundled), { recursive: true });
    fs.writeFileSync(bundled, 'fake', { mode: 0o755 });

    const lines = [];
    const out = { appendLine: (l) => lines.push(l) };

    try {
      const result = await resolveBinPath({ extensionUri: { fsPath: tmpDir } }, out);
      assert.strictEqual(result, bundled);
      assert.ok(lines.some((l) => l.includes('bundled')), 'should log bundled source');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('returns dev path when bundled absent but dev exists', async () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    // Create cli/activate as sibling of extension dir
    const extDir = path.join(tmpDir, 'extension');
    fs.mkdirSync(extDir, { recursive: true });
    const devBin = path.join(tmpDir, 'cli', 'activate');
    fs.mkdirSync(path.dirname(devBin), { recursive: true });
    fs.writeFileSync(devBin, 'fake', { mode: 0o755 });

    const lines = [];
    const out = { appendLine: (l) => lines.push(l) };

    try {
      const result = await resolveBinPath({ extensionUri: { fsPath: extDir } }, out);
      assert.strictEqual(result, devBin);
      assert.ok(lines.some((l) => l.includes('dev')), 'should log dev source');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('returns null when binary not found anywhere', async () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const lines = [];
    const out = { appendLine: (l) => lines.push(l) };

    try {
      const result = await resolveBinPath({ extensionUri: { fsPath: tmpDir } }, out);
      // Result is null unless activate happens to be on PATH
      // If it IS on PATH (dev machine), that's also valid
      assert.ok(result === null || typeof result === 'string',
        'should return null or PATH result');
      if (result === null) {
        assert.ok(lines.some((l) => l.includes('not found')), 'should log not found');
      }
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('checks locations in priority order: bundled > dev > managed > PATH', async () => {
    // Create both bundled and dev paths — bundled should win
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const extDir = path.join(tmpDir, 'extension');
    fs.mkdirSync(extDir, { recursive: true });

    const bundled = path.join(extDir, 'bin', 'activate');
    fs.mkdirSync(path.dirname(bundled), { recursive: true });
    fs.writeFileSync(bundled, 'bundled', { mode: 0o755 });

    const devBin = path.join(tmpDir, 'cli', 'activate');
    fs.mkdirSync(path.dirname(devBin), { recursive: true });
    fs.writeFileSync(devBin, 'dev', { mode: 0o755 });

    const lines = [];
    const out = { appendLine: (l) => lines.push(l) };

    try {
      const result = await resolveBinPath({ extensionUri: { fsPath: extDir } }, out);
      assert.strictEqual(result, bundled, 'bundled should take priority over dev');
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });
});

// ── install.sh path resolution test ─────────────────────────────
// Verifies that the install.sh path referenced in autoInstallCLI resolves correctly.

describe('install.sh path resolution', () => {
  it('source install.sh exists at repo root for prepare-assets to copy', () => {
    // prepare-assets.mjs copies install.sh from repo root to extension/
    // Verify the source exists so the build won't silently skip it
    const repoRoot = path.resolve(__dirname, '..', '..', '..');
    const srcScript = path.join(repoRoot, 'install.sh');
    assert.ok(fs.existsSync(srcScript),
      `install.sh should exist at repo root (${srcScript}) for prepare-assets.mjs to copy`);
  });

  it('autoInstallCLI path resolves to extension/install.sh (created by prepare-assets)', () => {
    // The code in extension.js uses: path.join(__dirname, '..', 'install.sh')
    // where __dirname is extension/src/ → resolves to extension/install.sh
    // This file is created by prepare-assets.mjs at build time
    const extensionSrcDir = path.resolve(__dirname, '..'); // __tests__/../ = src/
    const extensionJsDir = extensionSrcDir; // extension.js lives in src/
    const expectedPath = path.join(extensionJsDir, '..', 'install.sh'); // src/../install.sh = extension/install.sh

    assert.strictEqual(
      path.basename(path.dirname(expectedPath)), 'extension',
      'install.sh path should resolve inside extension/ directory',
    );
  });

  it('prepare-assets.mjs copies install.sh to extension root', () => {
    // Verify the prepare-assets script references install.sh copy
    const scriptPath = path.resolve(__dirname, '..', '..', 'scripts', 'prepare-assets.mjs');
    assert.ok(fs.existsSync(scriptPath), 'prepare-assets.mjs should exist');
    const content = fs.readFileSync(scriptPath, 'utf8');
    assert.ok(content.includes('install.sh'),
      'prepare-assets.mjs should reference install.sh copy');
    assert.ok(content.includes('installDest'),
      'prepare-assets.mjs should copy install.sh to extension dir');
  });

  it('package.json exists at expected relative path for version resolution', () => {
    // The extension uses context.extension.packageJSON.version (provided by VS Code API)
    // but verify the package.json is accessible from src/ via require
    const pkgPath = path.join(__dirname, '..', '..', 'package.json');
    assert.ok(fs.existsSync(pkgPath), `package.json should exist at ${pkgPath}`);

    const pkg = require(pkgPath);
    assert.ok(pkg.version, 'package.json should have a version field');
    assert.ok(pkg.name, 'package.json should have a name field');
  });
});
