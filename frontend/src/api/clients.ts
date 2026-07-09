import { apiClient } from "./http"
import type { Device } from "./devices"

export type Client = {
  ucode: string
  name: string
  phone?: string
  email?: string
  dni_cuit?: string
  address?: string
  notes?: string
  client_type: "particular" | "empresa"
  created_ts: string
}

export type ClientInput = Omit<Client, "ucode" | "created_ts">

export async function searchClients(q: string, page: number, pageSize: number) {
  const { data } = await apiClient.get<{
    clients: Client[]
    total: number
    page: number
    page_size: number
  }>("/clients", { params: { q, page, page_size: pageSize } })
  return data
}

export async function getClient(ucode: string) {
  const { data } = await apiClient.get<{ client: Client }>(`/clients/${ucode}`)
  return data.client
}

export async function createClient(input: ClientInput) {
  const { data } = await apiClient.post<{ client: Client }>("/clients", input)
  return data.client
}

export async function updateClient(ucode: string, input: Partial<ClientInput>) {
  const { data } = await apiClient.patch<{ client: Client }>(`/clients/${ucode}`, input)
  return data.client
}

export async function deleteClient(ucode: string) {
  await apiClient.delete(`/clients/${ucode}`)
}

export async function listClientDevices(ucode: string) {
  const { data } = await apiClient.get<{ devices: Device[] }>(`/clients/${ucode}/devices`)
  return data.devices
}
