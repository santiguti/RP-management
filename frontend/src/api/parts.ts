import { apiClient } from "./client"

export type Part = {
  ucode: string
  sku?: string
  name: string
  description?: string
  unit: string
  current_stock: string
  reorder_level?: string
  default_cost?: string
  default_sale_price?: string
  low_stock: boolean
  created_ts: string
}

export type PartInput = {
  sku?: string
  name: string
  description?: string
  unit?: string
  reorder_level?: string
  default_cost?: string
  default_sale_price?: string
}

export type Movement = {
  ucode: string
  movement_type: "purchase" | "use" | "adjustment" | "return"
  quantity: string
  unit_cost?: string
  supplier?: { ucode: string; name: string }
  work_order?: { ucode: string; wo_number: string }
  transaction?: { ucode: string }
  notes?: string
  created_ts: string
}

export type MovementInput = {
  movement_type: "purchase" | "adjustment" | "return"
  quantity: string
  adjustment_out?: boolean
  unit_cost?: string
  supplier_ucode?: string
  notes?: string
  link_transaction?: boolean
}

export type SearchPartsParams = {
  q?: string
  low_stock?: boolean
  page?: number
  page_size?: number
}

export type PartsResponse = {
  parts: Part[]
  total: number
  page: number
  page_size: number
}

export type MovementsResponse = {
  movements: Movement[]
  total: number
  page: number
  page_size: number
}

export async function searchParts(params: SearchPartsParams) {
  const { data } = await apiClient.get<PartsResponse>("/parts", { params })
  return data
}

export async function getPart(ucode: string) {
  const { data } = await apiClient.get<{ part: Part }>(`/parts/${ucode}`)
  return data.part
}

export async function createPart(input: PartInput) {
  const { data } = await apiClient.post<{ part: Part }>("/parts", input)
  return data.part
}

export async function updatePart(ucode: string, input: Partial<PartInput>) {
  const { data } = await apiClient.patch<{ part: Part }>(`/parts/${ucode}`, input)
  return data.part
}

export async function deletePart(ucode: string) {
  await apiClient.delete(`/parts/${ucode}`)
}

export async function listMovements(partUcode: string, page = 1, pageSize = 25) {
  const { data } = await apiClient.get<MovementsResponse>(`/parts/${partUcode}/movements`, {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function createMovement(partUcode: string, input: MovementInput) {
  const { data } = await apiClient.post<{ movement: Movement }>(`/parts/${partUcode}/movements`, input)
  return data.movement
}
