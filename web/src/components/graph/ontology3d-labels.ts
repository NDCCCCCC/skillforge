// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-k3m follow-up — always-on node labels for the 3D ontology
// graph. Lives in its own module so `three` stays inside the lazy chunk:
// OntologyGraphView (main chunk) imports this via dynamic import() and
// therefore never pulls three into the eager bundle.
//
// A label is a THREE.Sprite with a canvas-texture (white halo + dark text)
// so it stays readable over any background and always faces the camera.
// Textures are cached per label text; the Sprite itself is per call
// (a Sprite cannot be shared between two parents in the scene graph).

import * as THREE from 'three';

interface CachedLabel {
  tex: THREE.CanvasTexture;
  w: number;
  h: number;
}

const cache = new Map<string, CachedLabel>();

const FONT_PX = 42;
const PAD_PX = 10;
/** World units per texture pixel — tuned for the default camera distance. */
const PX_TO_WORLD = 0.055;

function measure(text: string): { w: number; h: number } {
  const ctx = document.createElement('canvas').getContext('2d');
  const font = `600 ${FONT_PX}px 'Inter Variable', Inter, system-ui, sans-serif`;
  if (!ctx) return { w: text.length * FONT_PX, h: FONT_PX + PAD_PX * 2 };
  ctx.font = font;
  return {
    w: Math.ceil(ctx.measureText(text).width) + PAD_PX * 2,
    h: FONT_PX + PAD_PX * 2,
  };
}

/**
 * Build a camera-facing label sprite for `text`, positioned just below a
 * sphere of radius `sphereRadius` (world units).
 */
export function makeLabelSprite(text: string, sphereRadius: number): THREE.Sprite {
  let entry = cache.get(text);
  if (!entry) {
    const { w, h } = measure(text);
    const cv = document.createElement('canvas');
    cv.width = w;
    cv.height = h;
    const ctx = cv.getContext('2d');
    if (ctx) {
      ctx.font = `600 ${FONT_PX}px 'Inter Variable', Inter, system-ui, sans-serif`;
      ctx.textBaseline = 'alphabetic';
      // Soft white halo so the label survives overlapping edges.
      ctx.strokeStyle = 'rgba(255,255,255,0.92)';
      ctx.lineWidth = 9;
      ctx.lineJoin = 'round';
      ctx.strokeText(text, PAD_PX, h - PAD_PX - 6);
      ctx.fillStyle = '#111827';
      ctx.fillText(text, PAD_PX, h - PAD_PX - 6);
    }
    const tex = new THREE.CanvasTexture(cv);
    tex.colorSpace = THREE.SRGBColorSpace;
    entry = { tex, w, h };
    cache.set(text, entry);
  }

  const mat = new THREE.SpriteMaterial({
    map: entry.tex,
    transparent: true,
    depthWrite: false,
  });
  const sprite = new THREE.Sprite(mat);
  sprite.scale.set(entry.w * PX_TO_WORLD, entry.h * PX_TO_WORLD, 1);
  sprite.position.set(0, -(sphereRadius + entry.h * PX_TO_WORLD * 0.5 + 2), 0);
  return sprite;
}
