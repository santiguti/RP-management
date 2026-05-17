import type { ReactNode } from "react"
import { Navigate } from "react-router-dom"

import { useAuth } from "@/hooks/use-auth"

export function RequireOwner({ children }: { children: ReactNode }) {
  const { data } = useAuth()
  if (data?.role !== "owner") return <Navigate to="/" replace />
  return <>{children}</>
}
