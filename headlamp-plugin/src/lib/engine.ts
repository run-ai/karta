// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { ApiProxy } from '@kinvolk/headlamp-plugin/lib';

const PLUGIN_NAME = 'karta';

export type VersionFn = () => string;

export interface KartaEngine {
  version: VersionFn;
}

interface GoRuntime {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

declare global {
  interface Window {
    Go?: new () => GoRuntime;
    kartaVersion?: VersionFn;
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

// loadScript injects a <script> tag and lets the browser fetch and execute
// it, instead of fetching the text and eval-ing it ourselves.
function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = src;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error(`fetch ${src}: failed to load`));
    document.head.appendChild(script);
  });
}

// The plugin bundle is executed from a string, so it cannot learn its own URL
// from the module system. The backend's plugin listing maps plugin names to
// their base paths; fall back to the conventional path if it is unavailable.
// Returned path is relative to getAppUrl(), not an absolute URL — ApiProxy
// requests resolve it against the app URL themselves.
async function findPluginBase(): Promise<string> {
  try {
    const list = (await ApiProxy.request('/plugins', {}, false, false)) as {
      path?: string;
      name?: string;
    }[];
    const entry = list.find(item => item.name === PLUGIN_NAME);
    if (entry?.path) {
      return entry.path;
    }
  } catch {
    // Fall through to the conventional path.
  }
  return `plugins/${PLUGIN_NAME}`;
}

async function waitForExports(timeoutMs: number): Promise<KartaEngine> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (typeof window.kartaVersion === 'function') {
      return { version: window.kartaVersion };
    }
    await new Promise(resolve => setTimeout(resolve, 10));
  }
  throw new Error('the WebAssembly module did not register its exports');
}

async function instantiate(): Promise<KartaEngine> {
  const base = await findPluginBase();

  // wasm_exec.js must load as a real <script> tag (not fetched as data), so
  // it needs an absolute URL — getAppUrl() is only needed for this one call.
  await loadScript(`${getAppUrl()}${base}/wasm_exec.js`);
  if (!window.Go) {
    throw new Error('wasm_exec.js did not define the Go runtime');
  }

  const go = new window.Go();
  // isJSON: false returns the raw Response instead of parsed JSON, which is
  // what WebAssembly.instantiateStreaming needs below. ApiProxy.request
  // already rejects on a non-ok response, so no manual .ok check is needed.
  const wasmResp = (await ApiProxy.request(
    `/${base}/karta.wasm`,
    { isJSON: false },
    false,
    false
  )) as Response;

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

let enginePromise: Promise<KartaEngine> | null = null;

// getKartaEngine instantiates the module once and reuses it. A load failure
// is not cached, so a transient fetch error can be retried.
export function getKartaEngine(): Promise<KartaEngine> {
  if (!enginePromise) {
    enginePromise = instantiate().catch(err => {
      enginePromise = null;
      throw err;
    });
  }
  return enginePromise;
}
