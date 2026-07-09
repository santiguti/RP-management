import { apiClient } from "./http"
import type { CounterpartyRef, PaymentMethod, Transaction, TransactionCategory } from "./transactions"

export type RecurringExpenseCategory = Extract<
  TransactionCategory,
  "rent" | "utilities" | "salary" | "taxes" | "supplies" | "other_expense"
>

export type RecurringExpense = {
  ucode: string
  name: string
  amount: string
  currency: string
  day_of_month: number
  category: RecurringExpenseCategory
  payment_method: PaymentMethod
  supplier?: CounterpartyRef
  description?: string
  active: boolean
  last_generated_date?: string
  created_ts: string
}

export type RecurringExpenseInput = {
  name: string
  amount: string
  currency?: string
  day_of_month: number
  category: RecurringExpenseCategory
  payment_method?: PaymentMethod
  supplier_ucode?: string
  description?: string
  active?: boolean
}

export type UpdateRecurringExpenseInput = Partial<RecurringExpenseInput>

export type RunRecurringNowResponse = {
  transaction: Transaction | null
  reason?: "already_generated_for_due_date"
}

export async function listRecurringExpenses() {
  const { data } = await apiClient.get<{ recurring_expenses: RecurringExpense[] }>(
    "/recurring-expenses"
  )
  return data.recurring_expenses
}

export async function getRecurringExpense(ucode: string) {
  const { data } = await apiClient.get<{ recurring_expense: RecurringExpense }>(
    `/recurring-expenses/${ucode}`
  )
  return data.recurring_expense
}

export async function createRecurringExpense(input: RecurringExpenseInput) {
  const { data } = await apiClient.post<{ recurring_expense: RecurringExpense }>(
    "/recurring-expenses",
    input
  )
  return data.recurring_expense
}

export async function updateRecurringExpense(ucode: string, input: UpdateRecurringExpenseInput) {
  const { data } = await apiClient.patch<{ recurring_expense: RecurringExpense }>(
    `/recurring-expenses/${ucode}`,
    input
  )
  return data.recurring_expense
}

export async function deleteRecurringExpense(ucode: string) {
  await apiClient.delete(`/recurring-expenses/${ucode}`)
}

export async function runRecurringNow(ucode: string) {
  const { data } = await apiClient.post<RunRecurringNowResponse>(
    `/recurring-expenses/${ucode}/run-now`,
    {}
  )
  return data
}
