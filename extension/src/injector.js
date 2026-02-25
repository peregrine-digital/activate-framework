/**
 * Silent injector — copies Activate managed files directly into the
 * workspace's `.github/` directory and hides them from git using
 * `.git/info/exclude`.
 *
 * Design goals:
 * - Zero multi-root workspace — no restart, no workspace ceremony
 * - Invisible to git — managed paths added to `.git/info/exclude`
 * - Never overwrites pre-existing user files
 * - Tracks what was injected via `.github/.activate-installed.json`
 * - Clean uninstall removes only files we placed
 *
 * PIPELINE: Every file mutation flows through writeSidecar() /
 * deleteSidecar(), which automatically handle THREE concerns:
 *   1. Delete stale files from disk (old sidecar − new sidecar)
 *   2. Write / remove the sidecar JSON
 *   3. Sync `.git/info/exclude`
 *
 * No caller ever touches exclude or deletes managed files directly —
 * the sidecar is the single source of truth. Callers describe the
 * DESIRED end-state; the pipeline diffs and cleans up.
 */
const vscode = require('vscode');
const { selectFiles, parseManifestData, inferCategory } = require('./manifest');
const {
  isRemoteMode,
  getSourceConfig,
  fetchFileAsBuffer,
  fetchJSONAuth,
} = require('./fetcher');

// ── Constants ───────────────────────────────────────────────────

/** Sidecar file that records which files we injected */
const SIDECAR_REL = '.github/.activate-installed.json';

/** VS Code MCP server config file (workspace-level) */
const MCP_CONFIG_REL = '.vscode/mcp.json';

/** Marker comment used in .git/info/exclude */
const EXCLUDE_MARKER_START = '# >>> Peregrine Activate (managed — do not edit)';
const EXCLUDE_MARKER_END = '# <<< Peregrine Activate';

// ── Helpers ─────────────────────────────────────────────────────

/**
 * Get the primary workspace folder URI.
 * Returns undefined if no workspace is open.
 */
function getWorkspaceRoot() {
  return vscode.workspace.workspaceFolders?.[0]?.uri;
}

/**
 * Read the sidecar JSON from the workspace.
 * Returns null if not present.
 *
 * Shape: { manifest: string, version: string, tier: string, files: string[], mcpServers?: string[] }
 */
async function readSidecar(wsRoot) {
  const uri = vscode.Uri.joinPath(wsRoot, SIDECAR_REL);
  try {
    const raw = await vscode.workspace.fs.readFile(uri);
    return JSON.parse(Buffer.from(raw).toString('utf8'));
  } catch {
    return null;
  }
}

/**
 * Write the sidecar JSON to the workspace.
 *
 * This is the SINGLE COMMIT POINT for all injection mutations.
 * It automatically handles:
 *   1. Stale file cleanup — diffs old vs new file lists, deletes
 *      files that are no longer in the new set from disk.
 *   2. Sidecar persistence — writes the JSON tracking file.
 *   3. Git exclude sync — derives exclude paths from the new file list.
 *
 * No caller ever deletes managed files or touches exclude directly.
 */
async function writeSidecar(wsRoot, data) {
  // 1. Read old state to compute stale files
  const oldSidecar = await readSidecar(wsRoot);
  const oldFiles = new Set(oldSidecar?.files || []);
  const newFiles = new Set(data.files || []);

  // 2. Delete stale files (in old but not in new)
  for (const oldRel of oldFiles) {
    if (!newFiles.has(oldRel)) {
      try {
        await vscode.workspace.fs.delete(vscode.Uri.joinPath(wsRoot, oldRel));
      } catch {
        // file may already be gone
      }
    }
  }

  // 3. Write sidecar
  const uri = vscode.Uri.joinPath(wsRoot, SIDECAR_REL);
  await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(wsRoot, '.github'));
  await vscode.workspace.fs.writeFile(uri, Buffer.from(JSON.stringify(data, null, 2) + '\n'));

  // 4. Sync git exclude
  const excludePaths = [SIDECAR_REL, ...(data.files || [])];
  await _syncGitExclude(wsRoot, excludePaths);
}

