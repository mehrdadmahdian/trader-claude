import { useCallback, useEffect, useState } from 'react'
import { GridLayout } from 'react-grid-layout'
import type { Layout } from 'react-grid-layout'
import 'react-grid-layout/css/styles.css'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { PanelSlot } from './PanelSlot'
import type { GridItem } from '@/types/terminal'

const GRID_CONFIG = { cols: 12, rowHeight: 30, margin: [4, 4] as [number, number] }
const DRAG_CONFIG = { handle: '.drag-handle' }
const RESIZE_CONFIG = { handles: ['se'] as ['se'] }

export function PanelGrid() {
  const workspaces    = useWorkspaceStore((s) => s.workspaces)
  const activeIndex   = useWorkspaceStore((s) => s.activeIndex)
  const activePanelId = useWorkspaceStore((s) => s.activePanelId)
  const updateLayout  = useWorkspaceStore((s) => s.updateLayout)
  const removePanel   = useWorkspaceStore((s) => s.removePanel)
  const updatePanel   = useWorkspaceStore((s) => s.updatePanel)
  const setActive     = useWorkspaceStore((s) => s.setActivePanel)

  const [width, setWidth] = useState(window.innerWidth)

  useEffect(() => {
    const handler = () => setWidth(window.innerWidth)
    window.addEventListener('resize', handler)
    return () => window.removeEventListener('resize', handler)
  }, [])

  const ws = workspaces[activeIndex]
  if (!ws) return null

  const onLayoutChange = useCallback(
    (layout: Layout) => updateLayout(layout as GridItem[]),
    [updateLayout],
  )

  return (
    <GridLayout
      className="w-full"
      layout={ws.layout as Layout}
      width={width}
      gridConfig={GRID_CONFIG}
      dragConfig={DRAG_CONFIG}
      resizeConfig={RESIZE_CONFIG}
      onLayoutChange={onLayoutChange}
    >
      {ws.layout.map((item) => {
        const config = ws.panels[item.i]
        if (!config) return null
        return (
          <div key={item.i}>
            <PanelSlot
              config={config}
              isActive={activePanelId === item.i}
              onFocus={() => setActive(item.i)}
              onClose={() => removePanel(item.i)}
              onUpdate={(update) => updatePanel(item.i, update)}
            />
          </div>
        )
      })}
    </GridLayout>
  )
}
