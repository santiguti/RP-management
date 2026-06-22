import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  addWorkOrderPart,
  listWorkOrderParts,
  removeWorkOrderPart,
  type WorkOrderPartInput,
} from "@/api/wo-parts"

export function useWorkOrderParts(workOrderUcode: string | undefined) {
  return useQuery({
    queryKey: ["work-orders", workOrderUcode, "parts"],
    queryFn: () => listWorkOrderParts(workOrderUcode ?? ""),
    enabled: Boolean(workOrderUcode),
  })
}

function useInvalidateWorkOrderParts() {
  const qc = useQueryClient()
  return async (workOrderUcode: string) => {
    await qc.invalidateQueries({ queryKey: ["work-orders", workOrderUcode, "parts"] })
    await qc.invalidateQueries({ queryKey: ["work-orders", workOrderUcode] })
    await qc.invalidateQueries({ queryKey: ["parts"] })
  }
}

export function useAddWorkOrderPart() {
  const invalidate = useInvalidateWorkOrderParts()
  return useMutation({
    mutationFn: ({ workOrderUcode, input }: { workOrderUcode: string; input: WorkOrderPartInput }) =>
      addWorkOrderPart(workOrderUcode, input),
    onSuccess: async (_part, vars) => invalidate(vars.workOrderUcode),
  })
}

export function useRemoveWorkOrderPart() {
  const invalidate = useInvalidateWorkOrderParts()
  return useMutation({
    mutationFn: ({ workOrderUcode, id }: { workOrderUcode: string; id: number }) =>
      removeWorkOrderPart(workOrderUcode, id),
    onSuccess: async (_unused, vars) => invalidate(vars.workOrderUcode),
  })
}
