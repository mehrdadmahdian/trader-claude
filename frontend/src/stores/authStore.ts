import { create } from 'zustand'
import * as authApi from '@/api/auth'
import type { User } from '@/types'

interface AuthState {
  user: User | null
  accessToken: string | null
  isAuthenticated: boolean
  isLoading: boolean

  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, displayName: string) => Promise<void>
  logout: () => Promise<void>
  refreshToken: () => Promise<void>
  initialize: () => Promise<void>
  setUser: (user: User | null) => void
  clearAuth: () => void
  setAccessToken: (token: string) => void
}

export const useAuthStore = create<AuthState>()((set, get) => ({
  user: null,
  accessToken: null,
  isAuthenticated: false,
  isLoading: true,

  login: async (email, password) => {
    const res = await authApi.login({ email, password })
    set({
      user: res.user,
      accessToken: res.access_token,
      isAuthenticated: true,
    })
  },

  register: async (email, password, displayName) => {
    const res = await authApi.register({ email, password, display_name: displayName })
    set({
      user: res.user,
      accessToken: res.access_token,
      isAuthenticated: true,
    })
  },

  logout: async () => {
    try {
      await authApi.logout()
    } catch {
      // Proceed with local logout even if server call fails
    }
    get().clearAuth()
  },

  refreshToken: async () => {
    const res = await authApi.refreshToken()
    set({ accessToken: res.access_token, isAuthenticated: true })
  },

  initialize: async () => {
    set({ isLoading: true })
    try {
      const res = await authApi.refreshToken()
      set({ accessToken: res.access_token, isAuthenticated: true })
      // Fetch user profile after getting a valid token
      const user = await authApi.getMe()
      set({ user, isLoading: false })
    } catch {
      set({ user: null, accessToken: null, isAuthenticated: false, isLoading: false })
    }
  },

  setUser: (user) => set({ user }),
  clearAuth: () => set({ user: null, accessToken: null, isAuthenticated: false }),
  setAccessToken: (token) => set({ accessToken: token, isAuthenticated: true }),
}))
