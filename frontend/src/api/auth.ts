import { apiClient } from "./http"

export type User = {
  ucode: string
  username: string
  full_name: string
  role: "owner" | "employee"
}

export async function login(username: string, password: string): Promise<User> {
  const { data } = await apiClient.post<{ user: User }>("/auth/login", { username, password })
  return data.user
}

export async function logout(): Promise<void> {
  await apiClient.post("/auth/logout")
}

export async function me(): Promise<User> {
  const { data } = await apiClient.get<{ user: User }>("/auth/me")
  return data.user
}
