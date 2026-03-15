# Bloomberg Terminal — Phase I: Panel Linking, Keyboard Shortcuts & AI Context

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Polish the Bloomberg terminal workspace with three interconnected quality-of-life features: (1) panel linking — panels sharing a color group automatically sync their active ticker; (2) global keyboard shortcuts — `Ctrl+G` focuses the command bar, `Escape` dismisses suggestions, `Ctrl+W` closes the active panel; (3) AI context awareness — the AI chat panel displays and uses the ticker from the panel it lives in; (4) workspace auto-save — debounced persistence of layout changes to the backend.

**Architecture:** All changes are pure frontend. No new backend routes or Go handler changes are required. The `workspaceStore.ts` gains one new action (`setLinkGroupTicker`). A new hook `useWorkspaceAutoSave` is introduced in `frontend/src/hooks/`. `WorkspaceLayout.tsx` coordinates global keyboard events via a single `useEffect`. `PanelSlot.tsx` gains an interactive link group dot and an inline ticker editor. `ChatPanel.tsx` gains an optional `contextTicker` prop. No new npm packages are needed.

**Tech Stack:** Zustand (`workspaceStore`), React `useRef` / `useEffect` / `useCallback`, React Query `useMutation`, existing `api/terminal.ts`, Tailwind, lucide-react.

**Two apps, one codebase:**
- `/` — Old trading workbench (untouched)
- `/terminal` — Bloomberg workspace (all Phase I changes are here)

---

## Task I1: Add `setLinkGroupTicker` to workspaceStore

**Files:**
- Modify: `frontend/src/stores/workspaceStore.ts`

This new action finds every panel in the active workspace that shares the given `linkGroup` and calls `updatePanel` on each with the new ticker. The action is a pure Zustand `set` — no async work, no side effects beyond store mutation.

**Step 1: Add the method signature to the `WorkspaceStore` interface**

In `workspaceStore.ts`, locate the `interface WorkspaceStore` block. Add the new method directly after `removePanel`:

```typescript
  // Panel linking
  setLinkGroupTicker: (linkGroup: NonNullable<LinkGroup>, ticker: string) => void
```

The full interface excerpt should read:

```typescript
interface WorkspaceStore {
  workspaces: WorkspaceConfig[]
  activeIndex: number
  activePanelId: string | null

  setActivePanel: (id: string | null) => void

  setActiveWorkspace: (index: number) => void
  addWorkspace: (name?: string) => void
  removeWorkspace: (index: number) => void
  renameWorkspace: (index: number, name: string) => void

  updateLayout: (layout: GridItem[]) => void

  addPanel: (config: Omit<PanelConfig, 'id'>) => string
  updatePanel: (panelId: string, update: Partial<PanelConfig>) => void
  removePanel: (panelId: string) => void

  // Panel linking
  setLinkGroupTicker: (linkGroup: NonNullable<LinkGroup>, ticker: string) => void

  loadServerWorkspaces: (workspaces: WorkspaceConfig[]) => void
}
```

**Step 2: Implement the action in the `create` call**

Inside the `create<WorkspaceStore>()(persist((set, get) => ({ ... })))` block, add the implementation immediately after the `removePanel` implementation:

```typescript
      setLinkGroupTicker: (linkGroup, ticker) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          const ws = workspaces[s.activeIndex]
          const updatedPanels = { ...ws.panels }
          Object.values(updatedPanels).forEach((panel) => {
            if (panel.linkGroup === linkGroup) {
              updatedPanels[panel.id] = { ...panel, ticker }
            }
          })
          workspaces[s.activeIndex] = { ...ws, panels: updatedPanels }
          return { workspaces }
        }),
```

**Step 3: Verify TypeScript compiles**

```bash
docker compose exec frontend npx tsc --noEmit 2>&1 | grep -E "workspaceStore|setLinkGroup"
```

Expected: no output (no errors).

**Commit:**
```bash
git add frontend/src/stores/workspaceStore.ts
git commit -m "feat(terminal): add setLinkGroupTicker action to workspaceStore"
```

---

## Task I2: Make the link group dot interactive in PanelSlot

**Files:**
- Modify: `frontend/src/components/terminal/PanelSlot.tsx`

