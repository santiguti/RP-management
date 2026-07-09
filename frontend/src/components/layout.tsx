import { useState } from "react"
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom"
import { LogOut, Menu } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { useAuth, useLogout } from "@/hooks/use-auth"
import { cn } from "@/lib/utils"

const linkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    "block rounded-md px-3 py-2 text-sm font-medium",
    isActive ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent hover:text-foreground",
  )

function NavLinks({ isOwner, onNavigate }: { isOwner: boolean; onNavigate?: () => void }) {
  return (
    <nav className="space-y-1">
      <NavLink to="/" end className={linkClass} onClick={onNavigate}>
        Dashboard
      </NavLink>
      <NavLink to="/clients" className={linkClass} onClick={onNavigate}>
        Clientes
      </NavLink>
      <NavLink to="/work-orders" className={linkClass} onClick={onNavigate}>
        Órdenes de trabajo
      </NavLink>
      <NavLink to="/parts" className={linkClass} onClick={onNavigate}>
        Repuestos
      </NavLink>
      <NavLink to="/transactions" className={linkClass} onClick={onNavigate}>
        Movimientos
      </NavLink>
      <NavLink to="/suppliers" className={linkClass} onClick={onNavigate}>
        Proveedores
      </NavLink>
      <NavLink to="/reports" className={linkClass} onClick={onNavigate}>
        Reportes
      </NavLink>
      {isOwner ? (
        <div className="pt-3">
          <div className="px-3 pb-1 text-xs font-semibold uppercase tracking-normal text-muted-foreground">
            Ajustes
          </div>
          <NavLink to="/settings/lookups" className={linkClass} onClick={onNavigate}>
            Catálogos
          </NavLink>
          <NavLink to="/settings/recurring-expenses" className={linkClass} onClick={onNavigate}>
            Gastos fijos
          </NavLink>
          <NavLink to="/import" className={linkClass} onClick={onNavigate}>
            Importar
          </NavLink>
          <NavLink to="/settings/audit-log" className={linkClass} onClick={onNavigate}>
            Bitácora
          </NavLink>
        </div>
      ) : null}
    </nav>
  )
}

export function AppLayout() {
  const { data: user } = useAuth()
  const logout = useLogout()
  const nav = useNavigate()
  const [mobileOpen, setMobileOpen] = useState(false)
  const isOwner = user?.role === "owner"

  const onLogout = async () => {
    await logout.mutateAsync()
    setMobileOpen(false)
    nav("/login", { replace: true })
  }

  return (
    <div className="min-h-dvh bg-muted/30">
      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r bg-background p-4 md:flex md:flex-col">
        <Link to="/" className="px-3 py-2 text-lg font-semibold">
          RP Management
        </Link>
        <div className="mt-6">
          <NavLinks isOwner={isOwner} />
        </div>
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
          <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
            <SheetTrigger asChild>
              <Button type="button" variant="ghost" size="icon" aria-label="Abrir menú">
                <Menu />
              </Button>
            </SheetTrigger>
            <SheetContent side="left" className="p-4">
              <SheetHeader>
                <SheetTitle>Menú</SheetTitle>
              </SheetHeader>
              <Link to="/" className="px-3 py-2 text-lg font-semibold" onClick={() => setMobileOpen(false)}>
                RP Management
              </Link>
              <NavLinks isOwner={isOwner} onNavigate={() => setMobileOpen(false)} />
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
            </SheetContent>
          </Sheet>
          <Link to="/" className="font-semibold">
            RP Management
          </Link>
          <div className="size-9" aria-hidden="true" />
        </header>
        <main className="p-4 md:p-8">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
