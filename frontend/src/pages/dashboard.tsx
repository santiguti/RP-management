import { Link, useNavigate } from "react-router-dom"

import type { LowStockPart, MoneySummary, TopClientRevenue } from "@/api/reports"
import type { WoStatus } from "@/api/work-orders"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useAuth } from "@/hooks/use-auth"
import { useDashboard } from "@/hooks/use-reports"
import { formatARSValue } from "@/lib/money"
import { cn } from "@/lib/utils"
import { woStatusLabels } from "@/components/wo-status-badge"

export function DashboardPage() {
  const { data: user } = useAuth()
  const dashboard = useDashboard()
  const nav = useNavigate()
  const data = dashboard.data

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 rounded-md border bg-background p-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">¡Hola, {user?.full_name ?? user?.username}!</h1>
          <p className="text-sm text-muted-foreground">Resumen operativo y financiero.</p>
        </div>
        <Badge variant="outline" className="capitalize">
          {user?.role}
        </Badge>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <MoneySummaryCard title="Hoy" summary={data?.today} isLoading={dashboard.isLoading} />
        <MoneySummaryCard title="Este mes" summary={data?.month} isLoading={dashboard.isLoading} />
      </div>

      <Card className="rounded-md">
        <CardHeader>
          <CardTitle>Órdenes de trabajo abiertas</CardTitle>
        </CardHeader>
        <CardContent>
          {dashboard.isLoading ? (
            <p className="text-sm text-muted-foreground">Cargando...</p>
          ) : data && Object.keys(data.open_work_orders_by_status).length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {Object.entries(data.open_work_orders_by_status).map(([status, count]) => (
                <Button
                  key={status}
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-8 gap-2"
                  onClick={() => nav(`/work-orders?status=${status}`)}
                >
                  <span>{statusLabel(status)}</span>
                  <Badge variant="secondary">{count}</Badge>
                </Button>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Sin órdenes abiertas.</p>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card className="rounded-md">
          <CardHeader>
            <CardTitle>Repuestos bajo stock</CardTitle>
          </CardHeader>
          <CardContent>
            {dashboard.isLoading ? (
              <p className="text-sm text-muted-foreground">Cargando...</p>
            ) : data && data.low_stock_parts.length > 0 ? (
              <div className="overflow-x-auto rounded-md border">
                <div className="grid min-w-[32rem] grid-cols-[1fr_80px_100px_90px] gap-3 border-b bg-muted/30 px-3 py-2 text-xs font-medium text-muted-foreground">
                  <span>Nombre</span>
                  <span className="text-right">Stock</span>
                  <span className="text-right">Reposición</span>
                  <span className="text-right">Faltante</span>
                </div>
                <div className="divide-y">
                  {data.low_stock_parts.map((part) => (
                    <LowStockPartRow key={part.ucode} part={part} />
                  ))}
                </div>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Sin repuestos por debajo del punto de reposición</p>
            )}
          </CardContent>
        </Card>

        <Card className="rounded-md">
          <CardHeader>
            <CardTitle>Órdenes listas hace +7 días</CardTitle>
          </CardHeader>
          <CardContent>
            {dashboard.isLoading ? (
              <p className="text-sm text-muted-foreground">Cargando...</p>
            ) : data && data.aging_ready_work_orders.length > 0 ? (
              <div className="divide-y rounded-md border">
                {data.aging_ready_work_orders.map((wo) => (
                  <Link
                    key={wo.ucode}
                    to={`/work-orders/${wo.ucode}`}
                    className="block px-3 py-2 text-sm hover:bg-accent"
                  >
                    <span className="font-medium">{wo.wo_number}</span>
                    <span className="text-muted-foreground"> · {wo.client_name} · hace {wo.days_ready} días</span>
                  </Link>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Sin órdenes atrasadas — todo entregado a tiempo.</p>
            )}
          </CardContent>
        </Card>

        <Card className="rounded-md">
          <CardHeader>
            <CardTitle>Top clientes (últimos 90 días)</CardTitle>
          </CardHeader>
          <CardContent>
            {dashboard.isLoading ? (
              <p className="text-sm text-muted-foreground">Cargando...</p>
            ) : data && data.top_clients_90d.length > 0 ? (
              <div className="divide-y rounded-md border">
                {data.top_clients_90d.map((client) => (
                  <TopClientRow key={client.ucode} client={client} />
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Sin movimientos en los últimos 90 días.</p>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function MoneySummaryCard({
  title,
  summary,
  isLoading,
}: {
  title: string
  summary?: MoneySummary
  isLoading?: boolean
}) {
  return (
    <Card className="rounded-md">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3 sm:grid-cols-3">
        <MoneyMetric label="Ingresos" value={summary?.income_ars} className="text-emerald-700" isLoading={isLoading} />
        <MoneyMetric label="Egresos" value={summary?.expense_ars} className="text-destructive" isLoading={isLoading} />
        <MoneyMetric
          label="Balance"
          value={summary?.net_ars}
          className={netClass(summary?.net_ars)}
          isLoading={isLoading}
        />
      </CardContent>
    </Card>
  )
}

function MoneyMetric({
  label,
  value,
  className,
  isLoading,
}: {
  label: string
  value?: string
  className?: string
  isLoading?: boolean
}) {
  return (
    <div className="rounded-md border bg-muted/20 p-3">
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
      <div className={cn("mt-1 text-lg font-semibold tracking-normal", className)}>
        {isLoading ? "..." : formatARSValue(value ?? "0")}
      </div>
    </div>
  )
}

function TopClientRow({ client }: { client: TopClientRevenue }) {
  return (
    <div className="flex items-center justify-between gap-3 px-3 py-2 text-sm">
      <span className="font-medium">{client.name}</span>
      <span className="text-muted-foreground">{formatARSValue(client.total_ars)}</span>
    </div>
  )
}

function LowStockPartRow({ part }: { part: LowStockPart }) {
  return (
    <Link
      to={`/parts/${part.ucode}`}
      className="grid min-w-[32rem] grid-cols-[1fr_80px_100px_90px] gap-3 px-3 py-2 text-sm hover:bg-accent"
    >
      <span className="min-w-0 truncate font-medium">{part.name}</span>
      <span className="text-right text-muted-foreground">{formatQuantity(part.current_stock, part.unit)}</span>
      <span className="text-right text-muted-foreground">{formatQuantity(part.reorder_level, part.unit)}</span>
      <span className="text-right font-medium text-destructive">{formatQuantity(part.deficit, part.unit)}</span>
    </Link>
  )
}

function statusLabel(status: string) {
  return woStatusLabels[status as WoStatus] ?? status
}

function netClass(value?: string) {
  const net = Number(value ?? 0)
  if (net > 0) return "text-emerald-700"
  if (net < 0) return "text-destructive"
  return "text-slate-700"
}

function formatQuantity(value: string, unit: string) {
  const normalized = Number(value).toLocaleString("es-AR", { maximumFractionDigits: 2 })
  return `${normalized} ${unit}`
}