The dot is already rendered when `config.linkGroup` is non-null. Phase I makes it: (a) always visible as a faint grey dot when unlinked; (b) clickable to cycle through `null → red → blue → green → yellow → null`; (c) calls `onUpdate` with the new `linkGroup`. The `onUpdate` prop already exists but was unused (aliased to `_`).

**Step 1: Define the link group cycle order**

At the top of `PanelSlot.tsx`, after the `LINK_COLORS` constant, add:

```typescript
const LINK_CYCLE: LinkGroup[] = [null, 'red', 'blue', 'green', 'yellow']

function nextLinkGroup(current: LinkGroup): LinkGroup {
  const idx = LINK_CYCLE.indexOf(current ?? null)
  return LINK_CYCLE[(idx + 1) % LINK_CYCLE.length]
}
```

**Step 2: Fix the `onUpdate` alias — remove the underscore**

Change the destructuring in the function signature from:

```typescript
export function PanelSlot({ config, isActive, onFocus, onClose, onUpdate: _ }: PanelSlotProps) {
```

to:

```typescript
export function PanelSlot({ config, isActive, onFocus, onClose, onUpdate }: PanelSlotProps) {
```

**Step 3: Replace the static link group dot with a clickable button**

In the JSX, replace the existing link group indicator block:

```tsx
        {/* Link group indicator */}
        {config.linkGroup && (
          <span className={cn('w-2 h-2 rounded-full shrink-0', LINK_COLORS[config.linkGroup])} />
        )}
```

with:

```tsx
        {/* Link group indicator — click to cycle */}
        <button
          className={cn(
            'w-3 h-3 rounded-full shrink-0 border transition-colors',
            config.linkGroup
              ? cn(LINK_COLORS[config.linkGroup], 'border-transparent')
              : 'bg-transparent border-muted-foreground/30 hover:border-muted-foreground/60',
          )}
          onClick={(e) => {
            e.stopPropagation()
            onUpdate({ linkGroup: nextLinkGroup(config.linkGroup ?? null) })
          }}
          title={config.linkGroup ? `Link group: ${config.linkGroup} — click to change` : 'Click to assign link group'}
        />
```

**Step 4: Add inline ticker editing**

Directly after the link group button, replace the static ticker badge:

```tsx
        {/* Ticker badge */}
        {config.ticker && (
          <span className="text-xs font-mono font-semibold text-foreground bg-muted px-1.5 py-0.5 rounded">
            {config.ticker}
          </span>
        )}
```

with an editable ticker badge that becomes a text input on click:

```tsx
        {/* Ticker badge — click to edit inline */}
        <TickerBadge
          ticker={config.ticker}
          onCommit={(newTicker) => onUpdate({ ticker: newTicker })}
        />
```

**Step 5: Add the `TickerBadge` sub-component**

Add this component above the `PanelSlot` function definition (still within the same file):

```typescript
interface TickerBadgeProps {
  ticker: string
  onCommit: (ticker: string) => void
}

function TickerBadge({ ticker, onCommit }: TickerBadgeProps) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(ticker)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (editing) {
      setDraft(ticker)
      inputRef.current?.select()
    }
  }, [editing, ticker])

  const commit = () => {
    const trimmed = draft.trim().toUpperCase()
    if (trimmed && trimmed !== ticker) onCommit(trimmed)
    setEditing(false)
  }

  if (!ticker) return null

  if (editing) {
    return (
      <input
        ref={inputRef}
        className="text-xs font-mono font-semibold bg-muted text-foreground px-1.5 py-0.5 rounded outline-none ring-1 ring-blue-500 w-20"
        value={draft}
        onChange={(e) => setDraft(e.target.value.toUpperCase())}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === 'Enter') { e.preventDefault(); commit() }
          if (e.key === 'Escape') { e.stopPropagation(); setEditing(false) }
        }}
        onClick={(e) => e.stopPropagation()}
        spellCheck={false}
        autoComplete="off"
      />
    )
  }

  return (
    <span
      className="text-xs font-mono font-semibold text-foreground bg-muted px-1.5 py-0.5 rounded cursor-pointer hover:bg-muted/80"
      onClick={(e) => { e.stopPropagation(); setEditing(true) }}
      title="Click to edit ticker"
    >
      {ticker}
    </span>
  )
}
```

