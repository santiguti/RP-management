import { Link, NavLink, Outlet, useNavigate } from "react-router-dom"
import { LogOut } from "lucide-react"

import { Button } from "@/components/ui/button"
import { useAuth, useLogout } from "@/hooks/use-auth"
import { cn } from "@/lib/utils"

const linkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    "block rounded-md px-3 py-2 text-sm font-medium",
    isActive ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent hover:text-foreground",
  )

export function AppLayout() {
  const { data: user } = useAuth()
  const logout = useLogout()
  const nav = useNavigate()

  const onLogout = async () => {
    await logout.mutateAsync()
    nav("/login", { replace: true })
  }

  return (
    <div className="min-h-dvh bg-muted/30">
      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r bg-background p-4 md:flex md:flex-col">
        <Link to="/" className="px-3 py-2 text-lg font-semibold">
          RP Management
        </Link>
        <nav className="mt-6 space-y-1">
          <NavLink to="/" end className={linkClass}>
            Dashboard
          </NavLink>
          <NavLink to="/clients" className={linkClass}>
            Clientes
          </NavLink>
          <NavLink to="/work-orders" className={linkClass}>
            Órdenes de trabajo
          </NavLink>
          <NavLink to="/transactions" className={linkClass}>
            Movimientos
          </NavLink>
          <NavLink to="/suppliers" className={linkClass}>
            Proveedores
          </NavLink>
          {user?.role === "owner" ? (
            <NavLink to="/settings/lookups" className={linkClass}>
              Ajustes
            </NavLink>
          ) : null}
        </nav>
        <div className="mt-auto space-y-3 px-3">
          <div className="text-xs text-muted-foreground">
            <div className="font-medium text-foreground">{user?.full_name ?? user?.username}</div>
            <div>Rol: {user?.role}</div>
          </div>
          <Button type="button" variant="outline" className="w-full justify-start" onClick={onLogout}>
            <LogOut />
            Cerrar sesión
          </Button>
        </div>
      </aside>

      <div className="md:pl-64">
        <header className="flex items-center justify-between border-b bg-background px-4 py-3 md:hidden">
          <Link to="/" className="font-semibold">
            RP Management
          </Link>
          <Button type="button" variant="outline" size="sm" onClick={onLogout}>
            Salir
          </Button>
        </header>
        <main className="p-4 md:p-8">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
