/**
 * Copilot Telemetry Logger
 *
 * Records daily GitHub Copilot premium-interaction quota usage to a local
 * JSONL file inside ~/.activate/.  Based on ironarch-technology/copilot-telemetry-logger.
 *
 * Log schema (one JSON object per line):
 *   {
 *     "date": "2026-02-24",
 *     "timestamp": "2026-02-24T14:00:00.000Z",
 *     "premium_entitlement": 300,
 *     "premium_remaining": 142,
 *     "premium_used": 158,
 *     "quota_reset_date_utc": "2026-03-01T00:00:00Z",
 *     "source": "github_copilot_internal",
 *     "version": 1
 *   }
 */
const vscode = require('vscode');
const os = require('os');
const path = require('path');
const fs = require('node:fs/promises');

// ── Constants ───────────────────────────────────────────────────

const ACTIVATE_DIR = path.join(os.homedir(), '.activate');
const DEFAULT_LOG_FILE = 'copilot-telemetry.jsonl';
const NO_RESET_KEY = 'no-reset';
const FORCE_RESET_ENV = 'COPILOT_TELEMETRY_FORCE_RESET';

// Global-state keys (scoped to this module)
const LAST_RUN_KEY = 'telemetry.lastRunDate';
const LAST_FOCUS_RUN_TS_KEY = 'telemetry.lastFocusRunTs';
const CURRENT_QUOTA_RESET_KEY = 'telemetry.currentQuotaReset';

/** Minimum interval between focus-triggered log runs (1 hour). */
const FOCUS_THROTTLE_MS = 60 * 60 * 1000;

// ── Public API ──────────────────────────────────────────────────

/**
 * Initialise the telemetry logger.
 * Call once from the extension's `activate()` function.
 *
 * @param {vscode.ExtensionContext} context
 */
function initTelemetry(context) {
  // Register the manual command
  context.subscriptions.push(
    vscode.commands.registerCommand('activate-framework.telemetryRunNow', async () => {
      await runDailyLog(context, 'manual');
    }),
  );

  if (_isTestEnvironment()) return;
  if (!_isEnabled()) return;

  // Log on activation (startup)
  void runDailyLog(context, 'startup');

  // Re-log on window focus, throttled to once per hour
  const focusDisposable = vscode.window.onDidChangeWindowState((state) => {
    if (!state.focused) return;
    if (!_isEnabled()) return;
    const last = context.globalState.get(LAST_FOCUS_RUN_TS_KEY) ?? 0;
    if (shouldRunOnFocus(last, Date.now(), FOCUS_THROTTLE_MS)) {
      void runDailyLog(context, 'focus');
      void context.globalState.update(LAST_FOCUS_RUN_TS_KEY, Date.now());
    }
  });
  context.subscriptions.push(focusDisposable);
}

/**
 * Run the daily log (fetch quota → append entry).
 *
 * @param {vscode.ExtensionContext} context
 * @param {'startup'|'manual'|'focus'} trigger
 */
async function runDailyLog(context, trigger) {
  try {
    if (!_isEnabled() && trigger !== 'manual') return;

    const today = new Date();
    const todayKey = formatDateKey(today);
    const lastRunKey = context.globalState.get(LAST_RUN_KEY);

    if (trigger === 'startup' && lastRunKey === todayKey) return;

    const data = await fetchCopilotUserData();

    // Support forced quota-reset for testing
    const forcedReset = process.env[FORCE_RESET_ENV];
    if (forcedReset && forcedReset.trim().length > 0) {
      data.quota_reset_date_utc = forcedReset.trim();
    }

    const quota = extractPremiumQuota(data);

    const entry = {
      date: todayKey,
      timestamp: new Date().toISOString(),
      premium_entitlement: quota?.entitlement ?? null,
      premium_remaining: quota?.remaining ?? null,
      premium_used:
        quota && quota.entitlement != null && quota.remaining != null
          ? Math.max(0, quota.entitlement - quota.remaining)
          : null,
      quota_reset_date_utc: data.quota_reset_date_utc ?? null,
      source: 'github_copilot_internal',
      version: 1,
    };

    await appendDailyLog(context, entry);
    await context.globalState.update(LAST_RUN_KEY, todayKey);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error('Copilot Telemetry Logger failed:', message);
    if (trigger === 'manual') {
      vscode.window.showWarningMessage(
        `Copilot Telemetry Logger failed to record usage: ${message}`,
      );
    }
  }
}

/**
 * Read all log entries from the active JSONL file.
 * Used by the Usage page in the control panel.
 *
 * @returns {Promise<Array<object>>}
 */
async function readLogEntries() {
  const dirPath = _resolveLogDir();
  const activeFilePath = path.join(dirPath, DEFAULT_LOG_FILE);
  try {
    const raw = await fs.readFile(activeFilePath, 'utf8');
    return raw
      .split('\n')
      .filter((line) => line.trim().length > 0)
      .map((line) => {
        try {
          return JSON.parse(line);
        } catch {
          return null;
        }
      })
      .filter(Boolean);
  } catch {
    return [];
  }
}

/**
 * Get the resolved path to the active log file.
 * @returns {string}
 */
function getLogFilePath() {
  return path.join(_resolveLogDir(), DEFAULT_LOG_FILE);
}

// ── Internal helpers ────────────────────────────────────────────

/**
 * Check whether telemetry logging is enabled in settings.
 * @returns {boolean}
 */
function _isEnabled() {
  const config = vscode.workspace.getConfiguration('peregrine-activate');
  return config.get('telemetry.enabled', true);
}

