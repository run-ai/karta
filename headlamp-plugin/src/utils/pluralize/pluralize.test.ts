// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { describe, expect, it } from 'vitest';
import { pluralize } from './pluralize';

describe('pluralize', () => {
  it('lowercases and appends s for regular kinds', () => {
    expect(pluralize('Deployment')).toBe('deployments');
    expect(pluralize('PyTorchJob')).toBe('pytorchjobs');
  });

  it('appends es for kinds ending in s/x/z/ch/sh', () => {
    expect(pluralize('Ingress')).toBe('ingresses');
    expect(pluralize('EndpointSlice')).toBe('endpointslices');
  });

  it('replaces a trailing consonant+y with ies', () => {
    expect(pluralize('NetworkPolicy')).toBe('networkpolicies');
  });

  it('does not treat a trailing vowel+y as a consonant+y', () => {
    expect(pluralize('Gateway')).toBe('gateways');
  });
});
