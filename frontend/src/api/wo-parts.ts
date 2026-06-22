import { apiClient } from "./client"

export type WorkOrderPart = {
  id: number
  part_ucode: string
  part_name: string
  part_unit: string
  quantity: string
  unit_price_charged: string
  cost_unit?: string
  subtotal: string
  created_ts: string
}

export type WorkOrderPartInput = {
  part_ucode: string
  quantity: string
  unit_price_charged: string
  cost_unit?: string
}

export async function listWorkOrderParts(workOrderUcode: string) {
  const { data } = await apiClient.get<{ work_order_parts: WorkOrderPart[] }>(`/work-orders/${workOrderUcode}/parts`)
  return data.work_order_parts
}

export async function addWorkOrderPart(workOrderUcode: string, input: WorkOrderPartInput) {
  const { data } = await apiClient.post<{ work_order_part: WorkOrderPart }>(`/work-orders/${workOrderUcode}/parts`, input)
  return data.work_order_part
}

export async function removeWorkOrderPart(workOrderUcode: string, id: number) {
  await apiClient.delete(`/work-orders/${workOrderUcode}/parts/${id}`)
}
