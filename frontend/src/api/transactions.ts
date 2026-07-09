import { apiClient } from "./http"

export type TransactionType = "income" | "expense"

export type PaymentMethod = "cash" | "transfer" | "card" | "mercadopago" | "other"

export type TransactionCategory =
  | "wo_payment"
  | "wo_deposit"
  | "part_purchase"
  | "supplies"
  | "rent"
  | "utilities"
  | "salary"
  | "taxes"
  | "food"
  | "transport"
  | "other_income"
  | "other_expense"

export type CounterpartyType = "client" | "supplier" | "none"

export type CounterpartyRef = {
  ucode: string
  name: string
}

export type WorkOrderRef = {
  ucode: string
  wo_number: string
}

export type Transaction = {
  ucode: string
  transaction_type: TransactionType
  amount: string
  currency: string
  fx_rate_to_ars: string
  transaction_date: string
  payment_method: PaymentMethod
  category: TransactionCategory
  counterparty_type: CounterpartyType
  client?: CounterpartyRef
  supplier?: CounterpartyRef
  work_order?: WorkOrderRef
  recurring_expense?: CounterpartyRef
  description?: string
  created_ts: string
}

export type TransactionInput = {
  transaction_type: TransactionType
  amount: string
  currency?: string
  fx_rate_to_ars?: string
  transaction_date?: string
  payment_method: PaymentMethod
  category: TransactionCategory
  counterparty_type: CounterpartyType
  client_ucode?: string
  supplier_ucode?: string
  work_order_ucode?: string
  description?: string
}

export type UpdateTransactionInput = Partial<
  Pick<TransactionInput, "transaction_date" | "payment_method" | "category" | "description">
>

export type ListTransactionsParams = {
  from?: string
  to?: string
  type?: TransactionType | ""
  category?: TransactionCategory | ""
  work_order_ucode?: string
  page?: number
  page_size?: number
}

export type ListTransactionsResponse = {
  transactions: Transaction[]
  total: number
  page: number
  page_size: number
}

export async function listTransactions(params: ListTransactionsParams = {}) {
  const { data } = await apiClient.get<ListTransactionsResponse>("/transactions", { params })
  return data
}

export async function getTransaction(ucode: string) {
  const { data } = await apiClient.get<{ transaction: Transaction }>(`/transactions/${ucode}`)
  return data.transaction
}

export async function createTransaction(input: TransactionInput) {
  const { data } = await apiClient.post<{ transaction: Transaction }>("/transactions", input)
  return data.transaction
}

export async function updateTransaction(ucode: string, input: UpdateTransactionInput) {
  const { data } = await apiClient.patch<{ transaction: Transaction }>(`/transactions/${ucode}`, input)
  return data.transaction
}

export async function deleteTransaction(ucode: string) {
  await apiClient.delete(`/transactions/${ucode}`)
}

export async function listWorkOrderTransactions(woUcode: string) {
  const { data } = await apiClient.get<{ transactions: Transaction[] }>(
    `/work-orders/${woUcode}/transactions`
  )
  return data.transactions
}
