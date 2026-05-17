import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { WoStatus } from "@/api/work-orders"

const statusLabels: Record<WoStatus, string> = {
  received: "Recibido",
  diagnosing: "Diagnosticando",
  quoted: "Presupuestado",
  approved: "Aprobado",
  rejected: "Rechazado",
  in_repair: "En reparación",
  waiting_parts: "Esperando repuestos",
  ready: "Listo",
  delivered: "Entregado",
  cancelled: "Cancelado",
}

const statusClasses: Record<WoStatus, string> = {
  received: "bg-slate-200 text-slate-900",
  diagnosing: "bg-blue-100 text-blue-900",
  quoted: "bg-indigo-100 text-indigo-900",
  approved: "bg-violet-100 text-violet-900",
  rejected: "bg-orange-100 text-orange-900",
  in_repair: "bg-amber-100 text-amber-900",
  waiting_parts: "bg-yellow-100 text-yellow-900",
  ready: "bg-emerald-100 text-emerald-900",
  delivered: "bg-green-100 text-green-900",
  cancelled: "bg-red-100 text-red-900",
}

type WoStatusBadgeProps = {
  status: WoStatus
  className?: string
}

export function WoStatusBadge({ status, className }: WoStatusBadgeProps) {
  return (
    <Badge variant="secondary" className={cn(statusClasses[status], className)}>
      {statusLabels[status]}
    </Badge>
  )
}

export { statusLabels as woStatusLabels }