/**
 * Delete the sidecar and clean up everything it tracks.
 *
 * Automatically handles:
 *   1. Deletes ALL tracked files from disk.
 *   2. Removes managed MCP servers from .vscode/mcp.json.
 *   3. Removes the sidecar JSON.
 *   4. Removes the git exclude block.
 *
 * Counterpart to writeSidecar() — between the two, no caller
 * ever needs to delete managed files directly.
 */
async function deleteSidecar(wsRoot) {
  // 1. Delete all tracked files
  const oldSidecar = await readSidecar(wsRoot);
  for (const relPath of oldSidecar?.files || []) {
    try {
      await vscode.workspace.fs.delete(vscode.Uri.joinPath(wsRoot, relPath));
    } catch {
      // file may already be gone
    }
  }

  // 2. Remove managed MCP servers from .vscode/mcp.json
  const mcpServers = oldSidecar?.mcpServers || [];
  if (mcpServers.length > 0) {
    await removeMcpServers(wsRoot, mcpServers);
  }

  // 3. Remove sidecar
  const uri = vscode.Uri.joinPath(wsRoot, SIDECAR_REL);
  try {
    await vscode.workspace.fs.delete(uri);
  } catch {
    // already gone
  }

  // 4. Remove git exclude block
  await _removeGitExclude(wsRoot);
}

/**
 * Check whether a file exists at the given URI.
 */
async function fileExists(uri) {
  try {
    await vscode.workspace.fs.stat(uri);
    return true;
  } catch {
    return false;
  }
}

// ── MCP Server Config Helpers ───────────────────────────────────

/**
 * Read the workspace's `.vscode/mcp.json` file.
 * Returns the parsed config or an empty object if not present.
 * @param {vscode.Uri} wsRoot
 * @returns {Promise<{servers?: object, inputs?: object}>}
 */
async function readMcpConfig(wsRoot) {
  const uri = vscode.Uri.joinPath(wsRoot, MCP_CONFIG_REL);
  try {
    const raw = await vscode.workspace.fs.readFile(uri);
    return JSON.parse(Buffer.from(raw).toString('utf8'));
  } catch {
    return {};
  }
}

/**
 * Write the workspace's `.vscode/mcp.json` file.
 * Merges managed servers into the existing config, preserving user-defined servers.
 * @param {vscode.Uri} wsRoot
 * @param {object} managedServers - Server configs to inject (keyed by server name)
 * @param {string[]} previousManagedNames - Previously managed server names (to clean up stale)
 */
async function writeMcpConfig(wsRoot, managedServers, previousManagedNames = []) {
  const config = await readMcpConfig(wsRoot);

  // Ensure servers object exists
  config.servers = config.servers || {};

  // Remove previously managed servers that are no longer in the managed set
  const newNames = new Set(Object.keys(managedServers));
  for (const oldName of previousManagedNames) {
    if (!newNames.has(oldName)) {
      delete config.servers[oldName];
    }
  }

  // Merge in managed servers (overwrite managed ones, don't touch user servers)
  for (const [name, serverConfig] of Object.entries(managedServers)) {
    config.servers[name] = serverConfig;
  }

  // Write the file
  const uri = vscode.Uri.joinPath(wsRoot, MCP_CONFIG_REL);
  await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(wsRoot, '.vscode'));
  await vscode.workspace.fs.writeFile(uri, Buffer.from(JSON.stringify(config, null, 2) + '\n'));
}

/**
 * Remove all managed MCP servers from the workspace config.
 * @param {vscode.Uri} wsRoot
 * @param {string[]} serverNames - Names of servers to remove
 */
