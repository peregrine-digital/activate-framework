/**
 * GitHub fetcher for remote manifest and file downloads.
 * Supports both public repos (raw.githubusercontent.com) and private repos
 * (via VS Code GitHub authentication or GITHUB_TOKEN env var).
 */
const vscode = require('vscode');
const https = require('https');

const DEFAULT_REPO = 'peregrine-digital/activate-framework';
const DEFAULT_BRANCH = 'main';
const API_BASE = 'https://api.github.com';
const RAW_BASE = 'https://raw.githubusercontent.com';

// Known manifest IDs (used when we can't list directory contents)
const KNOWN_MANIFESTS = ['activate-framework', 'ironarch'];

// ── Configuration ───────────────────────────────────────────────

/**
 * Get the configured source settings.
 * @returns {{source: string, repo: string, branch: string}}
 */
function getSourceConfig() {
  const config = vscode.workspace.getConfiguration('activate-framework');
  return {
    source: config.get('source', 'bundled'),
    repo: config.get('remoteRepo', DEFAULT_REPO),
    branch: config.get('remoteBranch', DEFAULT_BRANCH),
  };
}

/**
 * Check if remote mode is enabled.
 * @returns {boolean}
 */
function isRemoteMode() {
  return getSourceConfig().source === 'remote';
}

/**
 * Get a GitHub token from VS Code's authentication provider.
 * Falls back to GITHUB_TOKEN environment variable.
 * @returns {Promise<string|undefined>}
 */
async function getGitHubToken() {
  // Try VS Code's built-in GitHub authentication
  try {
    const session = await vscode.authentication.getSession('github', ['repo'], { createIfNone: false });
    if (session?.accessToken) {
      return session.accessToken;
    }
  } catch {
    // Authentication not available or user declined
  }
  // Fall back to environment variable
  return process.env.GITHUB_TOKEN;
}

// ── Core fetch functions ────────────────────────────────────────

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
 * Fetch a file from GitHub with authentication support.
 * Uses GitHub API with token for private repos, falls back to raw.githubusercontent.com.
 * @param {string} filePath - Path relative to repo root
 * @param {object} [options]
 * @returns {Promise<string|null>} File contents or null on failure
 */
async function fetchFileAuth(filePath, options = {}) {
  const cfg = getSourceConfig();
  const targetRepo = options.repo || cfg.repo;
  const targetBranch = options.branch || cfg.branch;
  const [owner, repo] = targetRepo.split('/');
  const token = await getGitHubToken();

  if (token) {
    // Use GitHub API for authenticated access
    const url = `${API_BASE}/repos/${targetRepo}/contents/${filePath}?ref=${targetBranch}`;
    return new Promise((resolve) => {
      const req = https.get(url, {
        headers: {
          'User-Agent': 'peregrine-activate-vscode',
          Authorization: `Bearer ${token}`,
          Accept: 'application/vnd.github.raw+json',
        },
      }, (res) => {
        if (res.statusCode !== 200) { resolve(null); return; }
        let data = '';
        res.on('data', (chunk) => { data += chunk; });
        res.on('end', () => resolve(data));
        res.on('error', () => resolve(null));
      });
      req.on('error', () => resolve(null));
    });
  }

  // Fallback to raw.githubusercontent.com (public repos only)
  return fetchRaw(owner, repo, targetBranch, filePath);
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
 * Now supports fetching a specific manifest from the manifests/ directory.
 * Returns { installed, skipped, version } or null if manifest fetch failed.
 */
async function fetchAndSync(context, { owner, repo, branch, pluginPath, tier, selectFiles, targetRoot, manifestId }) {
  let manifest;
  let version;

  // Try multi-manifest path first
  if (manifestId) {
    const manifestContent = await fetchRaw(owner, repo, branch, `${pluginPath}/manifests/${manifestId}.json`);
    if (manifestContent) {
      try {
        manifest = JSON.parse(manifestContent);
        version = manifest.version || 'unknown';
      } catch { /* fall through */ }
    }
  }

  // Fall back to legacy manifest.json
  if (!manifest) {
    manifest = await fetchManifest(owner, repo, branch, pluginPath);
    if (!manifest) return null;
    version = manifest.version || await fetchVersion(owner, repo, branch, pluginPath);
    if (!version) return null;
  }

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

  // Write version with manifest info
  const versionUri = vscode.Uri.joinPath(targetRoot, '.activate-version');
  const versionData = JSON.stringify({ manifest: manifestId || 'activate-framework', version });
  await vscode.workspace.fs.writeFile(versionUri, Buffer.from(versionData + '\n'));

  return { installed, skipped, version };
}

// ── Remote manifest discovery ───────────────────────────────────

/**
 * Fetch and parse a JSON file from GitHub with auth support.
 * @param {string} filePath
 * @param {object} [options]
 * @returns {Promise<object|null>}
 */
async function fetchJSONAuth(filePath, options) {
  const text = await fetchFileAuth(filePath, options);
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

/**
 * Discover all manifests from the remote repo.
 * Uses configuration settings unless overridden.
 * @param {object} [options]
 * @returns {Promise<Array<{id: string, name: string, description: string, version: string, basePath: string, files: Array, tiers: Array|undefined}>>}
 */
async function discoverRemoteManifests(options = {}) {
  // Try to fetch manifest index first
  try {
    const index = await fetchJSONAuth('manifests/index.json', options);
    if (index && Array.isArray(index.manifests)) {
      const results = [];
      for (const id of index.manifests) {
        const manifest = await loadRemoteManifest(id, options);
        if (manifest) results.push(manifest);
      }
      if (results.length > 0) return results;
    }
  } catch {
    // No index.json — fall back to known manifests
  }

  // Try known manifest names
  const results = [];
  for (const id of KNOWN_MANIFESTS) {
    const manifest = await loadRemoteManifest(id, options);
    if (manifest) results.push(manifest);
  }

  return results;
}

/**
 * Load a single manifest by ID from the remote repo.
 * @param {string} manifestId
 * @param {object} [options]
 * @returns {Promise<{id: string, name: string, description: string, version: string, basePath: string, files: Array, tiers: Array|undefined}|null>}
 */
async function loadRemoteManifest(manifestId, options = {}) {
  const data = await fetchJSONAuth(`manifests/${manifestId}.json`, options);
  if (!data) return null;
  return {
    id: manifestId,
    name: data.name || manifestId,
    description: data.description || '',
    version: data.version || 'unknown',
    basePath: data.basePath || '',
    files: data.files || [],
    tiers: data.tiers,
  };
}

/**
 * Fetch a file's content as a Buffer from the remote repo.
 * Uses configuration settings unless overridden.
 * @param {string} filePath - Path relative to repo root
 * @param {object} [options]
 * @returns {Promise<Buffer|null>}
 */
async function fetchFileAsBuffer(filePath, options) {
  const text = await fetchFileAuth(filePath, options);
  if (!text) return null;
  return Buffer.from(text, 'utf8');
}

module.exports = {
  // Config
  DEFAULT_REPO,
  DEFAULT_BRANCH,
  getSourceConfig,
  isRemoteMode,
  getGitHubToken,
  // Core fetch
  fetchRaw,
  fetchFileAuth,
  fetchFileAsBuffer,
  fetchJSONAuth,
  // Manifest discovery
  discoverRemoteManifests,
  loadRemoteManifest,
  // Legacy
  fetchManifest,
  fetchVersion,
  fetchAndSync,
};
