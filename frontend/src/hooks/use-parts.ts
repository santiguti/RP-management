import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  createMovement,
  createPart,
  deletePart,
  getPart,
  listMovements,
  searchParts,
  updatePart,
  type MovementInput,
  type PartInput,
  type SearchPartsParams,
} from "@/api/parts"

export function useParts(params: SearchPartsParams) {
  return useQuery({
    queryKey: ["parts", params],
    queryFn: () => searchParts(params),
  })
}

export function usePart(ucode: string | undefined) {
  return useQuery({
    queryKey: ["parts", ucode],
    queryFn: () => getPart(ucode ?? ""),
    enabled: Boolean(ucode),
  })
}

export function usePartMovements(ucode: string | undefined, page = 1, pageSize = 25) {
  return useQuery({
    queryKey: ["parts", ucode, "movements", page, pageSize],
    queryFn: () => listMovements(ucode ?? "", page, pageSize),
    enabled: Boolean(ucode),
  })
}

function useInvalidateParts() {
  const qc = useQueryClient()
  return async (ucode?: string) => {
    await qc.invalidateQueries({ queryKey: ["parts"] })
    if (ucode) {
      await qc.invalidateQueries({ queryKey: ["parts", ucode] })
      await qc.invalidateQueries({ queryKey: ["parts", ucode, "movements"] })
    }
    await qc.invalidateQueries({ queryKey: ["reports"] })
  }
}

export function useCreatePart() {
  const invalidate = useInvalidateParts()
  return useMutation({
    mutationFn: createPart,
    onSuccess: async (part) => invalidate(part.ucode),
  })
}

export function useUpdatePart() {
  const invalidate = useInvalidateParts()
  return useMutation({
    mutationFn: ({ ucode, input }: { ucode: string; input: Partial<PartInput> }) => updatePart(ucode, input),
    onSuccess: async (_part, vars) => invalidate(vars.ucode),
  })
}

export function useDeletePart() {
  const invalidate = useInvalidateParts()
  return useMutation({
    mutationFn: deletePart,
    onSuccess: async (_unused, ucode) => invalidate(ucode),
  })
}

export function useCreateMovement() {
  const invalidate = useInvalidateParts()
  return useMutation({
    mutationFn: ({ ucode, input }: { ucode: string; input: MovementInput }) => createMovement(ucode, input),
    onSuccess: async (_movement, vars) => invalidate(vars.ucode),
  })
}

export type { MovementInput, PartInput, SearchPartsParams }
