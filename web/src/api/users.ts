import type { Envelope, ListParams, User } from '../types/api'
import { request, requestData } from './client'

const base = '/api/v1/users'

function query(params: ListParams): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  const encoded = search.toString()
  return encoded ? `?${encoded}` : ''
}

/** Admin only — the listing exposes every account's email address. */
export async function listUsers(params: ListParams = {}): Promise<Envelope<User[]>> {
  return request<User[]>(`${base}${query(params)}`)
}

/** Your own record, or anybody's if you are an admin. */
export async function getUser(id: number): Promise<User> {
  return requestData<User>(`${base}/${id}`)
}

export interface UpdateUserPayload {
  name?: string
  email?: string
  /** Admin-only, and rejected outright on your own row. */
  role?: 'user' | 'admin'
  /** Admin-only. */
  is_active?: boolean
}

export async function updateUser(id: number, payload: UpdateUserPayload): Promise<User> {
  return requestData<User>(`${base}/${id}`, { method: 'PATCH', body: payload })
}

export async function createUser(payload: {
  name: string
  email: string
  password: string
  role?: 'user' | 'admin'
}): Promise<User> {
  return requestData<User>(base, { method: 'POST', body: payload })
}

export async function deleteUser(id: number): Promise<void> {
  await request(`${base}/${id}`, { method: 'DELETE' })
}