async function removeMcpServers(wsRoot, serverNames) {
  if (serverNames.length === 0) return;

  const config = await readMcpConfig(wsRoot);
  if (!config.servers) return;

  let changed = false;
  for (const name of serverNames) {
    if (config.servers[name]) {
      delete config.servers[name];
      changed = true;
    }
  }

  if (!changed) return;

  // If no servers left, consider removing the entire file or just the servers key
  if (Object.keys(config.servers).length === 0) {
    delete config.servers;
  }

  const uri = vscode.Uri.joinPath(wsRoot, MCP_CONFIG_REL);
  if (Object.keys(config).length === 0) {
    // Config is empty, remove the file
    try {
      await vscode.workspace.fs.delete(uri);
    } catch {
      // file may already be gone
    }
  } else {
    await vscode.workspace.fs.writeFile(uri, Buffer.from(JSON.stringify(config, null, 2) + '\n'));
  }
}

/**
 * Load MCP server definitions from a JSON asset file.
 * Returns an object mapping server names to their configs.
 * Supports both bundled assets and remote fetch based on settings.
 * @param {vscode.ExtensionContext} context
 * @param {string} srcPath - Relative path to the JSON file in assets/
 * @param {string} [basePath] - Base path in repo (for remote mode)
 * @returns {Promise<object>}
 */
async function loadMcpServerConfig(context, srcPath, basePath) {
  if (isRemoteMode()) {
    try {
      // Build the full remote path
      const remotePath = basePath ? `${basePath}/${srcPath}` : srcPath;
      const data = await fetchJSONAuth(remotePath);
      if (data) return data;
    } catch {
      // Fall back to bundled
    }
  }

  // Bundled mode
  const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', srcPath);
  try {
    const raw = await vscode.workspace.fs.readFile(srcUri);
    return JSON.parse(Buffer.from(raw).toString('utf8'));
  } catch {
    return {};
  }
}

// ── Git exclude (PRIVATE — only called by writeSidecar/deleteSidecar) ──

/**
 * Get the URI for `.git/info/exclude` in the workspace.
 */
function _getGitExcludeUri(wsRoot) {
  return vscode.Uri.joinPath(wsRoot, '.git', 'info', 'exclude');
}

/**
 * Sync `.git/info/exclude` with the given paths.
 * Replaces any existing Activate block, or appends a new one.
 * PRIVATE — called automatically by writeSidecar().
 */
async function _syncGitExclude(wsRoot, paths) {
  const excludeUri = _getGitExcludeUri(wsRoot);

  // Ensure .git/info/ exists
  try {
    await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(wsRoot, '.git', 'info'));
  } catch {
    // .git may not exist (not a git repo) — bail silently
    return;
  }

  let content;
  try {
    const raw = await vscode.workspace.fs.readFile(excludeUri);
    content = Buffer.from(raw).toString('utf8');
  } catch {
    content = '';
  }

  // Build the new block
  const block = [
    EXCLUDE_MARKER_START,
    ...paths,
    EXCLUDE_MARKER_END,
  ].join('\n');

  // Replace existing block or append
  const startIdx = content.indexOf(EXCLUDE_MARKER_START);
  const endIdx = content.indexOf(EXCLUDE_MARKER_END);

  if (startIdx !== -1 && endIdx !== -1) {
    content = content.slice(0, startIdx) + block + content.slice(endIdx + EXCLUDE_MARKER_END.length);
  } else {
    if (content.length > 0 && !content.endsWith('\n')) {
      content += '\n';
    }
    content += '\n' + block + '\n';
  }

  await vscode.workspace.fs.writeFile(excludeUri, Buffer.from(content));
}

/**
 * Remove the Activate block from `.git/info/exclude`.
 * PRIVATE — called automatically by deleteSidecar().
 */
async function _removeGitExclude(wsRoot) {
  const excludeUri = _getGitExcludeUri(wsRoot);

  let content;
  try {
    const raw = await vscode.workspace.fs.readFile(excludeUri);
    content = Buffer.from(raw).toString('utf8');
  } catch {
    return;
  }

  const startIdx = content.indexOf(EXCLUDE_MARKER_START);
  const endIdx = content.indexOf(EXCLUDE_MARKER_END);

  if (startIdx === -1 || endIdx === -1) return;

  content = content.slice(0, startIdx) + content.slice(endIdx + EXCLUDE_MARKER_END.length);
  content = content.replace(/\n{3,}/g, '\n\n');

  await vscode.workspace.fs.writeFile(excludeUri, Buffer.from(content));
}

