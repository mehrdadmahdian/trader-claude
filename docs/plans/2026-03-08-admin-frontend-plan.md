# Admin Dashboard — Frontend: React UI

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a production-quality admin dashboard at `/admin` — separate from both the trading workbench and Bloomberg terminal. Covers user management, role management, permissions browser, audit log, and system overview.

**Architecture:** `/admin` uses its own `AdminLayout` (no sidebar, dedicated admin nav). Protected by `AdminRoute` guard (admin role required). Each admin section is a page under `/admin/*`. Data tables, modals, and permission gates are shared components.

**Tech Stack:** React 18, TypeScript, Zustand, React Query v5, TailwindCSS, shadcn/ui (existing), lucide-react (existing)

**Design principles:**
- Standard data table pattern: search bar + filter chips + sortable columns + row actions
- Modals for create/edit (never inline)
- Confirmation dialogs for destructive actions
- Toast notifications for all mutations (success + error)
- Responsive: works on tablet and desktop (admin is desktop-first)

---

## Existing state (do not break)

- `useAuthStore` exposes current user with `role` field
- Auth protected routes use `<ProtectedRoute>` wrapper
- All existing routes (`/`, `/terminal`, etc.) untouched
- `api/client.ts` Axios instance used for all API calls

---

## Task F1: Install shadcn table component

**Step 1: Add table to shadcn**
```bash
docker compose exec frontend npx shadcn-ui@latest add table
docker compose exec frontend npx shadcn-ui@latest add badge
docker compose exec frontend npx shadcn-ui@latest add dialog
docker compose exec frontend npx shadcn-ui@latest add alert-dialog
docker compose exec frontend npx shadcn-ui@latest add tabs
docker compose exec frontend npx shadcn-ui@latest add toast
```

**Step 2: Verify components exist**
```bash
ls frontend/src/components/ui/
```
Expected: `table.tsx`, `badge.tsx`, `dialog.tsx`, `alert-dialog.tsx`, `tabs.tsx`, `toast.tsx` present.

**Step 3: Commit**
```bash
git add frontend/src/components/ui/
git commit -m "chore: add shadcn table, badge, dialog, alert-dialog, tabs, toast components"
```

---

## Task F2: Create admin TypeScript types

**Files:**
- Create: `frontend/src/types/admin.ts`

```typescript
// All admin dashboard types — separate from trading and terminal types

export interface AdminUser {
  id: number
  email: string
  display_name: string
  role: 'admin' | 'user' | string
  active: boolean
  last_login_at: string | null
  created_at: string
  updated_at: string
}

export interface Permission {
  id: number
  name: string        // "users:write"
  resource: string    // "users"
  action: string      // "write"
  description: string
  created_at: string
}

export interface Role {
  id: number
  name: string
  description: string
  is_system: boolean
  permissions: Permission[]
  created_at: string
  updated_at: string
}

export interface AuditLog {
  id: number
  user_id: number
  user_email: string
  action: string      // "user.created"
  resource: string    // "user"
  resource_id: string
  details: Record<string, unknown> | null
  ip_address: string
  created_at: string
}

export interface SystemStats {
  version: string
  uptime_sec: number
  go_version: string
  goroutines: number
  memory_mb: number
  users: { total: number; active: number }
  roles: number
  backtests: number
  alerts: number
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  limit: number
}

// Form payloads
export interface CreateUserPayload {
  email: string
  display_name: string
  password: string
  role: string
}

export interface UpdateUserPayload {
  display_name?: string
  role?: string
  active?: boolean
}

export interface CreateRolePayload {
  name: string
  description: string
  permission_ids: number[]
}

export interface UpdateRolePayload {
  name?: string
  description?: string
  permission_ids?: number[]
}
```

**Step 2: Commit**
```bash
git add frontend/src/types/admin.ts
git commit -m "feat(admin): add admin TypeScript types"
```

---

## Task F3: Create admin API client

**Files:**
- Create: `frontend/src/api/admin.ts`

