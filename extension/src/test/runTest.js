/**
 * VS Code integration test runner.
 *
 * Downloads a real VS Code instance and runs tests inside the
 * Extension Development Host. This catches issues that unit tests
 * miss — activation failures, command registration, webview loading, etc.
 *
 * Run: npm run test:integration
 */
const path = require('path');
const { runTests } = require('@vscode/test-electron');

async function main() {
  const extensionDevelopmentPath = path.resolve(__dirname, '..', '..');
  const extensionTestsPath = path.resolve(__dirname, 'integration', 'index.js');

  // Open the repo root as workspace so workspace-dependent commands work
  const testWorkspace = path.resolve(__dirname, '..', '..', '..', '..');

  await runTests({
    extensionDevelopmentPath,
    extensionTestsPath,
    launchArgs: [
      testWorkspace,
      '--disable-extensions',
    ],
  });
}

main().catch((err) => {
  console.error('Failed to run integration tests:', err);
  process.exit(1);
});
