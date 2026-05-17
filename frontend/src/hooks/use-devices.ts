import { useMutation, useQueryClient } from "@tanstack/react-query"

import { createDevice, deleteDevice, updateDevice, type DeviceInput } from "@/api/devices"

export function useCreateDevice(clientUcode?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createDevice,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["clients", clientUcode, "devices"] })
    },
  })
}

export function useUpdateDevice(clientUcode?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ucode, input }: { ucode: string; input: Partial<DeviceInput> }) =>
      updateDevice(ucode, input),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["clients", clientUcode, "devices"] })
    },
  })
}

export function useDeleteDevice(clientUcode?: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteDevice,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["clients", clientUcode, "devices"] })
    },
  })
}
