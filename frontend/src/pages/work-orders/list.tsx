import { useEffect, useMemo, useState } from "react"
import { Link, useNavigate, useSearchParams } from "react-router-dom"
import { Plus, Search } from "lucide-react"

import type { Client } from "@/api/clients"
import { searchClients } from "@/api/clients"
import type { WorkOrder, WoStatus } from "@/api/work-orders"
import { DataTable, type Column } from "@/components/data-table"
import { EntityCombobox } from "@/components/entity-combobox"
import { WoStatusBadge, woStatusLabels } from "@/components/wo-status-badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useDebounce } from "@/hooks/use-debounce"
import { useWorkOrders } from "@/hooks/use-work-orders"

const LIST_PAGE_SIZE = 25
const KANBAN_PAGE_SIZE = 500

const activeStatuses: WoStatus[] = [
  "received",
  "diagnosing",
  "quoted",
  "approved",
  "in_repair",
  "waiting_parts",
  "ready",
  "delivered",
]

const closedStatuses: WoStatus[] = ["rejected", "cancelled"]

const statusOptions: Array<{ value: WoStatus | ""; label: string }> = [
  { value: "", label: "Todos los estados" },
  { value: "received", label: woStatusLabels.received },
  { value: "diagnosing", label: woStatusLabels.diagnosing },
  { value: "quoted", label: woStatusLabels.quoted },
  { value: "approved", label: woStatusLabels.approved },
  { value: "rejected", label: woStatusLabels.rejected },
  { value: "in_repair", label: woStatusLabels.in_repair },
  { value: "waiting_parts", label: woStatusLabels.waiting_parts },
  { value: "ready", label: woStatusLabels.ready },
  { value: "delivered", label: woStatusLabels.delivered },
  { value: "cancelled", label: woStatusLabels.cancelled },
]

