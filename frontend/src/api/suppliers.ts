import { apiClient } from "./http"

export type Supplier = {
  ucode: string
  name: string
  phone?: string
  email?: string
  notes?: string
  created_ts: string
}

export type SupplierInput = {
  name: string
  phone?: string
  email?: string
  notes?: string
}

export async function listSuppliers() {
  const { data } = await apiClient.get<{ suppliers: Supplier[] }>("/suppliers")
  return data.suppliers
}

export async function getSupplier(ucode: string) {
  const { data } = await apiClient.get<{ supplier: Supplier }>(`/suppliers/${ucode}`)
  return data.supplier
}

export async function createSupplier(input: SupplierInput) {
  const { data } = await apiClient.post<{ supplier: Supplier }>("/suppliers", input)
  return data.supplier
}

export async function updateSupplier(ucode: string, input: Partial<SupplierInput>) {
  const { data } = await apiClient.patch<{ supplier: Supplier }>(`/suppliers/${ucode}`, input)
  return data.supplier
}

export async function deleteSupplier(ucode: string) {
  await apiClient.delete(`/suppliers/${ucode}`)
}
