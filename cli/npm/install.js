/**
 * postinstall script — verifies the platform binary is available.
 *
 * npm/yarn/pnpm will have already installed the matching optionalDependency
 * for this platform. This script just validates it worked and prints a
 * helpful message if not.
 */

"use strict";

const path = require("path");
const fs = require("fs");

const PLATFORM_PACKAGES = {
  "darwin-arm64":  "@anthropic/activate-cli-darwin-arm64",
  "darwin-x64":    "@anthropic/activate-cli-darwin-x64",
  "linux-arm64":   "@anthropic/activate-cli-linux-arm64",
  "linux-x64":     "@anthropic/activate-cli-linux-x64",
  "win32-x64":     "@anthropic/activate-cli-win32-x64",
};

const platformKey = `${process.platform}-${process.arch}`;
const pkg = PLATFORM_PACKAGES[platformKey];

if (!pkg) {
  console.warn(
    `[activate] Warning: no prebuilt binary for ${platformKey}. ` +
    `You can build from source: cd cli && go build -o activate .`
  );
  process.exit(0);
}

try {
  const pkgDir = path.dirname(require.resolve(`${pkg}/package.json`));
  const ext = process.platform === "win32" ? ".exe" : "";
  const bin = path.join(pkgDir, `activate${ext}`);

  if (fs.existsSync(bin)) {
    // Make executable on Unix
    if (process.platform !== "win32") {
      fs.chmodSync(bin, 0o755);
    }
    console.log(`[activate] Binary ready: ${platformKey}`);
  } else {
    console.warn(
      `[activate] Warning: platform package ${pkg} installed but binary not found.\n` +
      `You can build from source: cd cli && go build -o activate .`
    );
  }
} catch {
  console.warn(
    `[activate] Warning: platform package ${pkg} not installed.\n` +
    `You can build from source: cd cli && go build -o activate .`
  );
}
