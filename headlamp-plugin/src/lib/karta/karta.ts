// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { ApiProxy } from '@kinvolk/headlamp-plugin/lib';

const PLUGIN_NAME = 'karta';
const WASM_EXPORTS_TIMEOUT_MS = 3000;

export interface Envelope {
  data: string | null;
  error: string | null;
}

export interface KartaEngine {
  buildTree(definitionJSON: string, workloadJSON: string): Envelope;
  attributePods(definitionJSON: string, workloadJSON: string, podsJSON: string): Envelope;
  evaluatePhases(definitionJSON: string, workloadJSON: string): Envelope;
  listCatalog(): Envelope;
}

interface GoRuntime {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

declare global {
  interface Window {
    Go?: new () => GoRuntime;
    karta?: KartaEngine;
  }
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = src;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error(`fetch ${src}: failed to load`));
    document.head.appendChild(script);
  });
}

// Exported so it can be unit-tested directly, without needing to also mock
// the rest of instantiate()'s WASM-loading flow.
export async function loadScriptViaApiProxy(path: string): Promise<void> {
  const resp = (await ApiProxy.request(path, { isJSON: false }, false, false)) as Response;
  const text = await resp.text();
  const blobUrl = URL.createObjectURL(new Blob([text], { type: 'application/javascript' }));
  try {
    await loadScript(blobUrl);
  } finally {
    URL.revokeObjectURL(blobUrl);
  }
}

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

function isKartaLoaded(karta?: KartaEngine): karta is KartaEngine {
  return !!karta && Object.keys(karta).length > 0;
}

async function waitForExports(): Promise<KartaEngine> {
  const deadline = Date.now() + WASM_EXPORTS_TIMEOUT_MS;
  while (Date.now() < deadline) {
    if (isKartaLoaded(window.karta)) {
      return window.karta;
    }
    await new Promise(resolve => setTimeout(resolve, 10));
  }
  throw new Error('the WebAssembly module did not register its exports');
}

async function instantiate(): Promise<KartaEngine> {
  const base = await findPluginBase();

  await loadScriptViaApiProxy(`/${base}/wasm_exec.js`);
  if (!window.Go) {
    throw new Error('wasm_exec.js did not define the Go runtime');
  }

  const go = new window.Go();
  const wasmResp = (await ApiProxy.request(
    `/${base}/karta.wasm`,
    { isJSON: false },
    false,
    false
  )) as Response;

  let instance: WebAssembly.Instance;
  try {
    ({ instance } = await WebAssembly.instantiateStreaming(wasmResp, go.importObject));
  } catch (err) {
    console.error('karta: failed to instantiate the WASM module', err);
    throw err;
  }

  go.run(instance);
  return waitForExports();
}

let enginePromise: Promise<KartaEngine> | null = null;

export function getKartaEngine(): Promise<KartaEngine> {
  if (!enginePromise) {
    enginePromise = instantiate().catch(err => {
      enginePromise = null;
      throw err;
    });
  }
  return enginePromise;
}
