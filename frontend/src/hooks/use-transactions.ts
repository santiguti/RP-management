import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  createTransaction,
  deleteTransaction,
  getTransaction,
  listTransactions,
  listWorkOrderTransactions,
  updateTransaction,
  type ListTransactionsParams,
  type TransactionInput,
  type UpdateTransactionInput,
} from "@/api/transactions"

export function useTransactions(params: ListTransactionsParams) {
  return useQuery({
    queryKey: ["transactions", params],
    queryFn: () => listTransactions(params),
  })
}

export function useTransaction(ucode: string | undefined) {
  return useQuery({
    queryKey: ["transactions", ucode],
    queryFn: () => getTransaction(ucode ?? ""),
    enabled: Boolean(ucode),
  })
}

export function useWorkOrderTransactions(woUcode: string | undefined) {
  return useQuery({
    queryKey: ["work-orders", woUcode, "transactions"],
    queryFn: () => listWorkOrderTransactions(woUcode ?? ""),
    enabled: Boolean(woUcode),
  })
}

export function useCreateTransaction() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createTransaction,
    onSuccess: async (transaction, input) => {
      await invalidateTransactionQueries(qc, transaction.work_order?.ucode ?? input.work_order_ucode)
      await qc.invalidateQueries({ queryKey: ["transactions", transaction.ucode] })
    },
  })
}

export function useUpdateTransaction() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ucode, input }: { ucode: string; input: UpdateTransactionInput }) =>
      updateTransaction(ucode, input),
    onSuccess: async (transaction, vars) => {
      await invalidateTransactionQueries(qc, transaction.work_order?.ucode)
      await qc.invalidateQueries({ queryKey: ["transactions", vars.ucode] })
    },
  })
}

export function useDeleteTransaction() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ucode }: { ucode: string; workOrderUcode?: string }) => deleteTransaction(ucode),
    onSuccess: async (_unused, vars) => {
      await invalidateTransactionQueries(qc, vars.workOrderUcode)
      await qc.invalidateQueries({ queryKey: ["transactions", vars.ucode] })
    },
  })
}

async function invalidateTransactionQueries(
  qc: ReturnType<typeof useQueryClient>,
  workOrderUcode?: string
) {
  await qc.invalidateQueries({ queryKey: ["transactions"] })
  await qc.invalidateQueries({ queryKey: ["reports"] })
  await qc.invalidateQueries({ queryKey: ["work-orders"] })
  if (workOrderUcode) {
    await qc.invalidateQueries({ queryKey: ["work-orders", workOrderUcode] })
    await qc.invalidateQueries({ queryKey: ["work-orders", workOrderUcode, "transactions"] })
  }
}

export type { ListTransactionsParams, TransactionInput, UpdateTransactionInput }