**Step 6: Add `useRef` to the imports at the top of `PanelSlot.tsx`**

The existing import is:
```typescript
import { useState } from 'react'
```

Change it to:
```typescript
import { useState, useEffect, useRef } from 'react'
```

**Step 7: Wire up link group sync in PanelGrid**

`PanelGrid.tsx` already passes `onUpdate={(update) => updatePanel(item.i, update)}` to each `PanelSlot`. We need it to also call `setLinkGroupTicker` whenever the update includes a `ticker` field and the panel has a `linkGroup`.

Open `frontend/src/components/terminal/PanelGrid.tsx`. Add `setLinkGroupTicker` to the store subscriptions:

```typescript
  const setLinkGroupTicker = useWorkspaceStore((s) => s.setLinkGroupTicker)
```

Then replace the `onUpdate` prop passed to `PanelSlot`:

```tsx
              onUpdate={(update) => {
                updatePanel(item.i, update)
                // If ticker changed and panel has a link group, sync all linked panels
                if (update.ticker && config.linkGroup) {
                  setLinkGroupTicker(config.linkGroup, update.ticker)
                }
              }}
```

**Verification:**
```bash
docker compose exec frontend npx tsc --noEmit 2>&1 | grep -E "PanelSlot|PanelGrid|TickerBadge"
```
Expected: no output.

Open the terminal at `http://localhost:5173/terminal`. Assign two panels to the same link group (e.g. red) by clicking their dots until red. Click the ticker badge on one panel, type a new ticker, press Enter. Confirm the other panel's ticker badge updates immediately.

**Commit:**
```bash
git add frontend/src/components/terminal/PanelSlot.tsx \
        frontend/src/components/terminal/PanelGrid.tsx
git commit -m "feat(terminal): interactive link group dot and inline ticker editing in PanelSlot"
```

---

## Task I3: Wire link group sync after CommandBar `execute`

**Files:**
- Modify: `frontend/src/components/terminal/CommandBar.tsx`

When the user presses Enter in the CommandBar and a panel is created (or the active panel is updated), if the source panel had a `linkGroup`, the new ticker must propagate to all other panels in that group.

Currently `execute` only calls `addPanel`. We need to also:
1. Look up the active panel's `linkGroup`.
2. If set, call `setLinkGroupTicker` with the new ticker.

**Step 1: Subscribe to the active panel's config and `setLinkGroupTicker`**

In `CommandBar.tsx`, add these store subscriptions inside the `CommandBar` function, after the existing ones:

```typescript
  const workspaces        = useWorkspaceStore((s) => s.workspaces)
  const activeIndex       = useWorkspaceStore((s) => s.activeIndex)
  const activePanelId     = useWorkspaceStore((s) => s.activePanelId)
  const setLinkGroupTicker = useWorkspaceStore((s) => s.setLinkGroupTicker)
```

**Step 2: Update `execute` to propagate the ticker via link group**

Replace the existing `execute` callback:

```typescript
  const execute = useCallback((command: string) => {
    const parts = command.trim().toUpperCase().split(/\s+/)
    const ticker = parts[0] ?? ''
    const fnCode = parts[1] as FunctionCode | undefined

    if (!ticker || !fnCode || !FUNCTION_META[fnCode]) return

    addPanel({ functionCode: fnCode, ticker, market: '', timeframe: '1h' })
    setInput('')
    setSuggestions([])
  }, [addPanel])
```

with:

```typescript
  const execute = useCallback((command: string) => {
    const parts = command.trim().toUpperCase().split(/\s+/)
    const ticker = parts[0] ?? ''
    const fnCode = parts[1] as FunctionCode | undefined

    if (!ticker || !fnCode || !FUNCTION_META[fnCode]) return

    // Propagate ticker to all panels in the same link group (if active panel is linked)
    const activePanel = activePanelId
      ? workspaces[activeIndex]?.panels[activePanelId]
      : undefined
    if (activePanel?.linkGroup) {
      setLinkGroupTicker(activePanel.linkGroup, ticker)
    }

    addPanel({ functionCode: fnCode, ticker, market: '', timeframe: '1h' })
    setInput('')
    setSuggestions([])
  }, [addPanel, activePanelId, activeIndex, workspaces, setLinkGroupTicker])
```