// ── Manifest reading (reuse from installer) ─────────────────────

const {
  discoverManifests,
  readManifestById,
  discoverBundledManifests,
  readBundledManifestById,
  parseFrontmatterVersion,
} = require('./installer');

// ── Core injection logic ────────────────────────────────────────

/**
 * Inject managed files into the workspace's `.github/` directory.
 * MCP server configs are merged into `.vscode/mcp.json` instead.
 * Supports both bundled and remote sources based on settings.
 *
 * Pipeline: copy files → merge mcp servers → write sidecar → exclude auto-synced.
 *
 * @param {vscode.ExtensionContext} context
 * @param {string} tier
 * @param {string} [manifestId]
 * @returns {Promise<{injected: string[], skipped: string[], mcpServers: string[], version: string, manifestId: string}>}
 */
async function injectFiles(context, tier, manifestId) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) throw new Error('No workspace folder open');

  // Use smart discovery (respects remote mode setting)
  const chosen = manifestId
    ? await readManifestById(context, manifestId)
    : (await discoverManifests(context))[0];

  if (!chosen) throw new Error('No manifest available');

  const remoteMode = isRemoteMode();
  const basePath = chosen.basePath || '';

  const files = selectFiles(chosen.files, tier);
  const oldSidecar = await readSidecar(wsRoot);
  const previouslyInjected = new Set(oldSidecar?.files || []);
  const previousMcpServers = oldSidecar?.mcpServers || [];

  // Separate MCP server files from regular files
  const regularFiles = [];
  const mcpFiles = [];
  for (const f of files) {
    const cat = f.category || inferCategory(f.src);
    if (cat === 'mcp-servers') {
      mcpFiles.push(f);
    } else {
      regularFiles.push(f);
    }
  }

  const injected = [];
  const skipped = [];

  // Handle regular files (copy to .github/)
  for (const f of regularFiles) {
    const destRel = `.github/${f.dest}`;
    const destUri = vscode.Uri.joinPath(wsRoot, destRel);

    // If file exists and we didn't put it there, skip
    if ((await fileExists(destUri)) && !previouslyInjected.has(destRel)) {
      skipped.push(f.dest);
      continue;
    }

    try {
      await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(destUri, '..'));

      if (remoteMode) {
        // Fetch from GitHub
        const remotePath = basePath ? `${basePath}/${f.src}` : f.src;
        const content = await fetchFileAsBuffer(remotePath);
        if (content) {
          await vscode.workspace.fs.writeFile(destUri, content);
          injected.push(destRel);
        }
      } else {
        // Copy from bundled assets
        const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', f.src);
        await vscode.workspace.fs.copy(srcUri, destUri, { overwrite: true });
        injected.push(destRel);
      }
    } catch {
      // Asset may not exist in bundle or remote
    }
  }

  // Inject AGENTS.md at workspace root
  const agentsRel = 'AGENTS.md';
  const agentsDestUri = vscode.Uri.joinPath(wsRoot, agentsRel);

  if ((await fileExists(agentsDestUri)) && !previouslyInjected.has(agentsRel)) {
    skipped.push(agentsRel);
  } else {
    try {
      if (remoteMode) {
        const remotePath = basePath ? `${basePath}/AGENTS.md` : 'AGENTS.md';
        const content = await fetchFileAsBuffer(remotePath);
        if (content) {
          await vscode.workspace.fs.writeFile(agentsDestUri, content);
          injected.push(agentsRel);
        }
      } else {
        const agentsSrcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', 'AGENTS.md');
        await vscode.workspace.fs.copy(agentsSrcUri, agentsDestUri, { overwrite: true });
        injected.push(agentsRel);
      }
    } catch {
      // AGENTS.md may not be in the bundle or remote
    }
  }

  // Handle MCP server configs (merge into .vscode/mcp.json)
  const injectedMcpServers = [];
  const managedServers = {};

  for (const f of mcpFiles) {
    try {
      const serverConfigs = await loadMcpServerConfig(context, f.src, basePath);
      for (const [name, config] of Object.entries(serverConfigs)) {
        managedServers[name] = config;
        injectedMcpServers.push(name);
      }
    } catch {
      // MCP server config may not exist or be invalid
    }
  }

  // Write MCP config if we have any servers to manage
  if (injectedMcpServers.length > 0 || previousMcpServers.length > 0) {
    await writeMcpConfig(wsRoot, managedServers, previousMcpServers);
  }

  // Write sidecar — pipeline auto-deletes stale files and syncs exclude
  await writeSidecar(wsRoot, {
    manifest: chosen.id,
    version: chosen.version,
    tier,
    files: injected,
    mcpServers: injectedMcpServers,
    source: remoteMode ? 'remote' : 'bundled',
  });

  return { injected, skipped, mcpServers: injectedMcpServers, version: chosen.version, manifestId: chosen.id };
}

