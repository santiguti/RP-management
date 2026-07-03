import { useQuery } from "@tanstack/react-query"

import { listAuditLog, type ListAuditLogParams } from "@/api/audit-log"

export function useAuditLog(params: ListAuditLogParams) {
  return useQuery({
    queryKey: ["audit-log", params],
    queryFn: () => listAuditLog(params),
  })
}

export type { ListAuditLogParams }
