import { useMemo, useState } from "react"
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"

import type { PnLCategory } from "@/api/reports"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useBalance, usePnl } from "@/hooks/use-reports"
import { categoryLabels, formatARSValue } from "@/lib/money"
import { cn } from "@/lib/utils"

type ChartRow = PnLCategory & {
  label: string
  total: number
}

export function ReportsPage() {
  const initialRange = useMemo(() => defaultRange(), [])
  const [draftFrom, setDraftFrom] = useState(initialRange.from)
  const [draftTo, setDraftTo] = useState(initialRange.to)
  const [range, setRange] = useState(initialRange)
  const balance = useBalance(range)
  const pnl = usePnl(range)
  const incomeRows = useMemo(() => toChartRows(pnl.data?.income ?? []), [pnl.data?.income])
  const expenseRows = useMemo(() => toChartRows(pnl.data?.expense ?? []), [pnl.data?.expense])

  const net = Number(balance.data?.net_ars ?? 0)

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Reportes</h1>
          <p className="text-sm text-muted-foreground">Balance y P&L por categoría.</p>
        </div>
        <div className="grid gap-3 rounded-md border bg-background p-3 sm:grid-cols-[10rem_10rem_auto]">
          <DateField label="Desde" value={draftFrom} onChange={setDraftFrom} />
          <DateField label="Hasta" value={draftTo} onChange={setDraftTo} />
          <Button
            type="button"
            className="self-end"
            onClick={() => setRange({ from: draftFrom, to: draftTo })}
          >
            Aplicar
          </Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <MetricCard
          label="Ingresos"
          value={balance.data?.income_ars ?? "0"}
          className="text-emerald-700"
          isLoading={balance.isLoading}
        />
        <MetricCard
          label="Egresos"
          value={balance.data?.expense_ars ?? "0"}
          className="text-destructive"
          isLoading={balance.isLoading}
        />
        <MetricCard
          label="Balance"
          value={balance.data?.net_ars ?? "0"}
          className={cn(net > 0 && "text-emerald-700", net < 0 && "text-destructive", net === 0 && "text-slate-700")}
          isLoading={balance.isLoading}
        />
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <CategoryPanel
          title="Ingresos por categoría"
          rows={incomeRows}
          isLoading={pnl.isLoading}
          barColor="#047857"
        />
        <CategoryPanel
          title="Egresos por categoría"
          rows={expenseRows}
          isLoading={pnl.isLoading}
          barColor="#dc2626"
        />
      </div>
    </div>
  )
}

function DateField({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      <Input type="date" value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  )
}

function MetricCard({
  label,
  value,
  className,
  isLoading,
}: {
  label: string
  value: string
  className?: string
  isLoading?: boolean
}) {
  return (
    <Card className="rounded-md">
      <CardHeader>
        <CardTitle className="text-sm text-muted-foreground">{label}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className={cn("text-2xl font-semibold tracking-normal", className)}>
          {isLoading ? "..." : formatARSValue(value)}
        </div>
      </CardContent>
    </Card>
  )
}

function CategoryPanel({
  title,
  rows,
  isLoading,
  barColor,
}: {
  title: string
  rows: ChartRow[]
  isLoading?: boolean
  barColor: string
}) {
  return (
    <Card className="rounded-md">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <div className="flex h-72 items-center justify-center rounded-md border text-sm text-muted-foreground">
            Cargando...
          </div>
        ) : rows.length > 0 ? (
          <div className="h-72">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={rows} margin={{ top: 8, right: 16, bottom: 36, left: 12 }}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis
                  dataKey="label"
                  angle={-25}
                  textAnchor="end"
                  interval={0}
                  height={72}
                  tick={{ fontSize: 12 }}
                />
                <YAxis tickFormatter={(value) => shortMoney(value)} tick={{ fontSize: 12 }} />
                <Tooltip formatter={(value) => formatARSValue(Number(value))} />
                <Bar dataKey="total" fill={barColor} radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        ) : (
          <div className="flex h-72 items-center justify-center rounded-md border text-sm text-muted-foreground">
            No hay datos en este período
          </div>
        )}

        <div className="overflow-hidden rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Categoría</TableHead>
                <TableHead className="text-right">Total</TableHead>
                <TableHead className="text-right">Cantidad</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.length > 0 ? (
                rows.map((row) => (
                  <TableRow key={row.category}>
                    <TableCell>{row.label}</TableCell>
                    <TableCell className="text-right">{formatARSValue(row.total_ars)}</TableCell>
                    <TableCell className="text-right">{row.count}</TableCell>
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell colSpan={3} className="h-20 text-center text-muted-foreground">
                    No hay datos en este período
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  )
}

function toChartRows(rows: PnLCategory[]): ChartRow[] {
  return [...rows]
    .sort((a, b) => Number(b.total_ars) - Number(a.total_ars))
    .map((row) => ({
      ...row,
      label: categoryLabels[row.category as keyof typeof categoryLabels] ?? row.category,
      total: Number(row.total_ars),
    }))
}

function defaultRange() {
  const to = new Date()
  const from = new Date(to)
  from.setDate(to.getDate() - 29)
  return {
    from: toISODate(from),
    to: toISODate(to),
  }
}

function toISODate(date: Date) {
  return date.toISOString().slice(0, 10)
}

function shortMoney(value: number | string) {
  const amount = Number(value)
  if (!Number.isFinite(amount)) return "$0"
  if (Math.abs(amount) >= 1_000_000) return `$${Math.round(amount / 1_000_000)}M`
  if (Math.abs(amount) >= 1_000) return `$${Math.round(amount / 1_000)}k`
  return `$${amount}`
}
