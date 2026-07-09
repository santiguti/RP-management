import { apiClient } from "./http"

export type Device = {
  ucode: string
  client_ucode: string
  brand_ucode: string
  model_ucode?: string
  article_type_ucode: string
  serial_number?: string
  color?: string
  description?: string
  created_ts: string
}

export type DeviceInput = {
  client_ucode: string
  brand_ucode: string
  model_ucode?: string
  article_type_ucode: string
  serial_number?: string
  color?: string
  description?: string
}

export async function createDevice(input: DeviceInput) {
  const { data } = await apiClient.post<{ device: Device }>("/devices", input)
  return data.device
}

export async function updateDevice(ucode: string, input: Partial<DeviceInput>) {
  const { data } = await apiClient.patch<{ device: Device }>(`/devices/${ucode}`, input)
  return data.device
}

export async function deleteDevice(ucode: string) {
  await apiClient.delete(`/devices/${ucode}`)
}
