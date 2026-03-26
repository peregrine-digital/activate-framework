/**
 * Integration test entry point — runs inside the Extension Development Host.
 *
 * These tests exercise the REAL VS Code API. No mocks.
 * If a test fails here, the extension is broken for real users.
 */
const vscode = require('vscode');
const assert = require('assert');

// Mocha-compatible test suite (VS Code test runner expects this)
function run() {
  return new Promise((resolve, reject) => {
    const failures = [];
    const passed = [];

    async function test(name, fn) {
      try {
        await fn();
        passed.push(name);
        console.log(`  ✓ ${name}`);
      } catch (err) {
        failures.push({ name, error: err });
        console.error(`  ✗ ${name}: ${err.message}`);
      }
    }

    (async () => {
      console.log('\n=== VS Code Integration Tests ===\n');

      // Wait for extension to activate
      await test('extension activates', async () => {
        const ext = vscode.extensions.getExtension('peregrine.peregrine-activate');
        assert.ok(ext, 'Extension not found — check publisher.name in package.json');

        // Activate if not already active
        if (!ext.isActive) {
          await ext.activate();
        }
        assert.ok(ext.isActive, 'Extension failed to activate');
      });

      await test('openFile command is registered', async () => {
        const commands = await vscode.commands.getCommands(true);
        assert.ok(
          commands.includes('activate-framework.openFile'),
          'activate-framework.openFile not registered',
        );
      });

      await test('all expected commands are registered', async () => {
        const commands = await vscode.commands.getCommands(true);
        const expected = [
          'activate-framework.openFile',
          'activate-framework.installFile',
          'activate-framework.uninstallFile',
          'activate-framework.diffFile',
          'activate-framework.changeTier',
          'activate-framework.changeManifest',
          'activate-framework.addToWorkspace',
          'activate-framework.removeFromWorkspace',
          'activate-framework.updateAll',
          'activate-framework.installCLI',
          'activate-framework.checkForUpdates',
        ];
        for (const cmd of expected) {
          assert.ok(commands.includes(cmd), `Missing command: ${cmd}`);
        }
      });

      await test('control panel view is registered', async () => {
        // The view should be declared in package.json
        const ext = vscode.extensions.getExtension('peregrine.peregrine-activate');
        const views = ext?.packageJSON?.contributes?.views;
        assert.ok(views, 'No views in package.json');
        const allViews = Object.values(views).flat();
        const panel = allViews.find((v) => v.id === 'activate-framework.controlPanel');
        assert.ok(panel, 'controlPanel view not declared');
      });

      await test('openFile command handles missing file gracefully', async () => {
        // Should not throw, just log/warn
        await vscode.commands.executeCommand('activate-framework.openFile', null);
        await vscode.commands.executeCommand('activate-framework.openFile', {});
        await vscode.commands.executeCommand('activate-framework.openFile', { dest: '' });
      });

      // Summary
      console.log(`\n  ${passed.length} passed, ${failures.length} failed\n`);

      if (failures.length > 0) {
        for (const f of failures) {
          console.error(`  FAIL: ${f.name}\n    ${f.error.message}\n`);
        }
        reject(new Error(`${failures.length} integration test(s) failed`));
      } else {
        resolve();
      }
    })();
  });
}

module.exports = { run };