/**
 * Read the injected version info from the sidecar.
 * Returns { manifest, version, tier, files } or null.
 */
async function readInjectedVersion() {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return null;
  return readSidecar(wsRoot);
}

/**
 * Check if a file is injected in the workspace.
 */
async function isFileInjected(file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;
  const destUri = vscode.Uri.joinPath(wsRoot, '.github', file.dest);
  return fileExists(destUri);
}

/**
 * Inject a single file into the workspace.
 * Supports both bundled and remote sources based on settings.
 * @param {vscode.ExtensionContext} context
 * @param {{src: string, dest: string}} file
 * @param {string} [basePath] - Base path in repo (for remote mode)
 * @returns {Promise<boolean>}
 */
async function injectSingleFile(context, file, basePath) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const destRel = `.github/${file.dest}`;
  const destUri = vscode.Uri.joinPath(wsRoot, destRel);

  try {
    await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(destUri, '..'));

    if (isRemoteMode()) {
      // Fetch from GitHub
      const remotePath = basePath ? `${basePath}/${file.src}` : file.src;
      const content = await fetchFileAsBuffer(remotePath);
      if (!content) return false;
      await vscode.workspace.fs.writeFile(destUri, content);
    } else {
      // Copy from bundled assets
      const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', file.src);
      await vscode.workspace.fs.copy(srcUri, destUri, { overwrite: true });
    }

    // Update sidecar (this automatically syncs git exclude)
    const sidecar = (await readSidecar(wsRoot)) || { manifest: '', version: '', tier: '', files: [] };
    if (!sidecar.files.includes(destRel)) {
      sidecar.files.push(destRel);
    }
    await writeSidecar(wsRoot, sidecar);

    return true;
  } catch {
    return false;
  }
}

/**
 * Remove a single injected file from the workspace.
 * Removes it from the sidecar; writeSidecar pipeline deletes from disk.
 * @param {{dest: string}} file
 * @returns {Promise<boolean>}
 */
async function removeSingleFile(file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const destRel = `.github/${file.dest}`;
  const sidecar = await readSidecar(wsRoot);
  if (!sidecar) return false;

  sidecar.files = sidecar.files.filter((p) => p !== destRel);
  // Pipeline auto-deletes the removed file from disk and syncs exclude
  await writeSidecar(wsRoot, sidecar);
  return true;
}

/**
 * Inject a single MCP server into the workspace.
 * @param {vscode.ExtensionContext} context
 * @param {{src: string, dest: string}} file - MCP server manifest file entry
 * @returns {Promise<{serverNames: string[]}>}
 */
