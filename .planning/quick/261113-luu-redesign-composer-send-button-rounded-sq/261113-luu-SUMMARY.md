---
status: complete
task: 261113-luu
created: 2026-11-13
completed: 2026-11-13
---

# 261113-luu SUMMARY — composer 发送按钮视觉重设计

## 改动

`web/src/styles/theme.css` 的 `.composer-send` 全套重写（仅 CSS，未动 TSX）：

| 维度 | 旧 | 新 |
|------|-----|-----|
| 形状 | 正圆 `--radius-full` | 圆角方形 `--radius-md`（10px，呼应 composer 圆角语言） |
| 尺寸 | 40×40 | 38×38（图标 16→18px，stroke-width 2.25） |
| 启用态 | 实心 `--color-primary` | 135° primary→primary-hover 渐变 + 内高光 + 10px 光晕 |
| hover | 换色 + 上移 1px | 上移 1px + 放大 1.04 + 强光晕 |
| active | 无 | 缩小 0.98 回弹 |
| focus-visible | **无** | 2px primary-hover 焦点环 + 2px offset |
| 禁用态 | **实心 surface-3 灰圆**（抢眼的大色块） | 幽灵态：透明底 + 细描边 + 淡图标 + opacity .55 |

## 验证

- `npx vitest run src/__tests__/ChatInput.test.tsx` → 14/14 pass
- `npm run typecheck` → exit 0；`npm run lint` → exit 0
- 浏览器实测（localhost:8888，ego-browser）：读取计算样式确认 btnRect 38×38、borderRadius 10px、backgroundImage 为 primary→primary-hover 渐变、svg 18×18、disabled=false；禁用态截图确认为幽灵描边样式

## 提交

- 待编排方提交（CSS 单文件）
