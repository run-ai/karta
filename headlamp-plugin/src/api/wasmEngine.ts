// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

// Loads the Karta tree engine: a WebAssembly build of the karta Go library
// (headlamp-plugin/wasm) shipped with the plugin. Building trees runs fully
// in the browser, so no operator endpoint or extra RBAC is needed.

const PLUGIN_NAME = 'karta';

export interface BuildTreeResult {
  tree?: string;
  error?: string;
}

export interface MatchPodsResult {
  /** Pod name -> match for pods matched by the podSelector. */
  matches?: Record<string, { component: string; instance?: string }>;
  error?: string;
}

export type BuildTreeFn = (kartaJSON: string, workloadJSON: string) => BuildTreeResult;
export type MatchPodsFn = (kartaJSON: string, podListJSON: string) => MatchPodsResult;

export interface TreeEngine {
  buildTree: BuildTreeFn;
  matchPods: MatchPodsFn;
}

interface GoRuntime {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

declare global {
  interface Window {
    Go?: new () => GoRuntime;
    kartaBuildTree?: BuildTreeFn;
    kartaMatchPods?: MatchPodsFn;
    /** Set by Electron to the backend server port. */
    headlampBackendPort?: number;
    /** Base URL prefix Headlamp was built/served with, e.g. '/headlamp'. */
    headlampBaseUrl?: string;
    /** Present when running as the Docker Desktop extension. */
    ddClient?: unknown;
  }
}

function isElectron(): boolean {
  const proc = (window as { process?: { type?: string } }).process;
  if (typeof proc === 'object' && proc?.type === 'renderer') {
    return true;
  }
  return typeof navigator === 'object' && navigator.userAgent?.includes('Electron');
}

// getAppUrl mirrors Headlamp's internal helper of the same name, which is not
// exposed to plugins through pluginLib.
export function getAppUrl(): string {
  let backendPort = 4466;
  let useLocalhost = false;

  if (isElectron()) {
    if (window.headlampBackendPort) {
      backendPort = window.headlampBackendPort;
    }
    useLocalhost = true;
  }
  if (window.ddClient !== undefined) {
    backendPort = 64446;
    useLocalhost = true;
  }

  const origin = useLocalhost ? `http://localhost:${backendPort}` : window.location.origin;
  const baseUrl = isElectron() ? '' : (window.headlampBaseUrl ?? '');
  return origin + (baseUrl ? `${baseUrl}/` : '/');
}

// The plugin bundle is executed from a string, so it cannot learn its own URL
// from the module system. The backend's plugin listing maps plugin names to
// their base paths; fall back to the conventional path if it is unavailable.
async function findPluginBase(): Promise<string> {
  const appUrl = getAppUrl();
  try {
    const resp = await fetch(`${appUrl}plugins`);
    if (resp.ok) {
      const list = (await resp.json()) as { path?: string; name?: string }[];
      const entry = list.find(item => item.name === PLUGIN_NAME);
      if (entry?.path) {
        return `${appUrl}${entry.path}`;
      }
    }
  } catch {
    // Fall through to the conventional path.
  }
  return `${appUrl}plugins/${PLUGIN_NAME}`;
}

async function waitForExports(timeoutMs: number): Promise<TreeEngine> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (typeof window.kartaBuildTree === 'function' && typeof window.kartaMatchPods === 'function') {
      return { buildTree: window.kartaBuildTree, matchPods: window.kartaMatchPods };
    }
    await new Promise(resolve => setTimeout(resolve, 10));
  }
  throw new Error('the WebAssembly module did not register its exports');
}

async function instantiate(): Promise<TreeEngine> {
  const base = await findPluginBase();

  const execResp = await fetch(`${base}/wasm_exec.js`);
  if (!execResp.ok) {
    throw new Error(`fetch wasm_exec.js: HTTP ${execResp.status}`);
  }
  // wasm_exec.js is Go's plain-script runtime glue; evaluate it to define the
  // global Go constructor.
  new Function(await execResp.text())();
  if (!window.Go) {
    throw new Error('wasm_exec.js did not define the Go runtime');
  }

  const go = new window.Go();
  const wasmResp = await fetch(`${base}/karta.wasm`);
  if (!wasmResp.ok) {
    throw new Error(`fetch karta.wasm: HTTP ${wasmResp.status}`);
  }

  let instance: WebAssembly.Instance;
  try {
    ({ instance } = await WebAssembly.instantiateStreaming(wasmResp.clone(), go.importObject));
  } catch {
    // instantiateStreaming requires an application/wasm content type; fall
    // back to buffering for servers that mislabel it.
    const buffer = await wasmResp.arrayBuffer();
    ({ instance } = await WebAssembly.instantiate(buffer, go.importObject));
  }

  // run() resolves only when the Go program exits; the module blocks forever
  // to keep its exported functions alive.
  go.run(instance);
  return waitForExports(3000);
}

let enginePromise: Promise<TreeEngine> | null = null;

// getTreeEngine instantiates the module once and reuses it. A load failure is
// not cached, so a transient fetch error can be retried.
export function getTreeEngine(): Promise<TreeEngine> {
  if (!enginePromise) {
    enginePromise = instantiate().catch(err => {
      enginePromise = null;
      throw err;
    });
  }
  return enginePromise;
}