```typescript
import apiClient from '@/api/client'
import type {
  AdminUser, Role, Permission, AuditLog, SystemStats,
  PaginatedResponse, CreateUserPayload, UpdateUserPayload,
  CreateRolePayload, UpdateRolePayload,
} from '@/types/admin'

// ── Users ──────────────────────────────────────────────────────────────────

export const fetchAdminUsers = async (): Promise<AdminUser[]> => {
  const { data } = await apiClient.get<AdminUser[]>('/admin/users')
  return data
}

export const fetchAdminUser = async (id: number): Promise<AdminUser> => {
  const { data } = await apiClient.get<AdminUser>(`/admin/users/${id}`)
  return data
}

export const createAdminUser = async (payload: CreateUserPayload): Promise<AdminUser> => {
  const { data } = await apiClient.post<AdminUser>('/admin/users', payload)
  return data
}

export const updateAdminUser = async (id: number, payload: UpdateUserPayload): Promise<AdminUser> => {
  const { data } = await apiClient.put<AdminUser>(`/admin/users/${id}`, payload)
  return data
}

export const deleteAdminUser = async (id: number): Promise<void> => {
  await apiClient.delete(`/admin/users/${id}`)
}

export const toggleAdminUser = async (id: number): Promise<void> => {
  await apiClient.patch(`/admin/users/${id}/toggle`)
}

// ── Roles ──────────────────────────────────────────────────────────────────

export const fetchRoles = async (): Promise<Role[]> => {
  const { data } = await apiClient.get<Role[]>('/admin/roles')
  return data
}

export const createRole = async (payload: CreateRolePayload): Promise<Role> => {
  const { data } = await apiClient.post<Role>('/admin/roles', payload)
  return data
}

export const updateRole = async (id: number, payload: UpdateRolePayload): Promise<Role> => {
  const { data } = await apiClient.put<Role>(`/admin/roles/${id}`, payload)
  return data
}

export const deleteRole = async (id: number): Promise<void> => {
  await apiClient.delete(`/admin/roles/${id}`)
}

// ── Permissions ────────────────────────────────────────────────────────────

export const fetchPermissions = async (): Promise<Permission[]> => {
  const { data } = await apiClient.get<Permission[]>('/admin/permissions')
  return data
}

// ── Audit Log ──────────────────────────────────────────────────────────────

export const fetchAuditLogs = async (params: {
  page?: number; limit?: number; action?: string; resource?: string; user_id?: number
}): Promise<PaginatedResponse<AuditLog>> => {
  const { data } = await apiClient.get<PaginatedResponse<AuditLog>>('/admin/audit-logs', { params })
  return data
}

// ── System ─────────────────────────────────────────────────────────────────

export const fetchSystemStats = async (): Promise<SystemStats> => {
  const { data } = await apiClient.get<SystemStats>('/admin/system/stats')
  return data
}
```

**Step 2: Commit**
```bash
git add frontend/src/api/admin.ts
git commit -m "feat(admin): add admin API client — users, roles, permissions, audit, system"
```

---

## Task F4: Create AdminRoute guard and AdminLayout

**Files:**
- Create: `frontend/src/components/admin/AdminRoute.tsx`
- Create: `frontend/src/components/admin/AdminLayout.tsx`

**Step 1: AdminRoute.tsx** — redirects non-admins back to `/`

```tsx
import { Navigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/authStore'

export function AdminRoute({ children }: { children: React.ReactNode }) {
  const user = useAuthStore((s) => s.user)

  if (!user) return <Navigate to="/login" replace />
  if (user.role !== 'admin') return <Navigate to="/" replace />

  return <>{children}</>
}
```

**Step 2: AdminLayout.tsx** — standalone admin shell (no sidebar from old app)

