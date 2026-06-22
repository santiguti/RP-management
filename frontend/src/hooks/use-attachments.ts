import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  deleteAttachment,
  listAttachments,
  uploadAttachment,
  type AttachmentPhase,
} from "@/api/attachments"

export function useAttachments(workOrderUcode: string | undefined) {
  return useQuery({
    queryKey: ["work-orders", workOrderUcode, "attachments"],
    queryFn: () => listAttachments(workOrderUcode ?? ""),
    enabled: Boolean(workOrderUcode),
  })
}

function useInvalidateAttachments() {
  const qc = useQueryClient()
  return async (workOrderUcode: string) => {
    await qc.invalidateQueries({ queryKey: ["work-orders", workOrderUcode, "attachments"] })
  }
}

export function useUploadAttachment() {
  const invalidate = useInvalidateAttachments()
  return useMutation({
    mutationFn: ({ workOrderUcode, file, phase }: { workOrderUcode: string; file: File; phase: AttachmentPhase }) =>
      uploadAttachment(workOrderUcode, file, phase),
    onSuccess: async (_attachment, vars) => invalidate(vars.workOrderUcode),
  })
}

export function useDeleteAttachment() {
  const invalidate = useInvalidateAttachments()
  return useMutation({
    mutationFn: ({ workOrderUcode, attachmentUcode }: { workOrderUcode: string; attachmentUcode: string }) =>
      deleteAttachment(workOrderUcode, attachmentUcode),
    onSuccess: async (_unused, vars) => invalidate(vars.workOrderUcode),
  })
}
