# gogomail Frontend Design Language

This document defines the visual and interaction design direction for the gogomail webmail frontend.
All frontend agents and developers must read this file before writing any UI code.

**Design direction**: Notion Mail + Gmail 의 장점을 결합한 오픈소스 메일 클라이언트.
Notion Mail의 깔끔한 미니멀 UI · AI 트리아지 방향성, Gmail의 검증된 키보드 UX · 폴더 체계를 합친다.
Linear · Superhuman의 스팟라이트 검색 · 커맨드 팔레트 패턴을 원본 구현으로 탑재.
어떤 상업 제품의 에셋, CSS, 색상값도 복제하지 않는다.

---

## Core Design Principles

1. **Content-first**: Maximum space for the user's content. Minimal chrome, no decorative elements.
2. **Progressive disclosure**: Secondary actions (archive, delete, label) appear on hover, not always visible.
3. **High density without clutter**: Show enough information per row without cramming. ~52px row height for message list.
4. **Keyboard-centric**: Every action must have a keyboard shortcut. Shortcuts are discoverable via `?`.
5. **Dark mode is first-class**: Not an afterthought. Every token must have both light and dark values.
6. **Neutral base, single accent**: The UI is neutral gray/white. One brand accent color for interactive elements.

---

## Layout

### Shell Layout (2-pane + slide-in overlay)

```
┌─────────────┬──────────────────────────────────────────────────────────┐
│  Sidebar    │      Message List (full remaining width)                 │
│  ~220px     │      scrollable, always visible                          │
│  fixed      │                                                          │
└─────────────┴──────────────────────────────────────────────────────────┘
                                         ↑ Reading pane slides in from right
                                           as fixed overlay when msg selected
```

- **Sidebar**: Fixed width (~220px), collapsible with `[` key. Vertical scroll for long nav.
- **Message list**: Fills remaining space. Always visible — never replaced by reading pane.
- **Reading pane**: Fixed-position slide-in overlay from the right. Appears on message selection, closes on `Escape` or back button. Does NOT split the layout.
- **Responsive**: Below 768px, reading pane is full-screen. Message list remains accessible via back navigation.
- **Compose**: Floating modal, overlays content. Does not push any layout element.
- **DO NOT** implement a persistent 3-pane split layout. The slide-in overlay reading pane is the canonical behavior.

### Sidebar Structure

```
[Avatar] [Display Name]     ← account selector, top
[Search]                    ← global search input

VIEWS
  Inbox            99+
  Promotions       99+
  Labels           99+
  Social           28

MAIL
  All Mail
  Sent
  Drafts
  Trash

[App Name]                  ← app section if multi-app (calendar, contacts, drive)
  Calendar
  Contacts

[Settings]
[Help & Feedback]
```

- Section labels: uppercase, 11px, muted gray, not clickable
- Active item: subtle background fill (not a left border stripe)
- Unread badge: right-aligned, rounded pill, gray background

---

## Color Tokens

Define as CSS custom properties. Both themes must be set.

```css
/* Light mode */
:root {
  --color-bg-primary: #FFFFFF;
  --color-bg-secondary: #F7F7F5;      /* sidebar, hover */
  --color-bg-tertiary: #EFEEEB;       /* pressed, selected */
  --color-bg-overlay: rgba(0,0,0,0.04);

  --color-text-primary: #1A1A1A;
  --color-text-secondary: #6B6B6B;    /* preview text, dates */
  --color-text-tertiary: #9B9B9B;     /* placeholders, disabled */

  --color-border-subtle: #E8E8E5;     /* dividers */
  --color-border-default: #D5D5D0;

  --color-accent: #2F6EE0;            /* unread dot, links, primary button */
  --color-accent-hover: #2560C8;
  --color-accent-subtle: #EBF1FD;     /* selected row bg */

  --color-destructive: #D94F3D;
  --color-success: #2D9E5F;
  --color-warning: #E8A838;
}

/* Dark mode — applied via [data-theme="dark"] on <html> */
[data-theme="dark"] {
  --color-bg-primary: #191919;
  --color-bg-secondary: #1F1F1F;
  --color-bg-tertiary: #2A2A2A;
  --color-bg-overlay: rgba(255,255,255,0.04);

  --color-text-primary: #E8E8E5;
  --color-text-secondary: #8B8B88;
  --color-text-tertiary: #5B5B58;

  --color-border-subtle: #2E2E2B;
  --color-border-default: #3A3A37;

  --color-accent: #5B8EF0;
  --color-accent-hover: #6B9AF4;
  --color-accent-subtle: #1E2B45;

  --color-destructive: #E8675A;
  --color-success: #3DB870;
  --color-warning: #F0B84A;
}
```