**Verification:**
```bash
docker compose exec frontend npx tsc --noEmit 2>&1 | grep CommandBar
```
Expected: no output.

**Commit:**
```bash
git add frontend/src/components/terminal/CommandBar.tsx
git commit -m "feat(terminal): propagate link group ticker sync when CommandBar executes command"
```

---

## Task I4: Global keyboard shortcuts in WorkspaceLayout

**Files:**
- Modify: `frontend/src/components/terminal/WorkspaceLayout.tsx`
- Modify: `frontend/src/components/terminal/CommandBar.tsx`

Register three global shortcuts:
- `Ctrl+G` / `Cmd+G` — focus the CommandBar input
- `Escape` — blur CommandBar, clear suggestions (when input is empty; otherwise just clears input first)
- `Ctrl+W` / `Cmd+W` — close the currently active panel

**Step 1: Add a forwarded ref to `CommandBar`**

`WorkspaceLayout` needs to imperatively focus the CommandBar's `<input>`. The cleanest pattern is to export a ref-forwarding function or accept an optional `inputRef` prop.

Use a prop approach (simpler than `forwardRef` here since there is only one consumer).

In `CommandBar.tsx`, update the component to accept an optional `inputRef` prop:

```typescript
interface CommandBarProps {
  inputRef?: React.RefObject<HTMLInputElement | null>
}
```

Update the function signature:

```typescript
export function CommandBar({ inputRef: externalRef }: CommandBarProps = {}) {
```

Merge the external ref with the internal `inputRef`. Replace the existing internal ref declaration:

```typescript
  const inputRef = useRef<HTMLInputElement>(null)
```

with:

```typescript
  const internalRef = useRef<HTMLInputElement>(null)
  const inputRef = externalRef ?? internalRef
```

No other changes to `CommandBar.tsx` — the `<input ref={inputRef} ...>` already uses it correctly.

**Step 2: Add a `clearInput` imperative handle**

`WorkspaceLayout` also needs to clear the CommandBar's input and suggestions when `Escape` is pressed globally. Expose this via a second prop: `onClearRequest?: () => void` which `CommandBar` will call from its own `Escape` handler, but also add a separate exported callback approach.

Simplest approach: add `onRequestClear?: () => void` prop to `CommandBarProps`. When `WorkspaceLayout` wants to clear, it just calls a callback set in state, but since we only need focus + clear, the ref approach for focus is sufficient and `Escape` can simply blur the input (which triggers nothing) — and `CommandBar`'s existing `onKeyDown` already clears suggestions when `Escape` is pressed.

So for the `Escape` global shortcut: just call `inputRef.current?.blur()`. The user's next keypress naturally dismisses. This is the minimal correct behavior.

**Step 3: Rewrite `WorkspaceLayout.tsx`**

Replace the entire file with:

```typescript
import { useRef, useEffect } from 'react'
import { useNotificationWS } from '@/hooks/useNotifications'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { SignalToast } from '@/components/SignalToast'
import { CommandBar } from './CommandBar'
import { WorkspaceTabs } from './WorkspaceTabs'
import { PanelGrid } from './PanelGrid'
import { useWorkspaceAutoSave } from '@/hooks/useWorkspaceAutoSave'

export function WorkspaceLayout() {
  useNotificationWS()
  useWorkspaceAutoSave()

  const commandBarInputRef = useRef<HTMLInputElement>(null)
  const activePanelId = useWorkspaceStore((s) => s.activePanelId)
  const removePanel   = useWorkspaceStore((s) => s.removePanel)

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const isMac = navigator.platform.toUpperCase().includes('MAC')
      const ctrlOrCmd = isMac ? e.metaKey : e.ctrlKey

      // Ctrl/Cmd+G — focus command bar
      if (ctrlOrCmd && e.key === 'g') {
        e.preventDefault()
        commandBarInputRef.current?.focus()
        return
      }

      // Escape — blur command bar (CommandBar's own keydown handler clears suggestions)
      if (e.key === 'Escape') {
        commandBarInputRef.current?.blur()
        return
      }

      // Ctrl/Cmd+W — close active panel
      if (ctrlOrCmd && e.key === 'w') {
        e.preventDefault()
        if (activePanelId) removePanel(activePanelId)
        return
      }
    }

    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [activePanelId, removePanel])

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <CommandBar inputRef={commandBarInputRef} />
      <WorkspaceTabs />
      <main className="flex-1 overflow-hidden relative">
        <PanelGrid />
      </main>
      <SignalToast />
    </div>
  )
}
```

