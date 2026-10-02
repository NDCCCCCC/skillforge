// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// Quick 260929-d5w2 — prettySourcePath unit tests. The locked example
// from the task spec is case 1: the renamed 032 sec-file renders the
// short "base · section" form.
import { describe, expect, it } from 'vitest';
import { prettySourcePath } from './pretty-source';

describe('prettySourcePath', () => {
  it('splits the __secNN suffix into a " · " section label (locked example)', () => {
    expect(prettySourcePath('sql-guides/取数口径/032_保单号查支付方式与开户行__sec02_SQL_正文.md')).toBe(
      '032_保单号查支付方式与开户行 · SQL正文',
    );
  });

  it('returns the bare basename (extension stripped) for plain docs', () => {
    expect(prettySourcePath('sql-guides/取数口径/024_团客非车保单提数口径.md')).toBe(
      '024_团客非车保单提数口径',
    );
    expect(prettySourcePath('authority/annual_leave.md')).toBe('annual_leave');
  });

  it('drops underscores in the section label (sec01 echo form)', () => {
    expect(
      prettySourcePath('操作手册/012_定时任务桌面程序使用手册__sec01_定时任务桌面程序_-_完整使用手册.md'),
    ).toBe('012_定时任务桌面程序使用手册 · 定时任务桌面程序-完整使用手册');
  });

  it('passes through inputs without a directory, extension, or sec suffix', () => {
    expect(prettySourcePath('contract-approval.md')).toBe('contract-approval');
    expect(prettySourcePath('sql-guides/取数口径/030_承保清单')).toBe('030_承保清单');
  });
});