export function WorkOrdersListPage() {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const q = params.get("q") ?? ""
  const status = (params.get("status") ?? "") as WoStatus | ""
  const clientUcode = params.get("client_ucode") ?? ""
  const page = Number(params.get("page") ?? "1") || 1
  const [draftQ, setDraftQ] = useState(q)
  const debouncedQ = useDebounce(draftQ, 250)

  useEffect(() => {
    setDraftQ(q)
  }, [q])

  useEffect(() => {
    if (debouncedQ !== q) updateParams(params, setParams, { q: debouncedQ, page: 1 })
  }, [debouncedQ, params, q, setParams])

  const queryParams = useMemo(
    () => ({
      q,
      status,
      client_ucode: clientUcode || undefined,
      page,
      page_size: LIST_PAGE_SIZE,
    }),
    [clientUcode, page, q, status]
  )

  const kanbanQueryParams = useMemo(
    () => ({
      q,
      status,
      client_ucode: clientUcode || undefined,
      page: 1,
      page_size: KANBAN_PAGE_SIZE,
    }),
    [clientUcode, q, status]
  )

  const list = useWorkOrders(queryParams)
  const kanban = useWorkOrders(kanbanQueryParams)

  const columns = useMemo<Column<WorkOrder>[]>(
    () => [
      { header: "Nº", cell: (row) => <span className="font-medium">{row.wo_number}</span> },
      { header: "Cliente", cell: (row) => row.client.name },
      { header: "Dispositivo", cell: (row) => deviceName(row) },
      { header: "Estado", cell: (row) => <WoStatusBadge status={row.status} /> },
      { header: "Recibido", cell: (row) => formatDate(row.received_ts) },
    ],
    []
  )

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Órdenes de trabajo</h1>
          <p className="text-sm text-muted-foreground">Seguimiento de ingresos, diagnósticos y reparaciones.</p>
        </div>
        <Button asChild>
          <Link to="/work-orders/new">
            <Plus />
            Nueva orden
          </Link>
        </Button>
      </div>

      <div className="grid gap-3 lg:grid-cols-[minmax(240px,1fr)_220px_minmax(240px,1fr)]">
        <div className="relative">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={draftQ}
            onChange={(event) => setDraftQ(event.target.value)}
            placeholder="Buscar por Nº, cliente, teléfono o serie"
            className="pl-9"
          />
        </div>
        <select
          className="border-input bg-background h-9 rounded-md border px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          value={status}
          onChange={(event) =>
            updateParams(params, setParams, { status: event.target.value as WoStatus | "", page: 1 })
          }
        >
          {statusOptions.map((option) => (
            <option key={option.value || "all"} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <EntityCombobox
          queryKey={["work-order-filter-clients"]}
          value={clientUcode || null}
          onChange={(value) => updateParams(params, setParams, { client_ucode: value ?? "", page: 1 })}
          fetchOptions={(q) => searchClients(q, 1, 20).then((data) => data.clients)}
          getKey={(client) => client.ucode}
          getLabel={clientLabel}
          placeholder="Filtrar por cliente"
          emptyMessage="Sin clientes"
        />
      </div>

      <Tabs defaultValue="kanban">
        <TabsList>
          <TabsTrigger value="kanban">Kanban</TabsTrigger>
          <TabsTrigger value="list">Lista</TabsTrigger>
        </TabsList>
        <TabsContent value="kanban">
          <KanbanBoard
            workOrders={kanban.data?.work_orders ?? []}
            isLoading={kanban.isLoading}
            onOpen={(wo) => navigate(`/work-orders/${wo.ucode}`)}
          />
        </TabsContent>
        <TabsContent value="list">
          <DataTable
            columns={columns}
            rows={list.data?.work_orders ?? []}
            rowKey={(row) => row.ucode}
            onRowClick={(row) => navigate(`/work-orders/${row.ucode}`)}
            isLoading={list.isLoading}
            emptyMessage="No hay órdenes de trabajo"
            page={list.data?.page ?? page}
            pageSize={list.data?.page_size ?? LIST_PAGE_SIZE}
            total={list.data?.total ?? 0}
            onPageChange={(nextPage) => updateParams(params, setParams, { page: nextPage })}
            searchValue={q}
            onSearchChange={(nextQ) => updateParams(params, setParams, { q: nextQ, page: 1 })}
            showSearch={false}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function KanbanBoard({
  workOrders,
  isLoading,
  onOpen,
}: {
  workOrders: WorkOrder[]
  isLoading: boolean
  onOpen: (wo: WorkOrder) => void
}) {
  const buckets = useMemo(() => bucketWorkOrders(workOrders), [workOrders])
  const columns = [
    ...activeStatuses.map((status) => ({
      key: status,
      label: woStatusLabels[status],
      rows: buckets[status],
    })),
    {
      key: "closed",
      label: "Cerradas",
      rows: closedStatuses.flatMap((status) => buckets[status]),
    },
  ]

  if (isLoading) {
    return <div className="rounded-md border p-6 text-sm text-muted-foreground">Cargando órdenes...</div>
  }

  return (
    <div className="grid gap-3 overflow-x-auto pb-2 xl:grid-cols-9">
      {columns.map((column) => (
        <section key={column.key} className="min-w-64 space-y-3">
          <div className="flex h-9 items-center justify-between rounded-md border bg-background px-3 text-sm font-medium">
            <span>{column.label}</span>
            <span className="text-muted-foreground">{column.rows.length}</span>
          </div>
          <div className="space-y-2">
            {column.rows.length > 0 ? (
              column.rows.map((wo) => <WorkOrderCard key={wo.ucode} workOrder={wo} onOpen={onOpen} />)
            ) : (
              <div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">Sin órdenes</div>
            )}
          </div>
        </section>
      ))}
    </div>
  )
}

function WorkOrderCard({ workOrder, onOpen }: { workOrder: WorkOrder; onOpen: (wo: WorkOrder) => void }) {
  return (
    <Card
      role="button"
      tabIndex={0}
      className="cursor-pointer transition-colors hover:bg-accent/50"
      onClick={() => onOpen(workOrder)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") onOpen(workOrder)
      }}
    >
      <CardHeader className="space-y-1 p-3">
        <CardTitle className="text-sm">{workOrder.wo_number}</CardTitle>
        <p className="truncate text-sm text-muted-foreground">{workOrder.client.name}</p>
      </CardHeader>
      <CardContent className="space-y-2 p-3 pt-0 text-xs text-muted-foreground">
        <div className="truncate">{deviceName(workOrder)}</div>
        <div>{timeAgo(workOrder.received_ts)}</div>
      </CardContent>
    </Card>
  )
}

function bucketWorkOrders(workOrders: WorkOrder[]) {
  const buckets = Object.fromEntries(
    [...activeStatuses, ...closedStatuses].map((status) => [status, [] as WorkOrder[]])
  ) as Record<WoStatus, WorkOrder[]>
  for (const wo of workOrders) buckets[wo.status].push(wo)
  return buckets
}

function updateParams(
  current: URLSearchParams,
  setParams: (params: URLSearchParams) => void,
  next: { q?: string; status?: WoStatus | ""; client_ucode?: string; page?: number }
) {
  const out = new URLSearchParams(current)
  if (next.q !== undefined) {
    if (next.q) out.set("q", next.q)
    else out.delete("q")
  }
  if (next.status !== undefined) {
    if (next.status) out.set("status", next.status)
    else out.delete("status")
  }
  if (next.client_ucode !== undefined) {
    if (next.client_ucode) out.set("client_ucode", next.client_ucode)
    else out.delete("client_ucode")
  }
  if (next.page !== undefined) out.set("page", String(next.page))
  setParams(out)
}

function clientLabel(client: Client) {
  return client.phone ? `${client.name} · ${client.phone}` : client.name
}

function deviceName(wo: WorkOrder) {
  return [wo.device.brand_name, wo.device.model_name, wo.device.article_type_name].filter(Boolean).join(" · ")
}

function formatDate(value?: string) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("es-AR", { dateStyle: "short", timeStyle: "short" }).format(new Date(value))
}

function timeAgo(value: string) {
  const diffMs = Date.now() - new Date(value).getTime()
  const minutes = Math.max(1, Math.floor(diffMs / 60000))
  if (minutes < 60) return `Hace ${minutes} min`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `Hace ${hours} h`
  const days = Math.floor(hours / 24)
  return `Hace ${days} d`
}