**Verification:**
```bash
docker compose exec frontend npx tsc --noEmit 2>&1 | grep -E "WorkspaceLayout|CommandBar"
```
Expected: no output.

Open `http://localhost:5173/terminal`. Press `Ctrl+G` — the command bar input should receive focus (cursor appears). Type something and press `Escape` — suggestions should disappear and input clear. Add a panel, click it to make it active, press `Ctrl+W` — panel should close.

**Commit:**
```bash
git add frontend/src/components/terminal/WorkspaceLayout.tsx \
        frontend/src/components/terminal/CommandBar.tsx
git commit -m "feat(terminal): global keyboard shortcuts — Ctrl+G focus, Esc blur, Ctrl+W close panel"
```

---

## Task I5: AI context — pass active ticker to ChatPanel

**Files:**
- Modify: `frontend/src/components/ai/ChatPanel.tsx`
- Modify: `frontend/src/components/widgets/AIChatWidget.tsx`

The `AIChatWidget` receives a `ticker` prop (via `WidgetProps`) but currently passes nothing to `ChatPanel`. Phase I wires the ticker through and displays it as a context badge in the chat header. No system prompt injection (we do not know the AI backend architecture); badge-only is the preferred approach per the spec.

**Step 1: Add `contextTicker` prop to `ChatPanel`**

In `ChatPanel.tsx`, update the `ChatPanelProps` interface:

```typescript
interface ChatPanelProps {
  isOpen: boolean
  onClose: () => void
  contextTicker?: string
}
```

Update the function signature to destructure the new prop:

```typescript
export function ChatPanel({ isOpen, onClose, contextTicker }: ChatPanelProps) {
```

**Step 2: Display the context ticker badge in the header**

In `ChatPanel.tsx`, locate the header block:

```tsx
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-200 dark:border-zinc-700">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm text-zinc-900 dark:text-zinc-100">
              AI Assistant
            </span>
            <span className="text-xs px-2 py-0.5 rounded-full bg-violet-100 dark:bg-violet-900/40 text-violet-700 dark:text-violet-300">
              {pageContext.page}
            </span>
          </div>
```

Add the ticker badge after the page context badge:

```tsx
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-200 dark:border-zinc-700">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm text-zinc-900 dark:text-zinc-100">
              AI Assistant
            </span>
            <span className="text-xs px-2 py-0.5 rounded-full bg-violet-100 dark:bg-violet-900/40 text-violet-700 dark:text-violet-300">
              {pageContext.page}
            </span>
            {contextTicker && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-300 font-mono">
                {contextTicker}
              </span>
            )}
          </div>
```

**Step 3: Include `contextTicker` in the `pageContext` sent to the AI backend**

The `pageContext` object is of type `AIPageContext`. We want to pass the ticker through to the AI endpoint. Look up the existing type in `types/index.ts`:

Check `AIPageContext` — it already has `symbol?: string`. Map `contextTicker` to `symbol` when building the context in the `useEffect`:

```typescript
  // Capture context when panel opens
  useEffect(() => {
    if (!isOpen) return
    const page = getPageFromPath(window.location.pathname)
    const ctx: AIPageContext = {
      page,
      symbol: contextTicker ?? market.selectedSymbol ?? undefined,
      timeframe: market.selectedTimeframe,
    }
    if (page === 'backtest' && backtest.activeBacktest) {
      ctx.metrics = (backtest.activeBacktest.metrics as unknown as Record<string, unknown>) ?? {}
    }
    if (page === 'portfolio' && portfolio.positions.length > 0) {
      ctx.positions = portfolio.positions.map((p) => ({
        symbol: p.symbol,
        pnl_pct: p.unrealized_pnl_pct,
      }))
    }
    setPageContext(ctx)
  }, [isOpen, contextTicker]) // re-capture when contextTicker changes
```

