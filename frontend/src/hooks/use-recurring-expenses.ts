import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  createRecurringExpense,
  deleteRecurringExpense,
  getRecurringExpense,
  listRecurringExpenses,
  runRecurringNow,
  updateRecurringExpense,
  type RecurringExpenseInput,
  type UpdateRecurringExpenseInput,
} from "@/api/recurring-expenses"

export function useRecurringExpenses() {
  return useQuery({
    queryKey: ["recurring-expenses"],
    queryFn: listRecurringExpenses,
  })
}

export function useRecurringExpense(ucode: string | undefined) {
  return useQuery({
    queryKey: ["recurring-expenses", ucode],
    queryFn: () => getRecurringExpense(ucode ?? ""),
    enabled: Boolean(ucode),
  })
}

export function useCreateRecurringExpense() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createRecurringExpense,
    onSuccess: async (expense) => {
      await qc.invalidateQueries({ queryKey: ["recurring-expenses"] })
      await qc.invalidateQueries({ queryKey: ["recurring-expenses", expense.ucode] })
    },
  })
}

export function useUpdateRecurringExpense() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ucode, input }: { ucode: string; input: UpdateRecurringExpenseInput }) =>
      updateRecurringExpense(ucode, input),
    onSuccess: async (_expense, vars) => {
      await qc.invalidateQueries({ queryKey: ["recurring-expenses"] })
      await qc.invalidateQueries({ queryKey: ["recurring-expenses", vars.ucode] })
    },
  })
}

export function useDeleteRecurringExpense() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteRecurringExpense,
    onSuccess: async (_unused, ucode) => {
      await qc.invalidateQueries({ queryKey: ["recurring-expenses"] })
      await qc.invalidateQueries({ queryKey: ["recurring-expenses", ucode] })
      await qc.invalidateQueries({ queryKey: ["transactions"] })
    },
  })
}

export function useRunRecurringNow() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: runRecurringNow,
    onSuccess: async (_result, ucode) => {
      await qc.invalidateQueries({ queryKey: ["recurring-expenses"] })
      await qc.invalidateQueries({ queryKey: ["recurring-expenses", ucode] })
      await qc.invalidateQueries({ queryKey: ["transactions"] })
      await qc.invalidateQueries({ queryKey: ["reports"] })
    },
  })
}

export type { RecurringExpenseInput, UpdateRecurringExpenseInput }