async function injectSingleMcpServer(context, file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return { serverNames: [] };

  try {
    const serverConfigs = await loadMcpServerConfig(context, file.src);
    const serverNames = Object.keys(serverConfigs);

    if (serverNames.length === 0) return { serverNames: [] };

    // Read current sidecar for previous MCP servers
    const sidecar = (await readSidecar(wsRoot)) || { manifest: '', version: '', tier: '', files: [], mcpServers: [] };
    const previousMcpServers = sidecar.mcpServers || [];

    // Merge with existing managed servers
    const currentConfig = await readMcpConfig(wsRoot);
    const managedServers = {};

    // Keep previously managed servers that are still in the sidecar
    for (const prevName of previousMcpServers) {
      if (currentConfig.servers?.[prevName]) {
        managedServers[prevName] = currentConfig.servers[prevName];
      }
    }

    // Add the new servers
    for (const [name, config] of Object.entries(serverConfigs)) {
      managedServers[name] = config;
    }

    // Write MCP config
    await writeMcpConfig(wsRoot, managedServers, previousMcpServers);

    // Update sidecar
    const newMcpServers = [...new Set([...previousMcpServers, ...serverNames])];
    sidecar.mcpServers = newMcpServers;
    await writeSidecar(wsRoot, sidecar);

    return { serverNames };
  } catch {
    return { serverNames: [] };
  }
}

/**
 * Remove a single MCP server from the workspace.
 * @param {string} serverName - Name of the MCP server to remove
 * @returns {Promise<boolean>}
 */
async function removeSingleMcpServer(serverName) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const sidecar = await readSidecar(wsRoot);
  if (!sidecar) return false;

  const mcpServers = sidecar.mcpServers || [];
  if (!mcpServers.includes(serverName)) return false;

  // Remove from MCP config
  await removeMcpServers(wsRoot, [serverName]);

  // Update sidecar
  sidecar.mcpServers = mcpServers.filter((n) => n !== serverName);
  await writeSidecar(wsRoot, sidecar);

  return true;
}

/**
 * Remove all injected files from the workspace and clean up.
 * deleteSidecar pipeline handles file deletion, sidecar removal,
 * and git exclude cleanup.
 */
async function removeAllInjected() {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const sidecar = await readSidecar(wsRoot);
  if (!sidecar) return false;

  // Pipeline auto-deletes all tracked files, sidecar, and exclude block
  await deleteSidecar(wsRoot);
  return true;
}

/**
 * Update only the files that are currently injected.
 * Does not re-add files the user removed.
 * Supports both bundled and remote sources based on settings.
 *
 * @param {vscode.ExtensionContext} context
 * @returns {Promise<{updated: string[], version: string}>}
 */