Note: `contextTicker` is added to the dependency array so the context refreshes if the panel's ticker changes (e.g. via link group sync).

**Step 4: Update `AIChatWidget` to pass ticker down**

Replace `AIChatWidget.tsx`:

```typescript
import { ChatPanel } from '@/components/ai/ChatPanel'
import type { WidgetProps } from '@/types/terminal'

export function AIChatWidget({ ticker }: WidgetProps) {
  return (
    <div className="h-full flex flex-col overflow-hidden">
      <ChatPanel isOpen={true} onClose={() => {}} contextTicker={ticker || undefined} />
    </div>
  )
}
```

**Verification:**
```bash
docker compose exec frontend npx tsc --noEmit 2>&1 | grep -E "ChatPanel|AIChatWidget"
```
Expected: no output.

Open `http://localhost:5173/terminal`. Type `AAPL AI` in the command bar and press Enter. An AI panel should open. The header should show both the `terminal` page badge and an `AAPL` context badge in green. Change the panel ticker inline (Task I2) — the badge should update.

**Commit:**
```bash
git add frontend/src/components/ai/ChatPanel.tsx \
        frontend/src/components/widgets/AIChatWidget.tsx
git commit -m "feat(terminal): pass active ticker as context to AI chat panel"
```

---

## Task I6: Workspace auto-save hook

**Files:**
- Create: `frontend/src/hooks/useWorkspaceAutoSave.ts`
- Modify: `frontend/src/components/terminal/WorkspaceLayout.tsx` (already done in Task I4)

Auto-save the active workspace to the backend 2 seconds after any layout or panel change, but only if the workspace has already been persisted (i.e. has an `id` from the server).

**Step 1: Create `useWorkspaceAutoSave.ts`**

```typescript
import { useEffect, useRef } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { updateWorkspace } from '@/api/terminal'

/**
 * Debounced auto-save: persists the active workspace to the backend 2 seconds
 * after any change. Only runs when the workspace has a server-assigned `id`.
 */
export function useWorkspaceAutoSave() {
  const workspaces  = useWorkspaceStore((s) => s.workspaces)
  const activeIndex = useWorkspaceStore((s) => s.activeIndex)

  const activeWorkspace = workspaces[activeIndex]

  const mutation = useMutation({
    mutationFn: ({ id, ws }: { id: number; ws: typeof activeWorkspace }) =>
      updateWorkspace(id, ws),
    // Silently ignore errors — the user can still work; we'll retry on next change
  })

  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    // Only auto-save workspaces that have been persisted to the server
    if (!activeWorkspace?.id) return

    const id = activeWorkspace.id

    // Clear any existing debounce timer
    if (timerRef.current) clearTimeout(timerRef.current)

    timerRef.current = setTimeout(() => {
      mutation.mutate({ id, ws: activeWorkspace })
    }, 2000)

    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeWorkspace])
  // Note: `mutation` is intentionally omitted — mutate is stable across renders.
}
```

**Step 2: Confirm `WorkspaceLayout.tsx` already calls the hook**

The `WorkspaceLayout.tsx` rewritten in Task I4 already contains:

```typescript
  useWorkspaceAutoSave()
```

No further changes needed.

**Verification:**
```bash
docker compose exec frontend npx tsc --noEmit 2>&1 | grep useWorkspaceAutoSave
```
Expected: no output.

To test: Load the terminal. If a workspace was previously saved to the server (has an `id`), move a panel. Check the network tab in DevTools — 2 seconds after the drag ends, a `PUT /api/v1/workspaces/:id` request should fire.

For workspaces without a server `id` (all template workspaces during local development), no request fires — this is correct behavior.

**Commit:**
```bash
git add frontend/src/hooks/useWorkspaceAutoSave.ts
git commit -m "feat(terminal): debounced workspace auto-save hook (2s after any layout change)"
```

---

## Task I7: End-to-end smoke test

**Files:** No file changes — verification only.

**Step 1: Start services**
```bash
make up
make health
```

**Step 2: Open terminal**
Navigate to `http://localhost:5173/terminal`.

**Step 3: Test panel linking**
1. Add two GP panels: type `BTCUSDT GP`, press Enter. Type `ETHUSDT NEWS`, press Enter.
2. Click the link group dot on the GP panel until it turns red.
3. Click the link group dot on the NEWS panel until it also turns red.
4. Click the ticker badge on the GP panel, type `SOLUSDT`, press Enter.
5. Confirm the NEWS panel ticker badge also changed to `SOLUSDT`.

