import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { WorkspaceConfig, PanelConfig, GridItem, FunctionCode } from '@/types/terminal'

// ── Default workspace templates ────────────────────────────────────────────

const makePanel = (id: string, fn: FunctionCode, ticker: string, market = '', timeframe = '1h'): PanelConfig => ({
  id, functionCode: fn, ticker, market, timeframe, linkGroup: null,
})

export const WORKSPACE_TEMPLATES: WorkspaceConfig[] = [
  {
    name: 'Market Overview',
    layout: [
      { i: 'p1', x: 0, y: 0, w: 7, h: 14, minW: 3, minH: 4 },
      { i: 'p2', x: 7, y: 0, w: 5, h: 7,  minW: 3, minH: 4 },
      { i: 'p3', x: 7, y: 7, w: 5, h: 7,  minW: 3, minH: 4 },
    ],
    panels: {
      p1: makePanel('p1', 'HM',   '',        ''),
      p2: makePanel('p2', 'NEWS', '',        ''),
      p3: makePanel('p3', 'WL',  '',        ''),
    },
  },
  {
    name: 'Trader',
    layout: [
      { i: 'p1', x: 0, y: 0, w: 8, h: 10, minW: 3, minH: 4 },
      { i: 'p2', x: 8, y: 0, w: 4, h: 10, minW: 3, minH: 4 },
      { i: 'p3', x: 0, y: 10, w: 6, h: 8, minW: 3, minH: 4 },
      { i: 'p4', x: 6, y: 10, w: 6, h: 8, minW: 3, minH: 4 },
    ],
    panels: {
      p1: makePanel('p1', 'GP',   'BTCUSDT', 'binance', '1h'),
      p2: makePanel('p2', 'NEWS', '',        ''),
      p3: makePanel('p3', 'PORT', '',        ''),
      p4: makePanel('p4', 'ALRT', '',        ''),
    },
  },
  {
    name: 'Crypto',
    layout: [
      { i: 'p1', x: 0, y: 0, w: 6, h: 10, minW: 3, minH: 4 },
      { i: 'p2', x: 6, y: 0, w: 6, h: 10, minW: 3, minH: 4 },
      { i: 'p3', x: 0, y: 10, w: 6, h: 8, minW: 3, minH: 4 },
      { i: 'p4', x: 6, y: 10, w: 6, h: 8, minW: 3, minH: 4 },
    ],
    panels: {
      p1: makePanel('p1', 'GP', 'BTCUSDT', 'binance', '1h'),
      p2: makePanel('p2', 'GP', 'ETHUSDT', 'binance', '1h'),
      p3: makePanel('p3', 'NEWS', '', ''),
      p4: makePanel('p4', 'WL',  '', ''),
    },
  },
  {
    name: 'Analyst',
    layout: [
      { i: 'p1', x: 0, y: 0, w: 6, h: 12, minW: 3, minH: 4 },
      { i: 'p2', x: 6, y: 0, w: 6, h: 12, minW: 3, minH: 4 },
      { i: 'p3', x: 0, y: 12, w: 12, h: 8, minW: 3, minH: 4 },
    ],
    panels: {
      p1: makePanel('p1', 'FA',  'AAPL', 'yahoo'),
      p2: makePanel('p2', 'CAL', '',     ''),
      p3: makePanel('p3', 'SCR', '',     ''),
    },
  },
]

// ── Store ──────────────────────────────────────────────────────────────────

interface WorkspaceStore {
  workspaces: WorkspaceConfig[]
  activeIndex: number
  activePanelId: string | null

  // Panel focus
  setActivePanel: (id: string | null) => void

  // Workspace tabs
  setActiveWorkspace: (index: number) => void
  addWorkspace: (name?: string) => void
  removeWorkspace: (index: number) => void
  renameWorkspace: (index: number, name: string) => void

  // Layout persistence
  updateLayout: (layout: GridItem[]) => void

  // Panel CRUD
  addPanel: (config: Omit<PanelConfig, 'id'>) => string  // returns new panel id
  updatePanel: (panelId: string, update: Partial<PanelConfig>) => void
  removePanel: (panelId: string) => void

  // Sync from server
  loadServerWorkspaces: (workspaces: WorkspaceConfig[]) => void
}

export const useWorkspaceStore = create<WorkspaceStore>()(
  persist(
    (set, get) => ({
      workspaces: WORKSPACE_TEMPLATES,
      activeIndex: 0,
      activePanelId: null,

      setActivePanel: (id) => set({ activePanelId: id }),

      setActiveWorkspace: (index) => set({ activeIndex: index }),

      addWorkspace: (name = 'New Workspace') =>
        set((s) => ({
          workspaces: [...s.workspaces, { name, layout: [], panels: {} }],
          activeIndex: s.workspaces.length,
        })),

      removeWorkspace: (index) =>
        set((s) => {
          if (s.workspaces.length <= 1) return s
          const next = s.workspaces.filter((_, i) => i !== index)
          return { workspaces: next, activeIndex: Math.min(s.activeIndex, next.length - 1) }
        }),

      renameWorkspace: (index, name) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          workspaces[index] = { ...workspaces[index], name }
          return { workspaces }
        }),

      updateLayout: (layout) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          workspaces[s.activeIndex] = { ...workspaces[s.activeIndex], layout }
          return { workspaces }
        }),

      addPanel: (config) => {
        const id = crypto.randomUUID()
        set((s) => {
          const workspaces = [...s.workspaces]
          const ws = workspaces[s.activeIndex]
          const newPanel: PanelConfig = { ...config, id }
          const maxY = ws.layout.reduce((m, item) => Math.max(m, item.y + item.h), 0)
          workspaces[s.activeIndex] = {
            ...ws,
            layout: [...ws.layout, { i: id, x: 0, y: maxY, w: 6, h: 10, minW: 3, minH: 4 }],
            panels: { ...ws.panels, [id]: newPanel },
          }
          return { workspaces }
        })
        return id
      },

      updatePanel: (panelId, update) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          const ws = workspaces[s.activeIndex]
          workspaces[s.activeIndex] = {
            ...ws,
            panels: { ...ws.panels, [panelId]: { ...ws.panels[panelId], ...update } },
          }
          return { workspaces }
        }),

      removePanel: (panelId) =>
        set((s) => {
          const workspaces = [...s.workspaces]
          const ws = workspaces[s.activeIndex]
          const { [panelId]: _, ...panels } = ws.panels
          workspaces[s.activeIndex] = {
            ...ws,
            layout: ws.layout.filter((item) => item.i !== panelId),
            panels,
          }
          return { workspaces }
        }),

      loadServerWorkspaces: (serverWorkspaces) =>
        set({ workspaces: serverWorkspaces.length > 0 ? serverWorkspaces : WORKSPACE_TEMPLATES }),
    }),
    { name: 'trader-workspaces' },
  ),
)
