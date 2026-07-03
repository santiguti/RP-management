import { useMemo, useState } from "react"
import { useSearchParams } from "react-router-dom"

import type { AuditEntry, ListAuditLogParams } from "@/api/audit-log"
import { DataTable, type Column } from "@/components/data-table"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useAuditLog } from "@/hooks/use-audit-log"

const PAGE_SIZE = 25

const entityOptions = [
  { value: "", label: "Todas" },
  { value: "client", label: "Cliente" },
  { value: "supplier", label: "Proveedor" },
  { value: "work_order", label: "Orden de trabajo" },
  { value: "transaction", label: "Movimiento" },
  { value: "recurring_expense", label: "Gasto fijo" },
  { value: "part", label: "Repuesto" },
  { value: "part_movement", label: "Movimiento de repuesto" },
  { value: "attachment", label: "Adjunto" },
  { value: "auth", label: "Autenticación" },
  { value: "user", label: "Usuario" },
  { value: "work_order_part", label: "Repuesto en OT" },
  { value: "import", label: "Importación" },
]

const entityLabels = Object.fromEntries(entityOptions.map((option) => [option.value, option.label]))

const actionLabels: Record<string, string> = {
  "auth.login": "Inicio de sesión",
  "client.create": "Cliente creado",
  "client.update": "Cliente actualizado",
  "client.delete": "Cliente eliminado",
  "supplier.create": "Proveedor creado",
  "supplier.update": "Proveedor actualizado",
  "supplier.delete": "Proveedor eliminado",
  "wo.create": "OT creada",
  "wo.update": "OT actualizada",
  "wo.transition": "Transición de OT",
  "wo.part.add": "Repuesto agregado a OT",
  "wo.part.delete": "Repuesto quitado de OT",
  "transaction.create": "Movimiento creado",
  "transaction.update": "Movimiento actualizado",
  "transaction.delete": "Movimiento eliminado",
  "recurring_expense.create": "Gasto fijo creado",
  "recurring_expense.update": "Gasto fijo actualizado",
  "recurring_expense.delete": "Gasto fijo eliminado",
  "part.create": "Repuesto creado",
  "part.update": "Repuesto actualizado",
  "part.delete": "Repuesto eliminado",
  "part_movement.create": "Movimiento de repuesto creado",
  "attachment.create": "Adjunto creado",
  "attachment.delete": "Adjunto eliminado",
  "import.excel": "Importación de Excel",
}

