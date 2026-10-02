// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Loads @testing-library/jest-dom's custom matchers (toBeInTheDocument,
// toHaveAttribute, etc.) onto vitest's expect. Single-line setup per
// the vitest + testing-library recipe.
import '@testing-library/jest-dom';

// jsdom lacks ResizeObserver (OntologyGraphView uses it for the cytoscape
// zero-height guard). Minimal stand-in: constructor no-op, observe/unloop
// never fire, disconnect/unobserve no-op.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
if (typeof globalThis.ResizeObserver === 'undefined') {
  (globalThis as Record<string, unknown>).ResizeObserver = ResizeObserverStub;
}