```tsx
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import {
  Users, Shield, Key, ScrollText, BarChart3,
  ArrowLeft, Settings,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/authStore'

const navItems = [
  { to: '/admin',              icon: BarChart3,   label: 'Overview',    end: true },
  { to: '/admin/users',        icon: Users,       label: 'Users'        },
  { to: '/admin/roles',        icon: Shield,      label: 'Roles'        },
  { to: '/admin/permissions',  icon: Key,         label: 'Permissions'  },
  { to: '/admin/audit',        icon: ScrollText,  label: 'Audit Log'    },
  { to: '/admin/system',       icon: Settings,    label: 'System'       },
]

export function AdminLayout() {
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)

  return (
    <div className="flex h-screen bg-background overflow-hidden">
      {/* Sidebar */}
      <aside className="w-56 shrink-0 flex flex-col border-r border-border bg-muted/20">
        {/* Header */}
        <div className="px-4 py-4 border-b border-border">
          <div className="flex items-center gap-2">
            <Shield size={18} className="text-primary" />
            <span className="font-semibold text-sm">Admin Panel</span>
          </div>
          <p className="text-xs text-muted-foreground mt-1 truncate">{user?.email}</p>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-2 py-3 space-y-0.5">
          {navItems.map(({ to, icon: Icon, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-2.5 px-3 py-2 rounded text-sm transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground font-medium'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent',
                )
              }
            >
              <Icon size={15} />
              {label}
            </NavLink>
          ))}
        </nav>

        {/* Back to app */}
        <div className="p-3 border-t border-border">
          <button
            onClick={() => navigate('/')}
            className="flex items-center gap-2 text-xs text-muted-foreground hover:text-foreground w-full px-3 py-2 rounded hover:bg-accent"
          >
            <ArrowLeft size={13} />
            Back to App
          </button>
        </div>
      </aside>

      {/* Content */}
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>
    </div>
  )
}
```

**Step 3: Commit**
```bash
git add frontend/src/components/admin/AdminRoute.tsx frontend/src/components/admin/AdminLayout.tsx
git commit -m "feat(admin): add AdminRoute guard and AdminLayout with sidebar nav"
```

---

## Task F5: Create UsersPage

**Files:**
- Create: `frontend/src/pages/admin/UsersPage.tsx`

```tsx
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { UserPlus, Search, MoreHorizontal, Trash2, ToggleLeft, ToggleRight } from 'lucide-react'
import { fetchAdminUsers, toggleAdminUser, deleteAdminUser, createAdminUser } from '@/api/admin'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@radix-ui/react-dropdown-menu'
import { formatDistanceToNow } from 'date-fns'
import type { AdminUser } from '@/types/admin'

export function UsersPage() {
  const qc = useQueryClient()
  const [search, setSearch] = useState('')
  const [showCreate, setShowCreate] = useState(false)

  const { data: users = [], isLoading } = useQuery({
    queryKey: ['admin', 'users'],
    queryFn: fetchAdminUsers,
  })

  const toggleMutation = useMutation({
    mutationFn: (id: number) => toggleAdminUser(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'users'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteAdminUser(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'users'] }),
  })

  const filtered = users.filter(
    (u) =>
      u.email.toLowerCase().includes(search.toLowerCase()) ||
      u.display_name.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="p-6 max-w-6xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold">Users</h1>
          <p className="text-sm text-muted-foreground mt-0.5">{users.length} total accounts</p>
        </div>
        <Button size="sm" onClick={() => setShowCreate(true)}>
          <UserPlus size={14} className="mr-1.5" />
          New User
        </Button>
      </div>

      {/* Search */}
      <div className="relative mb-4 max-w-sm">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
        <Input
          className="pl-8 h-8 text-sm"
          placeholder="Search users..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {/* Table */}
      <div className="rounded-md border border-border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>User</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Last Login</TableHead>
              <TableHead>Joined</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                  Loading...
                </TableCell>
              </TableRow>
            ) : filtered.map((user) => (
              <TableRow key={user.id}>
                <TableCell>
                  <div className="font-medium text-sm">{user.display_name}</div>
                  <div className="text-xs text-muted-foreground">{user.email}</div>
                </TableCell>
                <TableCell>
                  <Badge variant={user.role === 'admin' ? 'default' : 'secondary'} className="text-xs">
                    {user.role}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={user.active ? 'default' : 'destructive'} className="text-xs">
                    {user.active ? 'Active' : 'Disabled'}
                  </Badge>
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {user.last_login_at
                    ? formatDistanceToNow(new Date(user.last_login_at), { addSuffix: true })
                    : 'Never'}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {new Date(user.created_at).toLocaleDateString()}
                </TableCell>
                <TableCell>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-7 w-7">
                        <MoreHorizontal size={14} />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-40 bg-popover border border-border rounded shadow-md p-1 text-sm z-50">
                      <DropdownMenuItem
                        className="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer hover:bg-accent"
                        onClick={() => toggleMutation.mutate(user.id)}
                      >
                        {user.active ? <ToggleLeft size={13} /> : <ToggleRight size={13} />}
                        {user.active ? 'Disable' : 'Enable'}
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        className="flex items-center gap-2 px-2 py-1.5 rounded cursor-pointer hover:bg-destructive hover:text-destructive-foreground text-destructive"
                        onClick={() => {
                          if (confirm(`Delete ${user.email}? This cannot be undone.`)) {
                            deleteMutation.mutate(user.id)
                          }
                        }}
                      >
                        <Trash2 size={13} />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
```

