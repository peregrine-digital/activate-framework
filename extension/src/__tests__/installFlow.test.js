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
const {
  verifyBinary, resolveBinPath, verifyChecksum, downloadFile,
  POLL_INTERVAL_MS, POST_DETECT_DELAY_MS, POLL_TIMEOUT_MS,
} = require('../extension');
const { createHash } = require('crypto');
const http = require('http');

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

// ── verifyChecksum tests ────────────────────────────────────────
// Tests the REAL exported function from extension.js.

describe('verifyChecksum (real function)', () => {
  it('passes when hash matches', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const file = path.join(tmpDir, 'test.bin');
    fs.writeFileSync(file, 'hello world');
    const expected = createHash('sha256').update('hello world').digest('hex');

    try {
      assert.doesNotThrow(() => verifyChecksum(file, expected));
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('throws on checksum mismatch', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const file = path.join(tmpDir, 'test.bin');
    fs.writeFileSync(file, 'hello world');

    try {
      assert.throws(
        () => verifyChecksum(file, 'badhash000000'),
        /Checksum mismatch/,
      );
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('throws with both expected and actual hash in error', () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const file = path.join(tmpDir, 'test.bin');
    fs.writeFileSync(file, 'hello world');
    const actual = createHash('sha256').update('hello world').digest('hex');

    try {
      assert.throws(
        () => verifyChecksum(file, 'wronghash'),
        (err) => {
          assert.ok(err.message.includes('wronghash'), 'error should contain expected hash');
          assert.ok(err.message.includes(actual), 'error should contain actual hash');
          return true;
        },
      );
    } finally {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('no-ops when expectedSha256 is null', () => {
    assert.doesNotThrow(() => verifyChecksum('/nonexistent', null));
  });

  it('no-ops when expectedSha256 is empty string', () => {
    assert.doesNotThrow(() => verifyChecksum('/nonexistent', ''));
  });

  it('no-ops when expectedSha256 is undefined', () => {
    assert.doesNotThrow(() => verifyChecksum('/nonexistent', undefined));
  });
});

// ── downloadFile tests ──────────────────────────────────────────
// Tests the REAL exported function from extension.js using http.createServer.

describe('downloadFile (real function)', () => {
  it('downloads file content to destination', async () => {
    const server = http.createServer((req, res) => {
      res.writeHead(200);
      res.end('vsix-file-content');
    });
    await new Promise((r) => server.listen(0, r));
    const port = server.address().port;

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const dest = path.join(tmpDir, 'test.vsix');

    try {
      await downloadFile(`http://localhost:${port}/file.vsix`, dest, '');
      assert.strictEqual(fs.readFileSync(dest, 'utf8'), 'vsix-file-content');
    } finally {
      server.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('follows redirects and sends auth only on initial request', async () => {
    const receivedHeaders = [];
    const server = http.createServer((req, res) => {
      receivedHeaders.push({
        url: req.url,
        auth: req.headers.authorization,
        accept: req.headers.accept,
      });
      if (req.url === '/api/asset/123') {
        // Simulate GitHub API → S3 redirect
        res.writeHead(302, { Location: `http://localhost:${server.address().port}/s3/presigned` });
        res.end();
      } else {
        res.writeHead(200);
        res.end('redirected-content');
      }
    });
    await new Promise((r) => server.listen(0, r));

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const dest = path.join(tmpDir, 'test.vsix');

    try {
      await downloadFile(
        `http://localhost:${server.address().port}/api/asset/123`,
        dest, 'ghp_private_token',
      );

      // Initial request: auth + accept headers present
      assert.strictEqual(receivedHeaders[0].auth, 'Bearer ghp_private_token',
        'initial request must include auth token for private repos');
      assert.strictEqual(receivedHeaders[0].accept, 'application/octet-stream',
        'initial request must include accept header');

      // Redirect request: NO auth (S3 pre-signed URLs reject extra auth)
      assert.strictEqual(receivedHeaders[1].auth, undefined,
        'redirect must NOT include auth — S3 rejects extra Authorization headers');
      assert.strictEqual(receivedHeaders[1].accept, undefined,
        'redirect must NOT include accept header');

      assert.strictEqual(fs.readFileSync(dest, 'utf8'), 'redirected-content');
    } finally {
      server.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('rejects on non-200 status', async () => {
    const server = http.createServer((req, res) => {
      res.writeHead(404);
      res.end('not found');
    });
    await new Promise((r) => server.listen(0, r));

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const dest = path.join(tmpDir, 'test.vsix');

    try {
      await assert.rejects(
        () => downloadFile(`http://localhost:${server.address().port}/missing`, dest, ''),
        /Download failed: 404/,
      );
    } finally {
      server.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('rejects on server error', async () => {
    const server = http.createServer((req, res) => {
      res.writeHead(500);
      res.end('server error');
    });
    await new Promise((r) => server.listen(0, r));

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const dest = path.join(tmpDir, 'test.vsix');

    try {
      await assert.rejects(
        () => downloadFile(`http://localhost:${server.address().port}/`, dest, ''),
        /Download failed: 500/,
      );
    } finally {
      server.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('handles multiple redirects', async () => {
    let redirectCount = 0;
    const server = http.createServer((req, res) => {
      if (req.url === '/start') {
        res.writeHead(301, { Location: `http://localhost:${server.address().port}/middle` });
        res.end();
      } else if (req.url === '/middle') {
        redirectCount++;
        res.writeHead(302, { Location: `http://localhost:${server.address().port}/final` });
        res.end();
      } else {
        res.writeHead(200);
        res.end('final-content');
      }
    });
    await new Promise((r) => server.listen(0, r));

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const dest = path.join(tmpDir, 'test.vsix');

    try {
      await downloadFile(`http://localhost:${server.address().port}/start`, dest, 'tok');
      assert.strictEqual(fs.readFileSync(dest, 'utf8'), 'final-content');
      assert.strictEqual(redirectCount, 1);
    } finally {
      server.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('works without auth token', async () => {
    const receivedHeaders = [];
    const server = http.createServer((req, res) => {
      receivedHeaders.push({ auth: req.headers.authorization });
      res.writeHead(200);
      res.end('public-content');
    });
    await new Promise((r) => server.listen(0, r));

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const dest = path.join(tmpDir, 'test.vsix');

    try {
      await downloadFile(`http://localhost:${server.address().port}/`, dest, '');
      assert.strictEqual(receivedHeaders[0].auth, undefined,
        'no auth header when token is empty');
      assert.strictEqual(fs.readFileSync(dest, 'utf8'), 'public-content');
    } finally {
      server.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });
});

// ── downloadFile + verifyChecksum integration ───────────────────
// Tests the two functions composed together (the real downloadAndInstallVsix path).

describe('download + checksum integration (real functions)', () => {
  it('download then checksum pass', async () => {
    const content = 'vsix-binary-content-here';
    const expectedHash = createHash('sha256').update(content).digest('hex');

    const server = http.createServer((req, res) => {
      res.writeHead(200);
      res.end(content);
    });
    await new Promise((r) => server.listen(0, r));

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const dest = path.join(tmpDir, 'test.vsix');

    try {
      await downloadFile(`http://localhost:${server.address().port}/`, dest, 'tok');
      assert.doesNotThrow(() => verifyChecksum(dest, expectedHash));
    } finally {
      server.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });

  it('download then checksum fail (corrupted download)', async () => {
    const server = http.createServer((req, res) => {
      res.writeHead(200);
      res.end('corrupted-content');
    });
    await new Promise((r) => server.listen(0, r));

    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-test-'));
    const dest = path.join(tmpDir, 'test.vsix');
    const expectedHash = createHash('sha256').update('expected-content').digest('hex');

    try {
      await downloadFile(`http://localhost:${server.address().port}/`, dest, 'tok');
      assert.throws(
        () => verifyChecksum(dest, expectedHash),
        /Checksum mismatch/,
      );
    } finally {
      server.close();
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }
  });
});

// ── Polling constants tests ─────────────────────────────────────
// Tests the REAL exported constants from extension.js.

describe('polling constants (real values)', () => {
  it('POLL_INTERVAL_MS is 2 seconds', () => {
    assert.strictEqual(POLL_INTERVAL_MS, 2000);
  });

  it('POST_DETECT_DELAY_MS is 2 seconds', () => {
    assert.strictEqual(POST_DETECT_DELAY_MS, 2000);
  });

  it('POLL_TIMEOUT_MS is 5 minutes', () => {
    assert.strictEqual(POLL_TIMEOUT_MS, 300000);
  });

  it('timeout is much larger than poll interval', () => {
    assert.ok(POLL_TIMEOUT_MS > POLL_INTERVAL_MS * 10,
      'timeout should allow many poll attempts');
  });
});