**Step 4: Test inline ticker edit via keyboard**
1. Click the ticker badge on any panel.
2. Type `AAPL`, press Enter.
3. Confirm ticker updates and input returns to badge display.
4. Click the badge again, press Escape — confirm edit cancels (old ticker remains).

**Step 5: Test keyboard shortcuts**
1. Click somewhere outside the command bar.
2. Press `Ctrl+G` (or `Cmd+G` on Mac). Confirm the command bar input receives focus.
3. Type `BTC GP` and press `Escape`. Confirm suggestions dismiss and input clears.
4. Add a panel, click on it (active panel ring turns blue). Press `Ctrl+W`. Confirm the panel closes.

**Step 6: Test AI context**
1. Type `MSFT AI`, press Enter.
2. Confirm the AI chat header shows both a `terminal` badge and an `MSFT` badge.
3. Use the inline ticker editor to change the panel's ticker to `GOOG`.
4. Confirm the AI header badge updates to `GOOG`.

**Step 7: Verify no TypeScript errors**
```bash
docker compose exec frontend npx tsc --noEmit
```
Expected: exits with code 0, no output.

**Step 8: Verify no ESLint errors**
```bash
make frontend-lint
```
Expected: no warnings or errors related to Phase I files.

---

## Phase I Completion Checklist

- [ ] **I1** — `setLinkGroupTicker(linkGroup, ticker)` action added to `workspaceStore.ts`; all linked panels in the active workspace update their `ticker` atomically
- [ ] **I2** — Link group dot in `PanelSlot` is always rendered (faint border when unlinked), cycles `null → red → blue → green → yellow → null` on click, calls `onUpdate({ linkGroup })`; inline ticker badge becomes an editable input on click, commits on Enter or blur, cancels on Escape; `PanelGrid` calls `setLinkGroupTicker` when `onUpdate` includes a `ticker` change on a linked panel
- [ ] **I3** — `CommandBar.execute` propagates new ticker to link group when the active panel is linked
- [ ] **I4** — `WorkspaceLayout` registers `window.keydown` handler: `Ctrl/Cmd+G` focuses command bar input (via forwarded ref prop), `Escape` blurs it, `Ctrl/Cmd+W` removes the active panel; handler cleaned up on unmount
- [ ] **I5** — `ChatPanel` accepts `contextTicker?: string` prop; shows green monospace badge in header; maps `contextTicker` to `AIPageContext.symbol` when building request payload; `AIChatWidget` passes its `ticker` prop through
- [ ] **I6** — `useWorkspaceAutoSave` hook debounces `updateWorkspace` calls by 2 seconds; only fires when `activeWorkspace.id` is defined; `WorkspaceLayout` calls the hook
- [ ] **I7** — Smoke test passes: panel linking syncs across two panels, inline ticker edit works, three keyboard shortcuts work, AI badge reflects panel ticker, `tsc --noEmit` exits clean

---

## Files Changed Summary

| File | Change |
|------|--------|
| `frontend/src/stores/workspaceStore.ts` | Add `setLinkGroupTicker` action |
| `frontend/src/components/terminal/PanelSlot.tsx` | Interactive link dot, `TickerBadge` sub-component, fix `onUpdate` alias |
| `frontend/src/components/terminal/PanelGrid.tsx` | Call `setLinkGroupTicker` on ticker update for linked panels |
| `frontend/src/components/terminal/CommandBar.tsx` | Accept `inputRef` prop, propagate link group in `execute` |
| `frontend/src/components/terminal/WorkspaceLayout.tsx` | Global keyboard shortcut `useEffect`, call `useWorkspaceAutoSave` |
| `frontend/src/components/ai/ChatPanel.tsx` | Add `contextTicker` prop, badge in header, wire to `AIPageContext.symbol` |
| `frontend/src/components/widgets/AIChatWidget.tsx` | Pass `ticker` as `contextTicker` to `ChatPanel` |
| `frontend/src/hooks/useWorkspaceAutoSave.ts` | New hook — debounced auto-save |
