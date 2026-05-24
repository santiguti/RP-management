import { apiClient } from "./client"

export type BalanceReport = {
  from: string
  to: string
  income_ars: string
  expense_ars: string
  net_ars: string
  transaction_count: number
}

export type PnLCategory = {
  category: string
  total_ars: string
  count: number
}

export type PnLReport = {
  from: string
  to: string
  income: PnLCategory[]
  expense: PnLCategory[]
}

export type MoneySummary = {
  income_ars: string
  expense_ars: string
  net_ars: string
}

export type AgingReadyWorkOrder = {
  ucode: string
  wo_number: string
  ready_ts: string
  client_name: string
  days_ready: number
}

export type TopClientRevenue = {
  ucode: string
  name: string
  total_ars: string
}

export type DashboardReport = {
  today: MoneySummary
  month: MoneySummary
  open_work_orders_by_status: Record<string, number>
  aging_ready_work_orders: AgingReadyWorkOrder[]
  top_clients_90d: TopClientRevenue[]
}

export type DateRangeParams = {
  from?: string
  to?: string
}

export type PnLParams = DateRangeParams & {
  period?: "month" | "year"
}

export async function getBalance(params: DateRangeParams = {}) {
  const { data } = await apiClient.get<BalanceReport>("/reports/balance", { params })
  return data
}

export async function getPnl(params: PnLParams = {}) {
  const { data } = await apiClient.get<PnLReport>("/reports/pnl", { params })
  return data
}

export async function getDashboard() {
  const { data } = await apiClient.get<DashboardReport>("/reports/dashboard")
  return data
}
