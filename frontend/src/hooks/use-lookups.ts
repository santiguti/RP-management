import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  createArticleType,
  createBrand,
  createDeviceModel,
  deleteArticleType,
  deleteBrand,
  deleteDeviceModel,
  listArticleTypes,
  listBrands,
  listDeviceModelsByBrand,
  updateArticleType,
  updateBrand,
  updateDeviceModel,
} from "@/api/lookups"

export function useBrands() {
  return useQuery({ queryKey: ["brands"], queryFn: listBrands })
}

export function useArticleTypes() {
  return useQuery({ queryKey: ["article-types"], queryFn: listArticleTypes })
}

export function useDeviceModels(brandUcode: string | null) {
  return useQuery({
    queryKey: ["device-models", brandUcode],
    queryFn: () => listDeviceModelsByBrand(brandUcode ?? ""),
    enabled: Boolean(brandUcode),
  })
}

export function useCreateBrand() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createBrand,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["brands"] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "brands"] })
    },
  })
}

export function useUpdateBrand() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: updateBrand,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["brands"] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "brands"] })
    },
  })
}

export function useDeleteBrand() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteBrand,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["brands"] })
      await qc.invalidateQueries({ queryKey: ["device-models"] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "brands"] })
    },
  })
}

export function useCreateDeviceModel(brandUcode: string | null) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createDeviceModel,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["device-models", brandUcode] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "device-models", brandUcode] })
    },
  })
}

export function useUpdateDeviceModel(brandUcode: string | null) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: updateDeviceModel,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["device-models", brandUcode] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "device-models", brandUcode] })
    },
  })
}

export function useDeleteDeviceModel(brandUcode: string | null) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteDeviceModel,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["device-models", brandUcode] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "device-models", brandUcode] })
    },
  })
}

export function useCreateArticleType() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createArticleType,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["article-types"] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "article-types"] })
    },
  })
}

export function useUpdateArticleType() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: updateArticleType,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["article-types"] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "article-types"] })
    },
  })
}

export function useDeleteArticleType() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteArticleType,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["article-types"] })
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "article-types"] })
    },
  })
}