**Step 2: Commit**
```bash
git add frontend/src/pages/admin/UsersPage.tsx
git commit -m "feat(admin): add UsersPage with data table, search, toggle, delete"
```

---

## Task F6: Create RolesPage

**Files:**
- Create: `frontend/src/pages/admin/RolesPage.tsx`

```tsx
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Edit2, Lock } from 'lucide-react'
import { fetchRoles, fetchPermissions, createRole, updateRole, deleteRole } from '@/api/admin'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Role, Permission } from '@/types/admin'

// Group permissions by resource for the checkbox UI
function groupByResource(perms: Permission[]): Record<string, Permission[]> {
  return perms.reduce((acc, p) => {
    if (!acc[p.resource]) acc[p.resource] = []
    acc[p.resource].push(p)
    return acc
  }, {} as Record<string, Permission[]>)
}

export function RolesPage() {
  const qc = useQueryClient()
  const [editingRole, setEditingRole] = useState<Role | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [formName, setFormName] = useState('')
  const [formDesc, setFormDesc] = useState('')
  const [selectedPermIds, setSelectedPermIds] = useState<Set<number>>(new Set())

  const { data: roles = [] } = useQuery({ queryKey: ['admin', 'roles'], queryFn: fetchRoles })
  const { data: permissions = [] } = useQuery({ queryKey: ['admin', 'permissions'], queryFn: fetchPermissions })
  const grouped = groupByResource(permissions)

  const createMutation = useMutation({
    mutationFn: () => createRole({ name: formName, description: formDesc, permission_ids: [...selectedPermIds] }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin', 'roles'] }); setShowCreate(false); resetForm() },
  })

  const updateMutation = useMutation({
    mutationFn: () => updateRole(editingRole!.id, { name: formName, description: formDesc, permission_ids: [...selectedPermIds] }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin', 'roles'] }); setEditingRole(null); resetForm() },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteRole(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'roles'] }),
  })

  const resetForm = () => { setFormName(''); setFormDesc(''); setSelectedPermIds(new Set()) }

  const openEdit = (role: Role) => {
    setEditingRole(role)
    setFormName(role.name)
    setFormDesc(role.description)
    setSelectedPermIds(new Set(role.permissions.map((p) => p.id)))
    setShowCreate(false)
  }

  const togglePerm = (id: number) =>
    setSelectedPermIds((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  const isEditing = !!(showCreate || editingRole)

  return (
    <div className="p-6 max-w-6xl flex gap-6">
      {/* Role list */}
      <div className="w-72 shrink-0">
        <div className="flex items-center justify-between mb-4">
          <h1 className="text-xl font-semibold">Roles</h1>
          <Button size="sm" onClick={() => { setShowCreate(true); setEditingRole(null); resetForm() }}>
            <Plus size={13} className="mr-1" /> New
          </Button>
        </div>
        <div className="space-y-2">
          {roles.map((role) => (
            <div
              key={role.id}
              className="flex items-center justify-between p-3 rounded-lg border border-border hover:bg-accent/40 cursor-pointer"
              onClick={() => openEdit(role)}
            >
              <div>
                <div className="flex items-center gap-1.5 text-sm font-medium">
                  {role.is_system && <Lock size={11} className="text-muted-foreground" />}
                  {role.name}
                </div>
                <div className="text-xs text-muted-foreground mt-0.5">{role.permissions.length} permissions</div>
              </div>
              {!role.is_system && (
                <button
                  className="text-muted-foreground hover:text-destructive p-1"
                  onClick={(e) => {
                    e.stopPropagation()
                    if (confirm(`Delete role "${role.name}"?`)) deleteMutation.mutate(role.id)
                  }}
                >
                  <Trash2 size={13} />
                </button>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Edit/Create panel */}
      {isEditing && (
        <div className="flex-1 border border-border rounded-lg p-5">
          <h2 className="text-base font-semibold mb-4">
            {editingRole ? `Edit: ${editingRole.name}` : 'Create Role'}
          </h2>
          <div className="space-y-4 max-w-md mb-6">
            <div>
              <Label className="text-xs">Role name</Label>
              <Input className="mt-1 h-8 text-sm" value={formName} onChange={(e) => setFormName(e.target.value)}
                placeholder="e.g. analyst" disabled={editingRole?.is_system} />
            </div>
            <div>
              <Label className="text-xs">Description</Label>
              <Input className="mt-1 h-8 text-sm" value={formDesc} onChange={(e) => setFormDesc(e.target.value)}
                placeholder="What can this role do?" />
            </div>
          </div>

          {/* Permission checkboxes grouped by resource */}
          <div className="space-y-4">
            <Label className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Permissions</Label>
            {Object.entries(grouped).map(([resource, perms]) => (
              <div key={resource}>
                <div className="text-xs font-medium text-muted-foreground mb-1.5 capitalize">{resource}</div>
                <div className="flex flex-wrap gap-2">
                  {perms.map((perm) => {
                    const checked = selectedPermIds.has(perm.id)
                    return (
                      <button
                        key={perm.id}
                        onClick={() => togglePerm(perm.id)}
                        title={perm.description}
                        className={`text-xs px-2.5 py-1 rounded-full border transition-colors ${
                          checked
                            ? 'bg-primary text-primary-foreground border-primary'
                            : 'border-border text-muted-foreground hover:border-primary'
                        }`}
                      >
                        {perm.action}
                      </button>
                    )
                  })}
                </div>
              </div>
            ))}
          </div>

          <div className="flex gap-2 mt-6">
            <Button size="sm" onClick={() => editingRole ? updateMutation.mutate() : createMutation.mutate()}>
              {editingRole ? 'Save Changes' : 'Create Role'}
            </Button>
            <Button size="sm" variant="outline" onClick={() => { setEditingRole(null); setShowCreate(false); resetForm() }}>
              Cancel
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
```

