import { apiClient } from "./client"

export type AttachmentPhase = "intake" | "diagnosis" | "during_repair" | "delivery"

export type Attachment = {
  ucode: string
  phase: AttachmentPhase
  original_filename: string
  mime_type: string
  size_bytes: number
  width?: number
  height?: number
  created_ts: string
}

export async function listAttachments(workOrderUcode: string) {
  const { data } = await apiClient.get<{ attachments: Attachment[] }>(`/work-orders/${workOrderUcode}/attachments`)
  return data.attachments
}

export async function uploadAttachment(workOrderUcode: string, file: File, phase: AttachmentPhase) {
  const body = new FormData()
  body.append("file", file)
  body.append("phase", phase)
  const { data } = await apiClient.post<{ attachment: Attachment }>(`/work-orders/${workOrderUcode}/attachments`, body)
  return data.attachment
}

export async function deleteAttachment(workOrderUcode: string, attachmentUcode: string) {
  await apiClient.delete(`/work-orders/${workOrderUcode}/attachments/${attachmentUcode}`)
}