async function updateInjectedFiles(context) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) throw new Error('No workspace folder open');

  const sidecar = await readSidecar(wsRoot);
  const manifestId = sidecar?.manifest || 'activate-framework';

  // Use smart manifest discovery (respects remote mode)
  let chosen;
  try {
    chosen = await readManifestById(context, manifestId);
  } catch {
    const all = await discoverManifests(context);
    chosen = all[0];
  }
  if (!chosen) throw new Error('No manifest available for update');

  const remoteMode = isRemoteMode();
  const basePath = chosen.basePath || '';
  const previousFiles = new Set(sidecar?.files || []);
  const updated = [];

  for (const f of chosen.files) {
    const destRel = `.github/${f.dest}`;
    if (!previousFiles.has(destRel)) continue;

    const destUri = vscode.Uri.joinPath(wsRoot, destRel);

    try {
      await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(destUri, '..'));

      if (remoteMode) {
        // Fetch from GitHub
        const remotePath = basePath ? `${basePath}/${f.src}` : f.src;
        const content = await fetchFileAsBuffer(remotePath);
        if (content) {
          await vscode.workspace.fs.writeFile(destUri, content);
          updated.push(f.dest);
        }
      } else {
        // Copy from bundled assets
        const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', f.src);
        await vscode.workspace.fs.copy(srcUri, destUri, { overwrite: true });
        updated.push(f.dest);
      }
    } catch {
      // file may not exist in bundle or remote
    }
  }

  // Re-copy AGENTS.md if it was injected
  if (previousFiles.has('AGENTS.md')) {
    try {
      if (remoteMode) {
        const remotePath = basePath ? `${basePath}/AGENTS.md` : 'AGENTS.md';
        const content = await fetchFileAsBuffer(remotePath);
        if (content) {
          await vscode.workspace.fs.writeFile(vscode.Uri.joinPath(wsRoot, 'AGENTS.md'), content);
        }
      } else {
        const agentsSrc = vscode.Uri.joinPath(context.extensionUri, 'assets', 'AGENTS.md');
        const agentsDest = vscode.Uri.joinPath(wsRoot, 'AGENTS.md');
        await vscode.workspace.fs.copy(agentsSrc, agentsDest, { overwrite: true });
      }
    } catch {}
  }

  // Update MCP servers that were previously injected
  const previousMcpServers = sidecar?.mcpServers || [];
  if (previousMcpServers.length > 0) {
    const managedServers = {};

    // Find MCP server files in the manifest and reload their configs
    for (const f of chosen.files) {
      const cat = f.category || inferCategory(f.src);
      if (cat !== 'mcp-servers') continue;

      try {
        const serverConfigs = await loadMcpServerConfig(context, f.src, basePath);
        for (const [name, config] of Object.entries(serverConfigs)) {
          // Only update servers that were previously injected
          if (previousMcpServers.includes(name)) {
            managedServers[name] = config;
          }
        }
      } catch {
        // Skip invalid configs
      }
    }

    // Write updated MCP config if we have servers to update
    if (Object.keys(managedServers).length > 0) {
      await writeMcpConfig(wsRoot, managedServers, previousMcpServers);
    }
  }

  // Update sidecar version (this automatically syncs git exclude)
  if (sidecar) {
    sidecar.version = chosen.version;
    sidecar.source = remoteMode ? 'remote' : 'bundled';
    await writeSidecar(wsRoot, sidecar);
  }

  return { updated, version: chosen.version };
}

/**
 * Read the frontmatter version from an injected file in the workspace.
 */
async function readInjectedFileVersion(file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return null;
  const destUri = vscode.Uri.joinPath(wsRoot, '.github', file.dest);
  try {
    const raw = await vscode.workspace.fs.readFile(destUri);
    return parseFrontmatterVersion(raw);
  } catch {
    return null;
  }
}

/**
 * Skip an update by stamping the frontmatter version in the injected file.
 */
async function skipInjectedFileUpdate(context, file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', file.src);
  let bundledVersion;
  try {
    const raw = await vscode.workspace.fs.readFile(srcUri);
    bundledVersion = parseFrontmatterVersion(raw);
  } catch {
    return false;
  }
  if (!bundledVersion) return false;

  const destUri = vscode.Uri.joinPath(wsRoot, '.github', file.dest);
  try {
    const raw = await vscode.workspace.fs.readFile(destUri);
    let text = Buffer.from(raw).toString('utf8');

    const fmMatch = text.match(/^(---\s*\n)([\s\S]*?)(\n---)/);
    if (!fmMatch) return false;

    const before = fmMatch[1];
    let fm = fmMatch[2];
    const after = fmMatch[3];

    if (fm.match(/^version:\s/m)) {
      fm = fm.replace(/^version:\s*['"]?[^'"\n]+['"]?\s*$/m, `version: '${bundledVersion}'`);
    } else {
      fm += `\nversion: '${bundledVersion}'`;
    }

    text = before + fm + after + text.slice(fmMatch[0].length);
    await vscode.workspace.fs.writeFile(destUri, Buffer.from(text));
    return true;
  } catch {
    return false;
  }
}

module.exports = {
  SIDECAR_REL,
  MCP_CONFIG_REL,
  getWorkspaceRoot,
  readSidecar,
  readInjectedVersion,
  injectFiles,
  injectSingleFile,
  removeSingleFile,
  removeAllInjected,
  isFileInjected,
  updateInjectedFiles,
  readInjectedFileVersion,
  skipInjectedFileUpdate,
  readMcpConfig,
  injectSingleMcpServer,
  removeSingleMcpServer,
};
