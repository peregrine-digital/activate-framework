const vscode = require('vscode');
const https = require('https');

/**
 * Fetch a file from a GitHub repo via the raw.githubusercontent.com URL.
 * Returns the content as a string, or null on failure.
 */
function fetchRaw(owner, repo, branch, filePath) {
  const url = `https://raw.githubusercontent.com/${owner}/${repo}/${branch}/${filePath}`;
  return new Promise((resolve) => {
    https.get(url, { headers: { 'User-Agent': 'peregrine-activate-vscode' } }, (res) => {
      if (res.statusCode === 301 || res.statusCode === 302) {
        // Follow redirect
        https.get(res.headers.location, { headers: { 'User-Agent': 'peregrine-activate-vscode' } }, (res2) => {
          if (res2.statusCode !== 200) { resolve(null); return; }
          let data = '';
          res2.on('data', (chunk) => { data += chunk; });
          res2.on('end', () => resolve(data));
          res2.on('error', () => resolve(null));
        }).on('error', () => resolve(null));
        return;
      }
      if (res.statusCode !== 200) { resolve(null); return; }
      let data = '';
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => resolve(data));
      res.on('error', () => resolve(null));
    }).on('error', () => resolve(null));
  });
}

/**
 * Fetch a file's contents as a Buffer from GitHub.
 * Returns null on failure.
 */
async function fetchFileBuffer(owner, repo, branch, filePath) {
  const content = await fetchRaw(owner, repo, branch, filePath);
  if (content === null) return null;
  return Buffer.from(content, 'utf8');
}

/**
 * Fetch the manifest.json from the GitHub repo.
 * Returns the parsed manifest, or null on failure.
 */
async function fetchManifest(owner, repo, branch, pluginPath) {
  const content = await fetchRaw(owner, repo, branch, `${pluginPath}/manifest.json`);
  if (!content) return null;
  try {
    return JSON.parse(content);
  } catch {
    return null;
  }
}

/**
 * Fetch the .activate-version from the GitHub repo.
 * Returns the version string, or null on failure.
 */
async function fetchVersion(owner, repo, branch, pluginPath) {
  const content = await fetchRaw(owner, repo, branch, `${pluginPath}/.activate-version`);
  if (!content) return null;
  return content.trim();
}

/**
 * Fetch all manifest files from GitHub and write them to the target directory.
 * Returns { installed, skipped, version } or null if manifest fetch failed.
 */
async function fetchAndSync(context, { owner, repo, branch, pluginPath, tier, selectFiles, targetRoot }) {
  const manifest = await fetchManifest(owner, repo, branch, pluginPath);
  if (!manifest) return null;

  const version = await fetchVersion(owner, repo, branch, pluginPath);
  if (!version) return null;

  const files = selectFiles(manifest.files, tier);
  const installed = [];
  const skipped = [];

  for (const f of files) {
    const content = await fetchFileBuffer(owner, repo, branch, `${pluginPath}/${f.src}`);
    if (!content) {
      skipped.push(f.dest);
      continue;
    }

    const dest = vscode.Uri.joinPath(targetRoot, '.github', f.dest);
    await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(dest, '..'));
    await vscode.workspace.fs.writeFile(dest, content);
    installed.push(f.dest);
  }

  // Fetch AGENTS.md to root
  const agentsContent = await fetchFileBuffer(owner, repo, branch, `${pluginPath}/AGENTS.md`);
  if (agentsContent) {
    const agentsDest = vscode.Uri.joinPath(targetRoot, 'AGENTS.md');
    await vscode.workspace.fs.writeFile(agentsDest, agentsContent);
  }

  // Write version
  const versionUri = vscode.Uri.joinPath(targetRoot, '.activate-version');
  await vscode.workspace.fs.writeFile(versionUri, Buffer.from(version + '\n'));

  return { installed, skipped, version };
}

module.exports = { fetchRaw, fetchManifest, fetchVersion, fetchAndSync };
