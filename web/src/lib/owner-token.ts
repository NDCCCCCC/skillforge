// Copyright 2026 SkillForge Authors
// SPDX-License-Identifier: Apache-2.0

// 最小身份层（owner token）—— 前端唯一入口。
//
// 后端引入最小身份层后：每个浏览器持有一个 owner token，后续请求
// 通过请求头 `X-Owner-Token` 发送；会话列表只返回归属自己的会话。
// 因此前端需要一个"这个浏览器是谁"的稳定标识。
//
// 为什么放在 lib/ 而不是各自组件：项目里有几十处发往后端的请求，
// 逐个加请求头既容易漏、也难以复核。apiFetch（lib/api.ts）与
// RealChatTransport（transport/RealChatTransport.ts）是仅有的两个
// 收口 choke point —— 在这两处注入，所有调用点自动获得身份头，
// 无需改动任何调用方。
//
// 为什么落 localStorage：token 必须在同一浏览器的不同会话之间保持
// 稳定（刷新后仍要能看到自己的历史会话），sessionStorage 做不到；
// 它不是凭证（服务端只用它做归属过滤，不做鉴权授权），所以不需要
// 额外的加密保护。

/** localStorage 键名 —— 与后端约定的前端存储键一致。 */
export const OWNER_TOKEN_STORAGE_KEY = 'sf_owner_token';

/**
 * 内存兜底：localStorage 不可用时（隐私模式 / 存储被禁用），
 * 至少保证本次页面生命周期内 token 稳定 —— 否则每个请求都会拿到
 * 一个新 token，后端会把它当成新用户，会话永远对不上。
 */
let memoryToken: string | null = null;

/**
 * 生成一个随机 token。
 *
 * 首选 crypto.randomUUID()；非安全上下文（http 局域网调试）或老
 * 浏览器没有 randomUUID 时回落到 getRandomValues 拼 32 位 hex；
 * 两者都不可用时用 Math.random + 时间戳兜底（仅剩这一种极端情况，
 * 仍优于让身份每次请求都变）。
 */
function generateOwnerToken(): string {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === 'function') {
    return c.randomUUID();
  }
  if (c && typeof c.getRandomValues === 'function') {
    const bytes = new Uint8Array(16);
    c.getRandomValues(bytes);
    return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  }
  return `${Math.random().toString(16).slice(2)}${Date.now().toString(16)}`;
}

/**
 * 读取当前浏览器的 owner token；不存在时生成一个并持久化。
 *
 * 惰性生成是刻意的：token 只在被真正需要（第一次发请求）时才写入，
 * 免得用户还没开始对话，localStorage 里就先多了一个身份。
 */
export function getOwnerToken(): string {
  try {
    const stored = window.localStorage.getItem(OWNER_TOKEN_STORAGE_KEY);
    if (stored && stored.length > 0) {
      return stored;
    }
  } catch {
    // localStorage 不可用 → 走内存兜底
  }

  const token = memoryToken ?? generateOwnerToken();
  memoryToken = token;
  try {
    window.localStorage.setItem(OWNER_TOKEN_STORAGE_KEY, token);
  } catch {
    // 存储被禁用 —— 内存兜底已保证本次会话内 token 稳定
  }
  return token;
}

/**
 * owner token 的请求头。调用方用 spread 合并进自己的 headers。
 * 单独导出是为了让「哪些请求带身份头」在一处可查。
 */
export function ownerTokenHeaders(): Record<string, string> {
  return { 'X-Owner-Token': getOwnerToken() };
}
