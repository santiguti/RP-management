import type { ReactNode } from "react"
import { Navigate } from "react-router-dom"
import { useAuth } from "@/hooks/use-auth"

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { data, isPending, isError } = useAuth()
  if (isPending) return <div className="p-8 text-sm text-slate-500">Cargando…</div>
  if (isError || !data) return <Navigate to="/login" replace />
  return <>{children}</>
}
