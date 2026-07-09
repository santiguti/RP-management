import { apiClient } from "./http"

export type AuditEntry = {
  ucode: string
  created_ts: string
  action: string
  entity_type: string
  entity_ucode?: string
  actor_username?: string
  actor_full_name?: string
  before?: unknown
  after?: unknown
  ip?: string
  user_agent?: string
}

export type ListAuditLogParams = {
  actor?: string
  entity_type?: string
  entity_ucode?: string
  action?: string
  from?: string
  to?: string
  page?: number
  page_size?: number
}

export type AuditLogResponse = {
  entries: AuditEntry[]
  total: number
  page: number
  page_size: number
}

export async function listAuditLog(params: ListAuditLogParams = {}) {
  const { data } = await apiClient.get<AuditLogResponse>("/audit-log", { params })
  return data
}
