import type { ActivateAPI } from '../api.js';
import type {
  AppState,
  Config,
  DiffResult,
  FileStatus,
  Manifest,
  TelemetryEntry,
} from '../types.js';

/**
 * VS Code webview adapter.
 *
 * Bridges ActivateAPI calls to vscode.postMessage() commands.
 * The extension host receives these messages and dispatches them
 * to the ActivateClient (JSON-RPC daemon).
 *
 * For request/response patterns, we use a pending-promise map keyed
 * by a monotonic request ID.
 */

declare const acquireVsCodeApi: () => {
  postMessage(msg: unknown): void;
  getState(): unknown;
  setState(state: unknown): void;
};

let nextReqId = 1;
const pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>();
const stateListeners = new Set<() => void>();

let vscodeApi: ReturnType<typeof acquireVsCodeApi> | null = null;

function getVsCodeApi() {
  if (!vscodeApi) {
    vscodeApi = acquireVsCodeApi();
  }
  return vscodeApi;
}

function request<T>(command: string, params?: Record<string, unknown>): Promise<T> {
  return new Promise((resolve, reject) => {
    const id = nextReqId++;
    pending.set(id, { resolve: resolve as (v: unknown) => void, reject });
    // JSON roundtrip strips Svelte 5 reactive Proxies which can't be
    // cloned by postMessage's structured clone algorithm.
    const msg = JSON.parse(JSON.stringify({ command, ...params, _reqId: id }));
    getVsCodeApi().postMessage(msg);
    // Timeout after 30s
    setTimeout(() => {
      if (pending.has(id)) {
        pending.delete(id);
        reject(new Error(`Request ${command} timed out`));
      }
    }, 30000);
  });
}

function fire(command: string, params?: Record<string, unknown>): void {
  getVsCodeApi().postMessage(JSON.parse(JSON.stringify({ command, ...params })));
}

// Listen for responses from the extension host
if (typeof window !== 'undefined') {
  window.addEventListener('message', (event) => {
    const msg = event.data;
    if (msg?._responseId && pending.has(msg._responseId)) {
      const { resolve, reject } = pending.get(msg._responseId)!;
      pending.delete(msg._responseId);
      if (msg._error) {
        reject(new Error(msg._error));
      } else {
        resolve(msg._result);
      }
    }
    if (msg?.type === 'stateChanged') {
      stateListeners.forEach((cb) => cb());
    }
  });
}

/**
 * Create an ActivateAPI backed by VS Code webview postMessage.
 */
export function createVSCodeAPI(): ActivateAPI {
  return {
    platform: 'vscode',
    getState: () => request<AppState>('getState'),
    getConfig: (scope) => request<Config>('getConfig', { scope }),
    setConfig: (updates) => request('setConfig', { updates }),
    refreshConfig: () => request('refreshConfig'),

    installFile: (file) => request('installFile', { file }),
    uninstallFile: (file) => request('uninstallFile', { file }),
    diffFile: (file) => { request('diffFile', { file }); return Promise.resolve({ file: file.dest, diff: '' }); },
    skipUpdate: (file) => request('skipUpdate', { file }),
    setFileOverride: (dest, override) => request('setOverride', { file: { file: dest, override } }),

    updateAll: () => request('updateAll'),
    addToWorkspace: () => request('addToWorkspace'),
    removeFromWorkspace: () => request('removeFromWorkspace'),

    listManifests: () => request<Manifest[]>('listManifests'),
    listBranches: () => request<string[]>('listBranches'),

    runTelemetry: () => request('refreshUsage'),
    readTelemetryLog: () => request<TelemetryEntry[]>('readTelemetryLog'),

    openFile: (file) => request('openFile', { file }),
    changeTier: () => request('changeTier'),
    changeManifest: () => request('changeManifest'),
    installCLI: () => request('installCLI'),
    checkForUpdates: () => request('checkForUpdates'),

    onStateChanged: (callback) => {
      stateListeners.add(callback);
      return () => stateListeners.delete(callback);
    },
  };
}
