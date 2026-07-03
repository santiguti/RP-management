import { useMutation, useQueryClient } from "@tanstack/react-query"

import { commitImport, previewImport, type ImportKind } from "@/api/import"

export function usePreviewImport() {
  return useMutation({
    mutationFn: ({ kind, file }: { kind: ImportKind; file: File }) => previewImport(kind, file),
  })
}

export function useCommitImport() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ kind, file }: { kind: ImportKind; file: File }) => commitImport(kind, file),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["clients"] }),
        qc.invalidateQueries({ queryKey: ["parts"] }),
        qc.invalidateQueries({ queryKey: ["transactions"] }),
        qc.invalidateQueries({ queryKey: ["reports"] }),
        qc.invalidateQueries({ queryKey: ["dashboard"] }),
        qc.invalidateQueries({ queryKey: ["audit-log"] }),
      ])
    },
  })
}
