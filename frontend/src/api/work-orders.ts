import { apiClient } from "./client"

export type WoStatus =
  | "received"
  | "diagnosing"
  | "quoted"
  | "approved"
  | "rejected"
  | "in_repair"
  | "waiting_parts"
  | "ready"
  | "delivered"
  | "cancelled"

export type WoEvent =
  | "start_diagnosis"
  | "quote"
  | "approve"
  | "reject"
  | "start_repair"
  | "mark_waiting_parts"
  | "resume_repair"
  | "mark_ready"
  | "deliver"
  | "cancel"

export type WorkOrder = {
  ucode: string
  wo_number: string
  status: WoStatus
  service_type: "in_shop" | "on_site"
  client: { ucode: string; name: string; phone?: string }
  device: {
    ucode: string
    brand_name: string
    model_name?: string
    article_type_name: string
    serial_number?: string
  }
  reported_issue: string
  diagnosis?: string
  quote_amount?: string
  quote_currency: string
  labor_amount?: string
  parts_amount?: string
  final_amount?: string
  intake_notes?: string
  accessories?: string
  device_pin?: string
  cancel_reason?: string
  received_ts: string
  started_ts?: string
  quote_sent_ts?: string
  quote_approved_ts?: string
  quote_rejected_ts?: string
  ready_ts?: string
  delivered_ts?: string
  cancelled_ts?: string
  allowed_events: WoEvent[]
}

export type WorkOrderListResponse = {
  work_orders: WorkOrder[]
  total: number
  page: number
  page_size: number
}

export type IntakeInput = {
  client_ucode: string
  device_ucode: string
  service_type: "in_shop" | "on_site"
  reported_issue: string
  intake_notes?: string
  accessories?: string
  device_pin?: string
}

export type TransitionInput = {
  quote_amount?: string
  quote_currency?: string
  diagnosis?: string
  labor_amount?: string
  parts_amount?: string
  final_amount?: string
  cancel_reason?: string
}

export type SearchWorkOrdersParams = {
  q?: string
  status?: WoStatus | ""
  client_ucode?: string
  page?: number
  page_size?: number
}

export type UpdateWorkOrderInput = Partial<
  Pick<
    WorkOrder,
    "service_type" | "reported_issue" | "diagnosis" | "intake_notes" | "accessories" | "device_pin"
  >
>

export async function searchWorkOrders(params: SearchWorkOrdersParams) {
  const { data } = await apiClient.get<WorkOrderListResponse>("/work-orders", { params })
  return data
}

export async function intakeWorkOrder(input: IntakeInput) {
  const { data } = await apiClient.post<{ work_order: WorkOrder }>("/work-orders", input)
  return data.work_order
}

export async function getWorkOrder(ucode: string) {
  const { data } = await apiClient.get<{ work_order: WorkOrder }>(`/work-orders/${ucode}`)
  return data.work_order
}

export async function updateWorkOrder(ucode: string, input: UpdateWorkOrderInput) {
  const { data } = await apiClient.patch<{ work_order: WorkOrder }>(`/work-orders/${ucode}`, input)
  return data.work_order
}

export async function transitionWorkOrder(
  ucode: string,
  event: WoEvent,
  input: TransitionInput = {}
) {
  const { data } = await apiClient.post<{ work_order: WorkOrder }>(
    `/work-orders/${ucode}/transitions/${event}`,
    input
  )
  return data.work_order
}
