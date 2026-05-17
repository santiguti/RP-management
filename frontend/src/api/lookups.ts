import { apiClient } from "./client"

export type Lookup = {
  ucode: string
  name: string
}

export type Brand = Lookup
export type ArticleType = Lookup

export type DeviceModel = Lookup & {
  brand_ucode: string
}

export async function listBrands() {
  const { data } = await apiClient.get<{ brands: Brand[] }>("/brands")
  return data.brands
}

export async function createBrand(name: string) {
  const { data } = await apiClient.post<{ brand: Brand }>("/brands", { name })
  return data.brand
}

export async function updateBrand(input: { ucode: string; name: string }) {
  const { data } = await apiClient.patch<{ brand: Brand }>(`/brands/${input.ucode}`, {
    name: input.name,
  })
  return data.brand
}

export async function deleteBrand(ucode: string) {
  await apiClient.delete(`/brands/${ucode}`)
}

export async function listArticleTypes() {
  const { data } = await apiClient.get<{ article_types: ArticleType[] }>("/article-types")
  return data.article_types
}

export async function createArticleType(name: string) {
  const { data } = await apiClient.post<{ article_type: ArticleType }>("/article-types", { name })
  return data.article_type
}

export async function updateArticleType(input: { ucode: string; name: string }) {
  const { data } = await apiClient.patch<{ article_type: ArticleType }>(
    `/article-types/${input.ucode}`,
    { name: input.name },
  )
  return data.article_type
}

export async function deleteArticleType(ucode: string) {
  await apiClient.delete(`/article-types/${ucode}`)
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

export async function updateDeviceModel(input: { ucode: string; name: string }) {
  const { data } = await apiClient.patch<{ device_model: DeviceModel }>(
    `/device-models/${input.ucode}`,
    { name: input.name },
  )
  return data.device_model
}

export async function deleteDeviceModel(ucode: string) {
  await apiClient.delete(`/device-models/${ucode}`)
}