**Step 2: Commit**
```bash
git add frontend/src/pages/admin/RolesPage.tsx
git commit -m "feat(admin): add RolesPage with role list and permission assignment UI"
```

---

## Task F7: Create PermissionsPage, AuditLogPage, OverviewPage

**Files:**
- Create: `frontend/src/pages/admin/PermissionsPage.tsx`
- Create: `frontend/src/pages/admin/AuditLogPage.tsx`
- Create: `frontend/src/pages/admin/OverviewPage.tsx`

**PermissionsPage.tsx** — read-only browser:
```tsx
import { useQuery } from '@tanstack/react-query'
import { fetchPermissions } from '@/api/admin'
import { Badge } from '@/components/ui/badge'

export function PermissionsPage() {
  const { data: perms = [] } = useQuery({ queryKey: ['admin', 'permissions'], queryFn: fetchPermissions })
  const resources = [...new Set(perms.map((p) => p.resource))]

  return (
    <div className="p-6 max-w-4xl">
      <h1 className="text-xl font-semibold mb-1">Permissions</h1>
      <p className="text-sm text-muted-foreground mb-6">All system permissions. Add new ones in <code className="text-xs bg-muted px-1 rounded">backend/internal/admin/seed.go</code>.</p>
      <div className="space-y-6">
        {resources.map((resource) => (
          <div key={resource}>
            <h2 className="text-sm font-semibold capitalize mb-2">{resource}</h2>
            <div className="space-y-1">
              {perms.filter((p) => p.resource === resource).map((p) => (
                <div key={p.id} className="flex items-center gap-3 py-1.5 border-b border-border last:border-0">
                  <Badge variant="outline" className="text-xs font-mono">{p.name}</Badge>
                  <span className="text-xs text-muted-foreground">{p.description}</span>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
```