---

## Typography

Font: **Inter** (primary). Fallback: `-apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`.

```css
/* Scale */
--text-xs:   11px / 1.4   /* badges, labels */
--text-sm:   13px / 1.5   /* secondary UI, dates */
--text-base: 14px / 1.6   /* body, default */
--text-md:   15px / 1.6   /* message list sender */
--text-lg:   18px / 1.4   /* reading pane subject */
--text-xl:   22px / 1.3   /* page titles */

/* Weight */
--weight-normal: 400
--weight-medium: 500
--weight-semibold: 600
```

**Message list row typography**:
- Sender: 14px, weight 500 (unread) / 400 (read), `var(--color-text-primary)`
- Subject: 14px, weight 600 (unread) / 400 (read), `var(--color-text-primary)`
- Preview: 14px, weight 400, `var(--color-text-secondary)`, truncated after subject with `·` separator
- Date: 13px, weight 400, `var(--color-text-secondary)`, right-aligned

---

## Spacing

4px base grid. Use multiples: 4, 8, 12, 16, 20, 24, 32, 40, 48.

```
Sidebar padding:          16px horizontal
Message list row:         12px vertical, 16px horizontal
Reading pane padding:     24px horizontal, 20px vertical
Compose modal padding:    16px
Section label margin-top: 20px
```

---

## Component Patterns

### Message List Row

```
┌──────────────────────────────────────────────────────────────────────┐
│ □  ● Sender Name      Subject bold · preview text gray ...    date  │
└──────────────────────────────────────────────────────────────────────┘
```

States:
- **Default (unread)**: white bg, sender+subject bold, blue dot visible
- **Read**: white bg, normal weight, no dot
- **Hover**: `var(--color-bg-secondary)` background, checkbox appears, action icons appear (archive, delete, snooze) on far right
- **Selected**: `var(--color-accent-subtle)` background
- **Pressed/active**: `var(--color-bg-tertiary)`

Date grouping separators: `오늘` / `어제` / `지난 7일` / `이번 달` — 12px, `var(--color-text-tertiary)`, 12px top margin.

### Unread Indicator

6px circle, `var(--color-accent)`, positioned left of sender. Disappears on read. Animated fade-out.

### Reading Pane

```
[← prev] [→ next]          [archive] [label] [snooze] [delete] [···]
─────────────────────────────────────────────────────────────────────
Subject line (text-lg, weight 600)

[Label tag] [Label tag]

Sender Name <email@domain.com>        Reply ↩  Forward ↪   date
수신인: 나  ▾
─────────────────────────────────────────────────────────────────────
[HTML email body rendered here]
```

- Label tags: small rounded pill, light background matching label color
- Sender metadata: 13px, muted
- HTML body: sandboxed iframe or sanitized render, max-width 680px, centered

### Compose Window

Floating modal, min-width 520px, position: fixed bottom-right (desktop) or centered (tablet/mobile).

```
┌──────────────────────────────────────────────────────────┐
│  From: Display Name <email>                    [─] [×]   │
│  To: _________________________________ Cc/Bcc            │
│  Subject: _____________________________________________   │
│  ·····                                                    │
│                                                          │
│  [body area, plain text + markdown or rich text]         │
│                                                          │
│  ─────────────────────────────────────────────────────   │
│  [📎] [B I U ···]                [🗑] [Send ▾]          │
└──────────────────────────────────────────────────────────┘
```

- Send button: primary blue, rounded, weight 500
- Send dropdown: schedule send, send later options
- Auto-save draft: debounced 2s after last keystroke, "Draft saved" indicator

### Settings Modal

Full-screen overlay with centered card (max-width 720px).

Left sub-navigation (180px) + right content panel. No page reload — all client-side navigation.

Settings categories: 수신함 설정 / 계정 / 알림 / 서명 / 테마 / 단축키 / ...

Toggle switches: custom-styled, `var(--color-accent)` when on.
Select dropdowns: clean, no native OS styling.

### Labels / Tags

Color-coded pill: 6px dot + text, or just colored pill.
Colors: 6-8 preset label colors, user-assignable.
Pill style: `border-radius: 4px`, `padding: 2px 8px`, `font-size: 12px`.

---

## Internationalization (i18n)

**Supported locales (initial)**: `ko` (한국어), `en` (English), `ja` (日本語), `zh-CN` (简体中文)

Designed for extension: adding a new locale requires only a translation file — no code changes.