/**
 * Resolve the directory for log files.
 * Uses `peregrine-activate.telemetry.logDirectory` setting, defaulting
 * to `~/.activate`.
 * @returns {string}
 */
function _resolveLogDir() {
  const config = vscode.workspace.getConfiguration('peregrine-activate');
  const configured = config.get('telemetry.logDirectory', '');
  const trimmed = configured.trim();

  if (trimmed.length === 0) return ACTIVATE_DIR;
  if (trimmed.startsWith('~')) return path.join(os.homedir(), trimmed.slice(1));
  return trimmed;
}

/**
 * Fetch GitHub Copilot internal user data including quota snapshots.
 * @returns {Promise<object>}
 */
async function fetchCopilotUserData() {
  const session = await vscode.authentication.getSession('github', ['user:email'], {
    createIfNone: true,
  });

  if (!session) {
    throw new Error('GitHub authentication session not available');
  }

  const response = await fetch('https://api.github.com/copilot_internal/user', {
    headers: {
      Authorization: `Bearer ${session.accessToken}`,
      Accept: 'application/json',
      'User-Agent': 'Peregrine-Activate-Telemetry',
    },
  });

  if (!response.ok) {
    throw new Error(`GitHub API returned ${response.status}: ${response.statusText}`);
  }

  const json = await response.json();
  return json ?? {};
}

/**
 * Extract the premium_interactions quota snapshot from the user data.
 * @param {object} data
 * @returns {{quota_id: string, entitlement?: number, remaining?: number}|null}
 */
function extractPremiumQuota(data) {
  const snapshots = data.quota_snapshots ? Object.values(data.quota_snapshots) : [];
  const premium = snapshots.find((q) => q.quota_id === 'premium_interactions');
  if (!premium || premium.unlimited) return null;
  return premium;
}

/**
 * Append a log entry to the active JSONL file, archiving first if the
 * quota reset date has changed.
 *
 * @param {vscode.ExtensionContext} context
 * @param {object} entry
 */
async function appendDailyLog(context, entry) {
  const dirPath = _resolveLogDir();
  await fs.mkdir(dirPath, { recursive: true });

  const currentQuotaKey = entry.quota_reset_date_utc ?? NO_RESET_KEY;
  const prevQuotaKey = context.globalState.get(CURRENT_QUOTA_RESET_KEY) ?? NO_RESET_KEY;

  await archiveActiveLogIfNeeded(dirPath, prevQuotaKey, currentQuotaKey, new Date());

  const activeFilePath = path.join(dirPath, DEFAULT_LOG_FILE);
  const line = JSON.stringify(entry) + '\n';
  await fs.appendFile(activeFilePath, line, 'utf8');

  await context.globalState.update(CURRENT_QUOTA_RESET_KEY, currentQuotaKey);
}

// ── Pure / testable helpers ─────────────────────────────────────

/**
 * Format a Date to `YYYY-MM-DD` (UTC).
 * @param {Date} date
 * @returns {string}
 */
function formatDateKey(date) {
  const year = date.getUTCFullYear();
  const month = String(date.getUTCMonth() + 1).padStart(2, '0');
  const day = String(date.getUTCDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

/**
 * Derive the archive date-stamp from the previous quota reset key.
 * @param {string} prevQuotaKey
 * @param {Date} now
 * @returns {string}
 */
function getArchiveDateStamp(prevQuotaKey, now) {
  if (prevQuotaKey !== NO_RESET_KEY) {
    const parsed = new Date(prevQuotaKey);
    if (!isNaN(parsed.getTime())) return formatDateKey(parsed);
  }
  return formatDateKey(now);
}

/**
 * Determine whether a focus-triggered run should proceed.
 * @param {number} lastRunTs - Epoch ms of last focus run
 * @param {number} nowMs     - Current epoch ms
 * @param {number} throttleMs
 * @returns {boolean}
 */
function shouldRunOnFocus(lastRunTs, nowMs, throttleMs) {
  return nowMs - lastRunTs >= throttleMs;
}

/**
 * If the quota reset key changed, rename the active log to an
 * archive file containing the date stamp.
 *
 * @param {string} dirPath
 * @param {string} prevQuotaKey
 * @param {string} currentQuotaKey
 * @param {Date} now
 * @returns {Promise<string|null>} Archive path if archival occurred
 */
async function archiveActiveLogIfNeeded(dirPath, prevQuotaKey, currentQuotaKey, now) {
  if (prevQuotaKey === currentQuotaKey) return null;

  const activeFilePath = path.join(dirPath, DEFAULT_LOG_FILE);

  try {
    await fs.stat(activeFilePath);
  } catch {
    return null;
  }

  const dateStamp = getArchiveDateStamp(prevQuotaKey, now);
  const archiveName = `copilot-telemetry-${dateStamp}.jsonl`;
  const archivePath = path.join(dirPath, archiveName);

  try {
    await fs.rename(activeFilePath, archivePath);
  } catch {
    await fs.copyFile(activeFilePath, archivePath);
    await fs.unlink(activeFilePath);
  }

  return archivePath;
}

/** @returns {boolean} */
function _isTestEnvironment() {
  return Boolean(
    process.env.VSCODE_EXTENSION_TEST ||
    process.env.VSCODE_TEST ||
    process.env.NODE_ENV === 'test',
  );
}

module.exports = {
  // Public
  initTelemetry,
  runDailyLog,
  readLogEntries,
  getLogFilePath,
  // Testable helpers
  formatDateKey,
  getArchiveDateStamp,
  shouldRunOnFocus,
  archiveActiveLogIfNeeded,
  extractPremiumQuota,
  FOCUS_THROTTLE_MS,
};
