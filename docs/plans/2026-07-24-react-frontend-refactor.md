# React Frontend Production Refactor

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor the chat UI frontend to handle all Go agent SSE event types, add dark mode, tool cards, citations, agent badges, smart scroll, and production-quality patterns.

**Architecture:** Event-driven SSE consumer pattern with React 19 functional components. All SSE event types are parsed in chat.api.ts, state is managed in ChatPage.tsx via useState/useReducer, and rendered through specialized presentational components. Tailwind v3 with CSS variables for dark mode. React Compiler auto-memoizes.

**Tech Stack:** React 19.2, React Router 7, Tailwind CSS 3.4, Vite 8, TypeScript 5.7, react-markdown + remark-gfm

---

### Task 1: Update SSE event types in chat.api.ts

**Files:**
- Modify: `apps/web/src/modules/chat/chat.api.ts`

**Changes:**
- Expand `ChatEvent` type to cover all Go event types: step, agent, memory, interrupt, citation, done (with usage)
- Add `AgentEvent`, `ToolCallState`, `CitationData` types
- Add usage tracking on `done`

**New types:**
```typescript
export type AgentEvent = {
  type: "agent";
  name: string;       // "general" | "code" | "research"
  message?: string;   // "Research Agent is searching..."
};

export type CitationData = {
  title: string;
  url?: string;
  snippet?: string;
};

export type UsageData = {
  inputTokens: number;
  outputTokens: number;
};

export type ChatEvent = {
  type: "step" | "text" | "tool_start" | "tool_end" | "citation" | "memory" | "agent" | "interrupt" | "error" | "done";
  node?: string;          // step: current node ID
  text?: string;          // text: token or citation: sources JSON
  name?: string;          // tool_start/tool_end/interrupt: tool name
  message?: string;       // error/memory/interrupt: detail message
  usage?: UsageData;      // done: token usage
};
```

### Task 2: Create ThemeToggle component

**Files:**
- Create: `apps/web/src/shared/components/ThemeToggle.tsx`

**Component:** Toggle button with sun/moon icons. Uses CSS class toggle on `<html>`. Persists to localStorage.

### Task 3: Create AgentBadge component

**Files:**
- Create: `apps/web/src/shared/components/AgentBadge.tsx`

**Component:** Small badge showing agent name with appropriate icon/color. Animates in when agent changes. Props: `agent: string | null`, `message?: string`.

### Task 4: Create ToolCallCard component

**Files:**
- Create: `apps/web/src/shared/components/ToolCallCard.tsx`

**Component:** Expandable card showing tool execution. States: running (spinner), completed (checkmark + result preview), error (red X + message). Props: `tool: { name: string; status: "running" | "done" | "error"; result?: string; error?: string }`.

### Task 5: Create CitationList component

**Files:**
- Create: `apps/web/src/shared/components/CitationList.tsx`

**Component:** List of clickable citation links at bottom of assistant message. Props: `citations: CitationData[]`.

### Task 6: Create SuggestionChips component

**Files:**
- Create: `apps/web/src/shared/components/SuggestionChips.tsx`

**Component:** Extract suggestion logic from EmptyState into reusable component. Props: `suggestions: string[]`, `onPick: (s: string) => void`.

### Task 7: Add dark mode CSS variables

**Files:**
- Modify: `apps/web/src/index.css`
- Modify: `apps/web/tailwind.config.js`

**Changes:**
- Add CSS custom properties for all colors in `:root` and `.dark`
- Add dark variant in tailwind config `darkMode: "class"`
- Add dark mode scrollbar styles
- Add dark mode prose styles

### Task 8: Update http.ts with retry, timeout, interceptors

**Files:**
- Modify: `apps/web/src/shared/api/http.ts`

**Changes:**
- Add configurable timeout (default 30s)
- Add simple retry logic (3 retries on network error with backoff)
- Add request/response interceptors pattern
- Better error messages with status code context

### Task 9: Refactor ChatPage.tsx for all event types

**Files:**
- Modify: `apps/web/src/modules/chat/components/ChatPage.tsx`

**Changes:**
- Track `activeAgent` from `agent` events
- Track `toolCalls` map (not just current tool name)
- Track `citations` array
- Show token usage on done
- Smart auto-scroll (respect user scroll position)
- Pass new event data to MessageBubble
- Add stop generation button

### Task 10: Refactor MessageBubble.tsx

**Files:**
- Modify: `apps/web/src/modules/chat/components/MessageBubble.tsx`

**Changes:**
- Accept and render `toolCalls` as ToolCallCard list
- Accept and render `citations` as CitationList
- Accept and render `agentName` as AgentBadge
- Add simple code block copy button
- Improve streaming caret animation

### Task 11: Add icons for new components

**Files:**
- Modify: `apps/web/src/shared/components/icons.tsx`

**Changes:**
- Add `SunIcon`, `MoonIcon`, `SearchIcon`, `AgentIcon`, `StopIcon`, `LinkIcon`, `WrenchIcon`, `BrainIcon`

### Task 12: Modernize Sidebar with search and skeleton

**Files:**
- Modify: `apps/web/src/shared/components/Sidebar.tsx`

**Changes:**
- Add search/filter input for conversations
- Add loading skeleton when conversations are empty and loading
- Add conversation rename (inline edit on double-click)
- Better mobile drawer overlay animation

### Task 13: Update App.tsx with ThemeProvider

**Files:**
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/main.tsx`

**Changes:**
- Add ThemeProvider context wrapping the app
- Initialize theme from localStorage or system preference
- Add ThemeToggle in layout

### Task 14: Update EmptyState to use SuggestionChips

**Files:**
- Modify: `apps/web/src/modules/chat/components/EmptyState.tsx`

**Changes:**
- Replace inline suggestion buttons with SuggestionChips component
- Add JARVIS-themed greeting

### Task 15: Typecheck and build

**Commands:**
```bash
pnpm --filter @app/web typecheck
pnpm --filter @app/web build
```

### Task 16: Commit

```bash
git add apps/web/
git commit -m "feat(web): production refactor - SSE events, dark mode, tool cards, citations"
```

---
