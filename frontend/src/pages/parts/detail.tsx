import { useMemo, useState } from "react"
import { AxiosError } from "axios"
import { Link, useNavigate, useParams } from "react-router-dom"
import { Pencil, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import type { Movement, MovementInput, PartInput } from "@/api/parts"
import { DataTable, type Column } from "@/components/data-table"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useCreateMovement, useDeletePart, usePart, usePartMovements, useUpdatePart } from "@/hooks/use-parts"
import { formatARSValue } from "@/lib/money"
import { formatDate } from "@/pages/clients/list"
import { MovementForm } from "./movement-form"
import { PartForm } from "./part-form"
import { showPartError } from "./list"

const PAGE_SIZE = 25

const movementLabels: Record<Movement["movement_type"], string> = {
  purchase: "Compra",
  use: "Uso",
  adjustment: "Ajuste",
  return: "Devolución",
}

export function PartDetailPage() {
  const { ucode } = useParams()
  const navigate = useNavigate()
  const part = usePart(ucode)
  const [movementPage, setMovementPage] = useState(1)
  const movements = usePartMovements(ucode, movementPage, PAGE_SIZE)
  const updatePart = useUpdatePart()
  const deletePart = useDeletePart()
  const createMovement = useCreateMovement()
  const [editOpen, setEditOpen] = useState(false)
  const [movementOpen, setMovementOpen] = useState(false)

  const columns = useMemo<Column<Movement>[]>(
    () => [
      { header: "Fecha", cell: (row) => formatDate(row.created_ts) },
      { header: "Tipo", cell: (row) => movementLabels[row.movement_type] },
      {
        header: "Cantidad",
        className: "text-right",
        cell: (row) => {
          const positive = Number(row.quantity) >= 0
          return <span className={positive ? "text-emerald-700" : "text-destructive"}>{positive ? "+" : ""}{row.quantity}</span>
        },
      },
      { header: "Costo unitario", className: "text-right", cell: (row) => row.unit_cost ? formatARSValue(row.unit_cost) : "-" },
      {
        header: "Origen",
        cell: (row) => row.supplier?.name ?? (row.work_order ? (
          <Link className="text-primary underline-offset-4 hover:underline" to={`/work-orders/${row.work_order.ucode}`} onClick={(event) => event.stopPropagation()}>
            OT {row.work_order.wo_number}
          </Link>
        ) : "-"),
      },
    ],
    [],
  )

  const onUpdate = async (input: PartInput) => {
    if (!ucode) return
    try {
      await updatePart.mutateAsync({ ucode, input })
      toast.success("Repuesto actualizado")
      setEditOpen(false)
    } catch (error) {
      showPartError(error)
    }
  }

  const onDelete = async () => {
    if (!ucode || !window.confirm("¿Eliminar repuesto?\nEl repuesto se ocultará pero el historial de movimientos se conserva.")) return
    try {
      await deletePart.mutateAsync(ucode)
      toast.success("Repuesto eliminado")
      navigate("/parts")
    } catch {
      toast.error("No se pudo eliminar el repuesto")
    }
  }

  const onCreateMovement = async (input: MovementInput) => {
    if (!ucode) return
    try {
      await createMovement.mutateAsync({ ucode, input })
      toast.success("Movimiento registrado")
      setMovementOpen(false)
    } catch (error) {
      const code = error instanceof AxiosError ? (error.response?.data as { error?: string } | undefined)?.error : undefined
      toast.error(code === "insufficient_stock" ? "Stock insuficiente" : "No se pudo registrar el movimiento")
    }
  }

  if (part.isLoading) return <div className="rounded-md border p-6 text-sm text-muted-foreground">Cargando repuesto...</div>
  if (!part.data) return <div className="rounded-md border p-6 text-sm text-muted-foreground">Repuesto no encontrado</div>

  const item = part.data
  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">{item.name}</h1>
          {item.sku ? <p className="mt-1 text-sm text-muted-foreground">SKU {item.sku}</p> : null}
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="outline" onClick={() => setEditOpen(true)}>
            <Pencil />
            Editar
          </Button>
          <Button type="button" variant="ghost" size="icon" title="Eliminar repuesto" onClick={() => void onDelete()}>
            <Trash2 />
          </Button>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Metric label="Stock actual" value={`${item.current_stock} ${item.unit}`} />
        <Metric label="Punto de reposición" value={item.reorder_level ? `${item.reorder_level} ${item.unit}` : "-"} />
        <Metric label="Costo" value={item.default_cost ? formatARSValue(item.default_cost) : "-"} />
        <Metric label="Precio sugerido" value={item.default_sale_price ? formatARSValue(item.default_sale_price) : "-"} />
      </div>

      <Tabs defaultValue="movements">
        <TabsList>
          <TabsTrigger value="movements">Movimientos</TabsTrigger>
        </TabsList>
        <TabsContent value="movements">
          <Card>
            <CardHeader>
              <CardTitle>Movimientos</CardTitle>
              <CardAction>
                <Button type="button" size="sm" onClick={() => setMovementOpen(true)}>
                  <Plus />
                  Registrar movimiento
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent>
              <DataTable
                columns={columns}
                rows={movements.data?.movements ?? []}
                rowKey={(row) => row.ucode}
                isLoading={movements.isLoading}
                emptyMessage="No hay movimientos cargados"
                page={movements.data?.page ?? movementPage}
                pageSize={movements.data?.page_size ?? PAGE_SIZE}
                total={movements.data?.total ?? 0}
                onPageChange={setMovementPage}
                searchValue=""
                onSearchChange={() => undefined}
                showSearch={false}
              />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <PartForm open={editOpen} onOpenChange={setEditOpen} part={item} onSubmit={onUpdate} isSubmitting={updatePart.isPending} />
      <MovementForm open={movementOpen} onOpenChange={setMovementOpen} part={item} onSubmit={onCreateMovement} isSubmitting={createMovement.isPending} />
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <Card className="gap-2 py-4 shadow-none">
      <CardContent className="space-y-1 px-4">
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className="font-semibold">{value}</p>
      </CardContent>
    </Card>
  )
}
