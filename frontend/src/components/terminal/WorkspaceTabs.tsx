import { useState } from 'react'
import { Plus, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useWorkspaceStore } from '@/stores/workspaceStore'

export function WorkspaceTabs() {
  const { workspaces, activeIndex, setActiveWorkspace, addWorkspace, removeWorkspace, renameWorkspace } = useWorkspaceStore()
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [editValue, setEditValue] = useState('')

  return (
    <div className="flex items-center gap-1 px-2 overflow-x-auto shrink-0 border-b border-border bg-muted/30">
      {workspaces.map((ws, i) => (
        <div
          key={i}
          className={cn(
            'flex items-center gap-1 px-3 py-1.5 text-xs rounded-t cursor-pointer select-none shrink-0',
            i === activeIndex
              ? 'bg-background border border-b-background border-border text-foreground font-medium'
              : 'text-muted-foreground hover:text-foreground',
          )}
          onClick={() => setActiveWorkspace(i)}
          onDoubleClick={() => { setEditingIndex(i); setEditValue(ws.name) }}
        >
          {editingIndex === i ? (
            <input
              autoFocus
              className="bg-transparent outline-none w-24 text-xs"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              onBlur={() => { renameWorkspace(i, editValue || ws.name); setEditingIndex(null) }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') { renameWorkspace(i, editValue || ws.name); setEditingIndex(null) }
                if (e.key === 'Escape') setEditingIndex(null)
              }}
              onClick={(e) => e.stopPropagation()}
            />
          ) : (
            <span>{ws.name}</span>
          )}
          {workspaces.length > 1 && (
            <button
              className="text-muted-foreground hover:text-red-500 ml-1"
              onClick={(e) => { e.stopPropagation(); removeWorkspace(i) }}
            >
              <X size={10} />
            </button>
          )}
        </div>
      ))}
      <button
        className="p-1.5 text-muted-foreground hover:text-foreground shrink-0"
        onClick={() => addWorkspace()}
        title="New workspace"
      >
        <Plus size={14} />
      </button>
    </div>
  )
}
