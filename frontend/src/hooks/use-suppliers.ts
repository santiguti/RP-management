import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  createSupplier,
  deleteSupplier,
  getSupplier,
  listSuppliers,
  updateSupplier,
  type SupplierInput,
} from "@/api/suppliers"

export function useSuppliers() {
  return useQuery({
    queryKey: ["suppliers"],
    queryFn: listSuppliers,
  })
}

export function useSupplier(ucode: string | undefined) {
  return useQuery({
    queryKey: ["suppliers", ucode],
    queryFn: () => getSupplier(ucode ?? ""),
    enabled: Boolean(ucode),
  })
}

export function useCreateSupplier() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createSupplier,
    onSuccess: async (supplier) => {
      await qc.invalidateQueries({ queryKey: ["suppliers"] })
      await qc.invalidateQueries({ queryKey: ["suppliers", supplier.ucode] })
    },
  })
}

export function useUpdateSupplier() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ucode, input }: { ucode: string; input: Partial<SupplierInput> }) =>
      updateSupplier(ucode, input),
    onSuccess: async (_supplier, vars) => {
      await qc.invalidateQueries({ queryKey: ["suppliers"] })
      await qc.invalidateQueries({ queryKey: ["suppliers", vars.ucode] })
    },
  })
}

export function useDeleteSupplier() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteSupplier,
    onSuccess: async (_unused, ucode) => {
      await qc.invalidateQueries({ queryKey: ["suppliers"] })
      await qc.invalidateQueries({ queryKey: ["suppliers", ucode] })
    },
  })
}

export type { SupplierInput }
