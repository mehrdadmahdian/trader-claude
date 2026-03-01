import client from './client'
import type { AuthResponse, LoginRequest, RegisterRequest, UpdateProfileRequest, User } from '@/types'

export const login = async (data: LoginRequest): Promise<AuthResponse> => {
  const res = await client.post<AuthResponse>('/api/v1/auth/login', data)
  return res.data
}

export const register = async (data: RegisterRequest): Promise<AuthResponse> => {
  const res = await client.post<AuthResponse>('/api/v1/auth/register', data)
  return res.data
}

export const refreshToken = async (): Promise<{ access_token: string }> => {
  const res = await client.post<{ access_token: string }>('/api/v1/auth/refresh')
  return res.data
}

export const logout = async (): Promise<void> => {
  await client.post('/api/v1/auth/logout')
}

export const getMe = async (): Promise<User> => {
  const res = await client.get<User>('/api/v1/auth/me')
  return res.data
}

export const updateMe = async (data: UpdateProfileRequest): Promise<User> => {
  const res = await client.put<User>('/api/v1/auth/me', data)
  return res.data
}