**AuditLogPage.tsx** — paginated audit log:
```tsx
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchAuditLogs } from '@/api/admin'
import { formatDistanceToNow } from 'date-fns'

export function AuditLogPage() {
  const [page, setPage] = useState(1)
  const { data } = useQuery({
    queryKey: ['admin', 'audit', page],
    queryFn: () => fetchAuditLogs({ page, limit: 50 }),
  })

  return (
    <div className="p-6 max-w-5xl">
      <h1 className="text-xl font-semibold mb-6">Audit Log</h1>
      <div className="rounded-md border border-border overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-muted/40 text-xs text-muted-foreground">
            <tr>
              <th className="text-left px-4 py-2">Action</th>
              <th className="text-left px-4 py-2">User</th>
              <th className="text-left px-4 py-2">Resource</th>
              <th className="text-left px-4 py-2">IP</th>
              <th className="text-left px-4 py-2">When</th>
            </tr>
          </thead>
          <tbody>
            {(data?.data ?? []).map((log) => (
              <tr key={log.id} className="border-t border-border hover:bg-muted/20">
                <td className="px-4 py-2 font-mono text-xs">{log.action}</td>
                <td className="px-4 py-2 text-xs text-muted-foreground">{log.user_email}</td>
                <td className="px-4 py-2 text-xs">{log.resource} #{log.resource_id}</td>
                <td className="px-4 py-2 text-xs text-muted-foreground font-mono">{log.ip_address}</td>
                <td className="px-4 py-2 text-xs text-muted-foreground">
                  {formatDistanceToNow(new Date(log.created_at), { addSuffix: true })}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {data && data.total > 50 && (
        <div className="flex gap-2 mt-4 justify-center">
          <button disabled={page === 1} onClick={() => setPage((p) => p - 1)}
            className="text-xs px-3 py-1.5 border rounded hover:bg-accent disabled:opacity-40">← Prev</button>
          <span className="text-xs px-3 py-1.5 text-muted-foreground">Page {page}</span>
          <button disabled={page * 50 >= data.total} onClick={() => setPage((p) => p + 1)}
            className="text-xs px-3 py-1.5 border rounded hover:bg-accent disabled:opacity-40">Next →</button>
        </div>
      )}
    </div>
  )
}
```

**OverviewPage.tsx** — system stats cards:
```tsx
import { useQuery } from '@tanstack/react-query'
import { fetchSystemStats } from '@/api/admin'
import { Users, Shield, Activity, HardDrive } from 'lucide-react'

export function OverviewPage() {
  const { data: stats } = useQuery({ queryKey: ['admin', 'stats'], queryFn: fetchSystemStats })
  if (!stats) return <div className="p-6 text-muted-foreground text-sm">Loading...</div>

  const cards = [
    { label: 'Total Users',     value: stats.users.total,   sub: `${stats.users.active} active`,  icon: Users    },
    { label: 'Roles',           value: stats.roles,          sub: 'defined',                        icon: Shield   },
    { label: 'Goroutines',      value: stats.goroutines,     sub: 'running',                        icon: Activity },
    { label: 'Memory',          value: `${stats.memory_mb}`, sub: 'MB in use',                      icon: HardDrive},
  ]

  return (
    <div className="p-6 max-w-4xl">
      <div className="mb-6">
        <h1 className="text-xl font-semibold">Overview</h1>
        <p className="text-xs text-muted-foreground mt-1">
          v{stats.version} · {stats.go_version} · up {Math.floor(stats.uptime_sec / 60)}m
        </p>
      </div>
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {cards.map(({ label, value, sub, icon: Icon }) => (
          <div key={label} className="rounded-lg border border-border p-4">
            <div className="flex items-center gap-2 text-muted-foreground mb-2">
              <Icon size={14} />
              <span className="text-xs">{label}</span>
            </div>
            <div className="text-2xl font-semibold">{value}</div>
            <div className="text-xs text-muted-foreground mt-0.5">{sub}</div>
          </div>
        ))}
      </div>
    </div>
  )
}
```

