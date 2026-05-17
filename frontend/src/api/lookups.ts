import { apiClient } from "./client"

export type Lookup = {
  ucode: string
  name: string
}

export type DeviceModel = Lookup & {
  brand_ucode: string
}

export async function listBrands() {
  const { data } = await apiClient.get<{ brands: Lookup[] }>("/brands")
  return data.brands
}

export async function createBrand(name: string) {
  const { data } = await apiClient.post<{ brand: Lookup }>("/brands", { name })
  return data.brand
}

export async function listArticleTypes() {
  const { data } = await apiClient.get<{ article_types: Lookup[] }>("/article-types")
  return data.article_types
}

export async function createArticleType(name: string) {
  const { data } = await apiClient.post<{ article_type: Lookup }>("/article-types", { name })
  return data.article_type
}

export async function listDeviceModelsByBrand(brandUcode: string) {
  const { data } = await apiClient.get<{ device_models: DeviceModel[] }>(
    `/brands/${brandUcode}/models`,
  )
  return data.device_models
}

export async function createDeviceModel(input: { brand_ucode: string; name: string }) {
  const { data } = await apiClient.post<{ device_model: DeviceModel }>("/device-models", input)
  return data.device_model
}
