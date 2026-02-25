/**
 * GitHub fetcher for remote manifest and file downloads.
 * Uses raw.githubusercontent.com for public repos, or GitHub API for private repos with token.
 */

const DEFAULT_REPO = 'peregrine-digital/activate-framework';
const DEFAULT_BRANCH = 'main';
const RAW_BASE = 'https://raw.githubusercontent.com';
const API_BASE = 'https://api.github.com';

/**
 * Fetch a file from GitHub.
 * Uses raw.githubusercontent.com for public access, GitHub API with token for private repos.
 * @param {string} filePath - Path relative to repo root
 * @param {object} [options]
 * @param {string} [options.repo] - GitHub repo (owner/repo)
 * @param {string} [options.branch] - Branch or tag name
 * @param {string} [options.token] - GitHub token for private repos (reads from GITHUB_TOKEN env if not provided)
 * @returns {Promise<string>} File contents as text
 */
export async function fetchFile(filePath, { repo = DEFAULT_REPO, branch = DEFAULT_BRANCH, token } = {}) {
  const authToken = token || process.env.GITHUB_TOKEN;

  if (authToken) {
    // Use GitHub API for authenticated access (works with private repos)
    const url = `${API_BASE}/repos/${repo}/contents/${filePath}?ref=${branch}`;
    const res = await fetch(url, {
      headers: {
        Authorization: `Bearer ${authToken}`,
        Accept: 'application/vnd.github.raw+json',
      },
    });
    if (!res.ok) {
      throw new Error(`Failed to fetch ${filePath}: ${res.status} ${res.statusText}`);
    }
    return res.text();
  }

  // Fallback to raw.githubusercontent.com (public repos only)
  const url = `${RAW_BASE}/${repo}/${branch}/${filePath}`;
  const res = await fetch(url);
  if (!res.ok) {
    if (res.status === 404) {
      throw new Error(`Failed to fetch ${filePath}: 404 Not Found (repo may be private — set GITHUB_TOKEN env var)`);
    }
    throw new Error(`Failed to fetch ${filePath}: ${res.status} ${res.statusText}`);
  }
  return res.text();
}

/**
 * Fetch and parse a JSON file from GitHub.
 * @param {string} filePath - Path relative to repo root
 * @param {object} [options]
 * @returns {Promise<object>}
 */
export async function fetchJSON(filePath, options) {
  const text = await fetchFile(filePath, options);
  return JSON.parse(text);
}

/**
 * Discover manifests from the remote manifests/ directory.
 * Since we can't list directory contents via raw.githubusercontent.com,
 * we fetch the known manifest index or try known manifest names.
 *
 * @param {object} [options]
 * @param {string} [options.repo]
 * @param {string} [options.branch]
 * @returns {Promise<Array<{id: string, name: string, description: string, version: string, basePath: string, files: Array}>>}
 */
export async function discoverRemoteManifests({ repo = DEFAULT_REPO, branch = DEFAULT_BRANCH } = {}) {
  // Try to fetch manifest index first (if it exists)
  try {
    const index = await fetchJSON('manifests/index.json', { repo, branch });
    if (Array.isArray(index.manifests)) {
      const results = [];
      for (const id of index.manifests) {
        const manifest = await loadRemoteManifest(id, { repo, branch });
        results.push(manifest);
      }
      return results;
    }
  } catch {
    // No index.json — fall back to known manifests
  }

  // Fall back to trying known manifest names
  const knownManifests = ['activate-framework', 'ironarch'];
  const results = [];

  for (const id of knownManifests) {
    try {
      const manifest = await loadRemoteManifest(id, { repo, branch });
      results.push(manifest);
    } catch {
      // Manifest doesn't exist — skip
    }
  }

  if (results.length === 0) {
    throw new Error(`No manifests found in ${repo}@${branch}`);
  }

  return results;
}

/**
 * Load a single manifest by ID from the remote repo.
 * @param {string} manifestId
 * @param {object} [options]
 * @returns {Promise<{id: string, name: string, description: string, version: string, basePath: string, files: Array}>}
 */
export async function loadRemoteManifest(manifestId, { repo = DEFAULT_REPO, branch = DEFAULT_BRANCH } = {}) {
  const data = await fetchJSON(`manifests/${manifestId}.json`, { repo, branch });
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
 * Install files by fetching from GitHub rather than copying from local disk.
 * @param {object} options
 * @param {Array} options.files - Files to install (with src/dest)
 * @param {string} options.basePath - Base path prefix for source files
 * @param {string} options.targetDir - Local destination directory
 * @param {string} options.version - Manifest version
 * @param {string} options.manifestId - Manifest identifier
 * @param {string} [options.repo] - GitHub repo
 * @param {string} [options.branch] - Branch/tag
 */
export async function installFilesFromRemote({
  files,
  basePath,
  targetDir,
  version,
  manifestId,
  repo = DEFAULT_REPO,
  branch = DEFAULT_BRANCH,
}) {
  const { mkdir, writeFile } = await import('node:fs/promises');
  const path = await import('node:path');

  for (const f of files) {
    // Build the source path: basePath + src
    const srcPath = basePath ? `${basePath}/${f.src}` : f.src;
    const destPath = path.join(targetDir, f.dest);

    try {
      const content = await fetchFile(srcPath, { repo, branch });
      await mkdir(path.dirname(destPath), { recursive: true });
      await writeFile(destPath, content);
      console.log(`  ✓  ${f.dest}`);
    } catch (err) {
      console.error(`  ✗  ${f.dest}: ${err.message}`);
    }
  }

  // Write version marker
  const versionFile = path.join(targetDir, '.github', '.activate-version');
  await mkdir(path.dirname(versionFile), { recursive: true });
  await writeFile(versionFile, JSON.stringify({ manifest: manifestId, version, remote: `${repo}@${branch}` }, null, 2));
}

export { DEFAULT_REPO, DEFAULT_BRANCH };
