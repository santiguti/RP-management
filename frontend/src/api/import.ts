import { apiClient } from "./http"

export type ImportKind = "clients" | "parts" | "transactions"

export type ImportRowError = {
  row: number
  column: string
  message: string
}

export type ImportResult = {
  kind: ImportKind
  total_rows: number
  valid: number
  invalid: number
  preview: unknown[]
  errors: ImportRowError[]
  committed: boolean
  inserted_ucodes?: string[]
}

export async function previewImport(kind: ImportKind, file: File) {
  return sendImport(kind, file, false)
}

export async function commitImport(kind: ImportKind, file: File) {
  return sendImport(kind, file, true)
}

async function sendImport(kind: ImportKind, file: File, confirm: boolean) {
  const form = new FormData()
  form.append("file", file)
  const { data } = await apiClient.post<ImportResult>("/import/excel", form, {
    params: { kind, confirm: confirm ? "true" : undefined },
  })
  return data
}
