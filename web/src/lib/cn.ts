// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// cn() — standard shadcn/ui className utility. Combines clsx (conditional
// classes) with tailwind-merge (last-write-wins on conflicting Tailwind
// utilities). Required by all shadcn-generated components.
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
