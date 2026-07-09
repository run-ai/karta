// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { afterEach, describe, expect, it } from 'vitest';
import { getAppUrl } from './wasmEngine';

describe('getAppUrl', () => {
  afterEach(() => {
    delete window.headlampBaseUrl;
    delete window.ddClient;
  });

  it('uses the page origin in a browser deployment', () => {
    expect(getAppUrl()).toBe(`${window.location.origin}/`);
  });

  it('prefixes the configured base URL', () => {
    window.headlampBaseUrl = '/headlamp';
    expect(getAppUrl()).toBe(`${window.location.origin}/headlamp/`);
  });

  it('targets the Docker Desktop backend port when ddClient is present', () => {
    window.ddClient = {};
    expect(getAppUrl()).toBe('http://localhost:64446/');
  });
});
