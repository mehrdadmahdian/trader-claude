import apiClient from '@/api/client'
import type { WorkspaceConfig } from '@/types/terminal'

// Shape returned by backend (layout + panel_states are JSON objects)
interface ServerWorkspace {
  id: number
  name: string
  is_template: boolean
  layout: WorkspaceConfig['layout']
  panel_states: WorkspaceConfig['panels']
  created_at: string
  updated_at: string
}

function toLocal(sw: ServerWorkspace): WorkspaceConfig {
  return {
    id: sw.id,
    name: sw.name,
    layout: sw.layout ?? [],
    panels: sw.panel_states ?? {},
  }
}

export async function fetchWorkspaces(): Promise<WorkspaceConfig[]> {
  const { data } = await apiClient.get<ServerWorkspace[]>('/workspaces')
  return data.map(toLocal)
}

export async function createWorkspace(ws: WorkspaceConfig): Promise<WorkspaceConfig> {
  const { data } = await apiClient.post<ServerWorkspace>('/workspaces', {
    name: ws.name,
    layout: ws.layout,
    panel_states: ws.panels,
  })
  return toLocal(data)
}

export async function updateWorkspace(id: number, ws: WorkspaceConfig): Promise<WorkspaceConfig> {
  const { data } = await apiClient.put<ServerWorkspace>(`/workspaces/${id}`, {
    name: ws.name,
    layout: ws.layout,
    panel_states: ws.panels,
  })
  return toLocal(data)
}

export async function deleteWorkspace(id: number): Promise<void> {
  await apiClient.delete(`/workspaces/${id}`)
}