**Library**: `next-intl` (Next.js App Router compatible, ICU message format)

**Locale resolution order** (highest priority first):
1. User preference — stored via Runtime Config Store `user:{id}` namespace, key `ui.locale`
2. Domain default locale — set by domain admin via `domain:{id}` namespace, key `ui.default_locale`
3. Browser `Accept-Language` header (first-visit fallback)
4. System fallback: `en`

**Translation files**: `messages/{locale}.json` (ICU message format)

```
messages/
  ko.json
  en.json
  ja.json
  zh-CN.json
```

**Rules**:
- All user-visible strings must use translation keys. No hardcoded Korean strings in JSX.
- Dates/times: use `Intl.DateTimeFormat` with the active locale. Never format manually.
- Numbers/currency: use `Intl.NumberFormat` with locale.
- Pluralization: use ICU `{count, plural, one{# item} other{# items}}` syntax.
- RTL: not required for initial 4 locales, but do not use `margin-left`/`margin-right` for directional spacing — use `margin-inline-start`/`margin-inline-end`.
- Locale switcher: available in user settings (환경설정 → 언어). Applies immediately without page reload.
- Server components: locale resolved from cookie/header, passed via `next-intl` provider.

**Key namespaces** (translation file structure):
```json
{
  "common": { "save": "저장", "cancel": "취소", ... },
  "mail": { "inbox": "수신함", "compose": "편지 쓰기", ... },
  "calendar": { "newEvent": "새 이벤트", ... },
  "contacts": { "newContact": "새 연락처", ... },
  "drive": { "upload": "업로드", ... },
  "settings": { "language": "언어", "theme": "테마 모드", ... },
  "auth": { "login": "로그인", "mfa.prompt": "인증 코드 입력", ... }
}
```

---

## Dark / Light Mode Switching

- Toggle in Settings (테마 모드: 라이트 / 다크 / 시스템)
- Stored in `localStorage` and user preferences (via Runtime Config Store API)
- Applied via `[data-theme="dark"]` on `<html>` element
- No flash of wrong theme on load (SSR-safe: read preference before first paint)
- Transition: `transition: background-color 200ms, color 200ms` on root elements only

---

## Motion & Transitions

Keep animations subtle and fast. Do not animate layout shifts.

```
Hover states:      background 100ms ease
Panel slide-in:    transform + opacity 150ms ease-out
Modal overlay:     opacity 120ms ease
Compose open:      scale(0.97→1) + opacity 120ms ease-out
Row actions:       opacity 80ms ease
```

Respect `prefers-reduced-motion`: disable all transitions/animations when set.

---

## Icons

Use a single consistent icon set (Lucide or Radix Icons recommended).
Size: 16px for inline UI, 18px for toolbar actions, 20px for sidebar nav.
Stroke width: 1.5px. Color: inherits from `currentColor`.
Do not mix icon sets.

---

## Accessibility

- All interactive elements: visible focus ring (`outline: 2px solid var(--color-accent); outline-offset: 2px`)
- Color contrast: WCAG AA minimum (4.5:1 for body text, 3:1 for large text)
- Keyboard navigation: Tab / Shift-Tab for all controls, Enter/Space for activation
- Screen reader: semantic HTML (`<nav>`, `<main>`, `<aside>`, `role="listbox"` etc.)
- ARIA live regions for unread count updates

---

## What to Avoid

- Do not use full-page navigation for reading a message (SPA, no page reload)
- Do not use heavy drop shadows (max `box-shadow: 0 1px 3px rgba(0,0,0,0.08)`)
- Do not use decorative gradients or background images
- Do not use more than 2 accent colors simultaneously
- Do not animate list items on scroll
- Do not copy exact layout pixel-values, color hex codes, or CSS class names from Notion Mail
- Do not use Notion's proprietary fonts or icon set

---

## Tech Stack (Frontend)

- **Framework**: Next.js 16 (App Router), TypeScript
- **Styling**: Tailwind CSS v4 with CSS custom properties for tokens
- **Components**: shadcn/ui as base, customized to this design system
- **State**: Zustand or Jotai for client state, TanStack Query for server state
- **Testing**: Vitest + Testing Library for components, Playwright for E2E
- **Monorepo layout**:
  ```
  apps/webmail/          ← main webmail app
  apps/console/            ← admin console
  packages/ui/           ← shared design system components
  packages/api-client/   ← typed API client from openapi.yaml
  ```

---

*Last updated: 2026-05-31 (reviewed; no design contract changes)*
*Reference images: Notion Mail UI screenshots (design inspiration only — original implementation required)*
