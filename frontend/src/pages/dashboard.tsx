import { useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useAuth, useLogout } from "@/hooks/use-auth"

export function DashboardPage() {
  const { data: user } = useAuth()
  const logout = useLogout()
  const nav = useNavigate()

  const onLogout = async () => {
    await logout.mutateAsync()
    nav("/login", { replace: true })
  }

  return (
    <div className="min-h-dvh bg-slate-50 p-8">
      <Card className="max-w-2xl">
        <CardHeader>
          <CardTitle>¡Hola, {user?.full_name ?? user?.username}!</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-between">
          <span className="text-sm text-slate-500">Rol: {user?.role}</span>
          <Button variant="outline" onClick={onLogout}>Cerrar sesión</Button>
        </CardContent>
      </Card>
    </div>
  )
}