export function AuditLogPage() {
  const [params, setParams] = useSearchParams()
  const [selected, setSelected] = useState<AuditEntry | undefined>()
  const page = Number(params.get("page") ?? "1") || 1
  const filters = {
    actor: params.get("actor") ?? "",
    entity_type: params.get("entity_type") ?? "",
    action: params.get("action") ?? "",
    from: params.get("from") ?? "",
    to: params.get("to") ?? "",
  }

  const queryParams = useMemo<ListAuditLogParams>(
    () => ({
      actor: filters.actor || undefined,
      entity_type: filters.entity_type || undefined,
      action: filters.action || undefined,
      from: filters.from ? startOfDay(filters.from) : undefined,
      to: filters.to ? endOfDay(filters.to) : undefined,
      page,
      page_size: PAGE_SIZE,
    }),
    [filters.action, filters.actor, filters.entity_type, filters.from, filters.to, page],
  )
  const auditLog = useAuditLog(queryParams)

  const columns = useMemo<Column<AuditEntry>[]>(
    () => [
      { header: "Fecha", cell: (row) => formatDate(row.created_ts) },
      { header: "Usuario", cell: (row) => row.actor_full_name ?? row.actor_username ?? "-" },
      { header: "Acción", cell: (row) => actionLabel(row.action) },
      { header: "Entidad", cell: (row) => entityLabel(row.entity_type) },
      {
        header: "Cambios",
        className: "w-36 text-right",
        cell: (row) => (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={(event) => {
              event.stopPropagation()
              setSelected(row)
            }}
          >
            Ver cambios
          </Button>
        ),
      },
    ],
    [],
  )

  const updateParams = (next: Record<string, string | number | undefined>) => {
    const out = new URLSearchParams(params)
    for (const [key, value] of Object.entries(next)) {
      if (value === undefined || value === "") out.delete(key)
      else out.set(key, String(value))
    }
    if (!("page" in next)) out.set("page", "1")
    setParams(out)
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-normal">Bitácora</h1>
        <p className="text-sm text-muted-foreground">Actividad registrada por usuario, entidad y acción.</p>
      </div>

      <div className="grid gap-3 rounded-md border bg-background p-3 sm:grid-cols-2 xl:grid-cols-6">
        <FilterField label="Usuario">
          <input
            className={inputClassName}
            value={filters.actor}
            onChange={(event) => updateParams({ actor: event.target.value })}
            placeholder="username"
          />
        </FilterField>
        <FilterField label="Entidad">
          <select
            className={inputClassName}
            value={filters.entity_type}
            onChange={(event) => updateParams({ entity_type: event.target.value })}
          >
            {entityOptions.map((option) => (
              <option key={option.value || "all"} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </FilterField>
        <FilterField label="Acción">
          <input
            className={inputClassName}
            value={filters.action}
            onChange={(event) => updateParams({ action: event.target.value })}
            placeholder="client.update"
          />
        </FilterField>
        <FilterField label="Desde">
          <input
            type="date"
            className={inputClassName}
            value={filters.from}
            onChange={(event) => updateParams({ from: event.target.value })}
          />
        </FilterField>
        <FilterField label="Hasta">
          <input
            type="date"
            className={inputClassName}
            value={filters.to}
            onChange={(event) => updateParams({ to: event.target.value })}
          />
        </FilterField>
        <div className="flex items-end">
          <Button type="button" variant="outline" className="w-full" onClick={() => setParams(new URLSearchParams())}>
            Limpiar filtros
          </Button>
        </div>
      </div>

      <DataTable
        columns={columns}
        rows={auditLog.data?.entries ?? []}
        rowKey={(row) => row.ucode}
        onRowClick={setSelected}
        isLoading={auditLog.isLoading}
        emptyMessage="No hay eventos de bitácora"
        page={auditLog.data?.page ?? page}
        pageSize={auditLog.data?.page_size ?? PAGE_SIZE}
        total={auditLog.data?.total ?? 0}
        onPageChange={(nextPage) => updateParams({ page: nextPage })}
        searchValue=""
        onSearchChange={() => undefined}
        showSearch={false}
      />

      <Dialog open={Boolean(selected)} onOpenChange={(open) => !open && setSelected(undefined)}>
        <DialogContent className="max-h-[85dvh] overflow-hidden sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle>{selected ? actionLabel(selected.action) : "Cambios"}</DialogTitle>
            <DialogDescription>
              {selected ? `${formatDate(selected.created_ts)} · ${entityLabel(selected.entity_type)}` : ""}
            </DialogDescription>
          </DialogHeader>
          <div className="grid min-h-0 gap-3 overflow-auto lg:grid-cols-2">
            <JsonPanel title="Antes" value={selected?.before} />
            <JsonPanel title="Después" value={selected?.after} />
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function FilterField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="space-y-1 text-sm">
      <span className="font-medium">{label}</span>
      {children}
    </label>
  )
}

function JsonPanel({ title, value }: { title: string; value: unknown }) {
  return (
    <section className="min-w-0 space-y-2">
      <h2 className="text-sm font-medium">{title}</h2>
      <pre className="max-h-[52dvh] overflow-auto rounded-md border bg-muted/30 p-3 text-xs leading-relaxed">
        {JSON.stringify(value ?? null, null, 2)}
      </pre>
    </section>
  )
}

function formatDate(value?: string) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("es-AR", { dateStyle: "short", timeStyle: "short" }).format(new Date(value))
}

function actionLabel(action: string) {
  return actionLabels[action] ?? action
}

function entityLabel(entityType: string) {
  return entityLabels[entityType] ?? entityType
}

function startOfDay(value: string) {
  return new Date(`${value}T00:00:00.000`).toISOString()
}

function endOfDay(value: string) {
  return new Date(`${value}T23:59:59.999`).toISOString()
}

const inputClassName =
  "border-input bg-background h-9 w-full rounded-md border px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