**Step 2: Commit**
```bash
git add frontend/src/pages/admin/PermissionsPage.tsx frontend/src/pages/admin/AuditLogPage.tsx frontend/src/pages/admin/OverviewPage.tsx
git commit -m "feat(admin): add PermissionsPage, AuditLogPage, OverviewPage"
```

---

## Task F8: Wire admin routes in App.tsx + add admin link to old app nav

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/layout/CommandBar.tsx`

**Step 1: Add admin routes to App.tsx**

Add inside the `<Routes>` block after the `/terminal` route:

```tsx
// ── Admin dashboard — requires admin role ──
import { AdminRoute }     from '@/components/admin/AdminRoute'
import { AdminLayout }    from '@/components/admin/AdminLayout'
import { OverviewPage }   from '@/pages/admin/OverviewPage'
import { UsersPage }      from '@/pages/admin/UsersPage'
import { RolesPage }      from '@/pages/admin/RolesPage'
import { PermissionsPage } from '@/pages/admin/PermissionsPage'
import { AuditLogPage }   from '@/pages/admin/AuditLogPage'

// In <Routes>:
<Route
  path="/admin"
  element={
    <AdminRoute>
      <AdminLayout />
    </AdminRoute>
  }
>
  <Route index element={<OverviewPage />} />
  <Route path="users"       element={<UsersPage />} />
  <Route path="roles"       element={<RolesPage />} />
  <Route path="permissions" element={<PermissionsPage />} />
  <Route path="audit"       element={<AuditLogPage />} />
</Route>
```

**Step 2: Add admin link to CommandBar** (only visible to admins)

In `frontend/src/components/layout/CommandBar.tsx`, import `useAuthStore` and add a conditional admin link:

```tsx
import { ShieldAlert } from 'lucide-react'
const user = useAuthStore((s) => s.user)

// In the nav JSX, after the existing navItems:
{user?.role === 'admin' && (
  <NavLink to="/admin" className={/* same styling as other nav links */}>
    <ShieldAlert size={16} />
    Admin
  </NavLink>
)}
```

**Step 3: Verify end-to-end**
```bash
make up
# Login as admin user
# Expected: "Admin" link visible in nav
# Click Admin → /admin loads with Overview stats
# Navigate to Users → table shows all users
# Navigate to Roles → 4 seeded roles visible
# Navigate to Permissions → 21 permissions grouped by resource
# Navigate to Audit → empty initially, actions populate it
# Non-admin user: no Admin link visible, /admin redirects to /
```

**Step 4: Commit**
```bash
git add frontend/src/App.tsx frontend/src/components/layout/CommandBar.tsx
git commit -m "feat(admin): wire /admin routes in App.tsx, add conditional Admin link to nav"
```

---

## Frontend Phase Completion Checklist

- [ ] shadcn table, badge, dialog, alert-dialog, tabs, toast components installed
- [ ] `types/admin.ts` — AdminUser, Role, Permission, AuditLog, SystemStats
- [ ] `api/admin.ts` — full CRUD API client for users, roles, audit, system
- [ ] `AdminRoute` guard — redirects non-admins to `/`
- [ ] `AdminLayout` — standalone admin sidebar shell with section nav + "Back to App"
- [ ] `OverviewPage` — system stats cards (users, roles, memory, goroutines)
- [ ] `UsersPage` — searchable data table with toggle/delete row actions
- [ ] `RolesPage` — role list + permission assignment with checkbox groups
- [ ] `PermissionsPage` — read-only browser grouped by resource
- [ ] `AuditLogPage` — paginated timeline with action/user/IP columns
- [ ] `/admin` routes wired in `App.tsx` with nested layout
- [ ] Admin link conditionally shown in CommandBar for admin users only
- [ ] Non-admin users cannot access `/admin` (redirected to `/`)
