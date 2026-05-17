import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  getWorkOrder,
  intakeWorkOrder,
  searchWorkOrders,
  transitionWorkOrder,
  updateWorkOrder,
  type IntakeInput,
  type SearchWorkOrdersParams,
  type TransitionInput,
  type UpdateWorkOrderInput,
  type WoEvent,
} from "@/api/work-orders"

export function useWorkOrders(params: SearchWorkOrdersParams) {
  return useQuery({
    queryKey: ["work-orders", params],
    queryFn: () => searchWorkOrders(params),
  })
}

export function useWorkOrder(ucode: string | undefined) {
  return useQuery({
    queryKey: ["work-orders", ucode],
    queryFn: () => getWorkOrder(ucode ?? ""),
    enabled: Boolean(ucode),
  })
}

export function useIntakeWorkOrder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: intakeWorkOrder,
    onSuccess: async (workOrder) => {
      await qc.invalidateQueries({ queryKey: ["work-orders"] })
      await qc.invalidateQueries({ queryKey: ["work-orders", workOrder.ucode] })
    },
  })
}

export function useUpdateWorkOrder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ucode, input }: { ucode: string; input: UpdateWorkOrderInput }) =>
      updateWorkOrder(ucode, input),
    onSuccess: async (_workOrder, vars) => {
      await qc.invalidateQueries({ queryKey: ["work-orders"] })
      await qc.invalidateQueries({ queryKey: ["work-orders", vars.ucode] })
    },
  })
}

export function useTransitionWorkOrder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      ucode,
      event,
      input = {},
    }: {
      ucode: string
      event: WoEvent
      input?: TransitionInput
    }) => transitionWorkOrder(ucode, event, input),
    onSuccess: async (_workOrder, vars) => {
      await qc.invalidateQueries({ queryKey: ["work-orders"] })
      await qc.invalidateQueries({ queryKey: ["work-orders", vars.ucode] })
    },
  })
}

export type { IntakeInput, SearchWorkOrdersParams, TransitionInput, UpdateWorkOrderInput }
