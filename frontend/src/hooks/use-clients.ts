import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  createClient,
  deleteClient,
  getClient,
  listClientDevices,
  searchClients,
  updateClient,
  type ClientInput,
} from "@/api/clients"

export function useClients(q: string, page: number, pageSize: number) {
  return useQuery({
    queryKey: ["clients", q, page, pageSize],
    queryFn: () => searchClients(q, page, pageSize),
  })
}

export function useClient(ucode: string | undefined) {
  return useQuery({
    queryKey: ["clients", ucode],
    queryFn: () => getClient(ucode ?? ""),
    enabled: Boolean(ucode),
  })
}

export function useClientDevices(ucode: string | undefined) {
  return useQuery({
    queryKey: ["clients", ucode, "devices"],
    queryFn: () => listClientDevices(ucode ?? ""),
    enabled: Boolean(ucode),
  })
}

export function useCreateClient() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createClient,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["clients"] })
    },
  })
}

export function useUpdateClient() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ ucode, input }: { ucode: string; input: Partial<ClientInput> }) =>
      updateClient(ucode, input),
    onSuccess: async (_client, vars) => {
      await qc.invalidateQueries({ queryKey: ["clients"] })
      await qc.invalidateQueries({ queryKey: ["clients", vars.ucode] })
    },
  })
}

export function useDeleteClient() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteClient,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["clients"] })
    },
  })
}
