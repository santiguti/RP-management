import { useEffect, useMemo, useState } from "react"
import { AxiosError } from "axios"
import { Link } from "react-router-dom"
import { Plus, X } from "lucide-react"
import { toast } from "sonner"

import { searchParts, type Part } from "@/api/parts"
import type { WorkOrder } from "@/api/work-orders"
import type { WorkOrderPartInput } from "@/api/wo-parts"
import { DataTable, type Column } from "@/components/data-table"
import { EntityCombobox } from "@/components/entity-combobox"
import { FormDialog } from "@/components/form-dialog"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { usePart } from "@/hooks/use-parts"
import { useAddWorkOrderPart, useRemoveWorkOrderPart, useWorkOrderParts } from "@/hooks/use-wo-parts"
import { formatARSValue } from "@/lib/money"

export function WorkOrderPartsTab({ workOrder }: { workOrder: WorkOrder }) {
  const parts = useWorkOrderParts(workOrder.ucode)
  const addPart = useAddWorkOrderPart()
  const removePart = useRemoveWorkOrderPart()
  const [formOpen, setFormOpen] = useState(false)
  const [partUcode, setPartUcode] = useState<string | null>(null)
  const [quantity, setQuantity] = useState("")
  const [unitPrice, setUnitPrice] = useState("")
  const selectedPart = usePart(partUcode ?? undefined)

  useEffect(() => {
    if (selectedPart.data?.ucode) setUnitPrice(selectedPart.data.default_sale_price ?? "")
  }, [selectedPart.data?.ucode, selectedPart.data?.default_sale_price])

  const columns = useMemo<Column<NonNullable<typeof parts.data>[number]>[]>(
    () => [
      {
        header: "Repuesto",
        cell: (row) => (
          <Link className="font-medium text-primary underline-offset-4 hover:underline" to={`/parts/${row.part_ucode}`}>
            {row.part_name}
          </Link>
        ),
      },
      { header: "Cantidad", className: "text-right", cell: (row) => `${row.quantity} ${row.part_unit}` },
      { header: "Precio unitario", className: "text-right", cell: (row) => formatARSValue(row.unit_price_charged) },
      { header: "Subtotal", className: "text-right", cell: (row) => formatARSValue(row.subtotal) },
      {
        header: "",
        className: "w-12 text-right",
        cell: (row) => (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            title="Quitar repuesto"
            onClick={(event) => {
              event.stopPropagation()
              void onRemove(row.id)
            }}
          >
            <X />
          </Button>
        ),
      },
    ],
    [parts.data],
  )

  const resetForm = () => {
    setPartUcode(null)
    setQuantity("")
    setUnitPrice("")
  }

  const onAdd = async () => {
    if (!partUcode || !isPositiveDecimal(quantity) || !isNonNegativeDecimal(unitPrice)) {
      toast.error("Completá repuesto, cantidad y precio unitario")
      return
    }
    const input: WorkOrderPartInput = {
      part_ucode: partUcode,
      quantity: quantity.trim(),
      unit_price_charged: unitPrice.trim(),
    }
    try {
      await addPart.mutateAsync({ workOrderUcode: workOrder.ucode, input })
      toast.success("Repuesto agregado")
      setFormOpen(false)
      resetForm()
    } catch (error) {
      const code = error instanceof AxiosError ? (error.response?.data as { error?: string } | undefined)?.error : undefined
      toast.error(code === "insufficient_stock" ? "Stock insuficiente" : "No se pudo agregar el repuesto")
    }
  }

  const onRemove = async (id: number) => {
    if (!window.confirm("¿Quitar repuesto de la OT?")) return
    try {
      await removePart.mutateAsync({ workOrderUcode: workOrder.ucode, id })
      toast.success("Repuesto quitado")
    } catch {
      toast.error("No se pudo quitar el repuesto")
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Repuestos</CardTitle>
        <CardAction>
          <Button type="button" size="sm" onClick={() => setFormOpen(true)}>
            <Plus />
            Agregar repuesto
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <DataTable
          columns={columns}
          rows={parts.data ?? []}
          rowKey={(row) => String(row.id)}
          isLoading={parts.isLoading}
          emptyMessage="Sin repuestos cargados"
          page={1}
          pageSize={Math.max(parts.data?.length ?? 0, 1)}
          total={parts.data?.length ?? 0}
          onPageChange={() => undefined}
          searchValue=""
          onSearchChange={() => undefined}
          showSearch={false}
        />
      </CardContent>
      <CardFooter className="justify-end border-t text-sm font-semibold">
        Total repuestos: {formatARSValue(workOrder.parts_amount ?? "0")}
      </CardFooter>

      <FormDialog
        open={formOpen}
        onOpenChange={(open) => {
          setFormOpen(open)
          if (!open) resetForm()
        }}
        title="Agregar repuesto"
        onSubmit={onAdd}
        isSubmitting={addPart.isPending}
      >
        <Field label="Repuesto">
          <EntityCombobox<Part>
            queryKey={["parts"]}
            value={partUcode}
            onChange={setPartUcode}
            fetchOptions={(q) => searchParts({ q, page: 1, page_size: 20 }).then((data) => data.parts)}
            getKey={(part) => part.ucode}
            getLabel={(part) => part.sku ? `${part.name} · ${part.sku}` : part.name}
            placeholder="Buscar repuesto"
          />
        </Field>
        {selectedPart.data ? (
          <div className="text-sm text-muted-foreground">Stock disponible: {selectedPart.data.current_stock} {selectedPart.data.unit}</div>
        ) : null}
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Cantidad">
            <Input inputMode="decimal" value={quantity} onChange={(event) => setQuantity(event.target.value)} />
          </Field>
          <Field label="Precio unitario">
            <Input inputMode="decimal" value={unitPrice} onChange={(event) => setUnitPrice(event.target.value)} />
          </Field>
        </div>
      </FormDialog>
    </Card>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  )
}

function isPositiveDecimal(value: string) {
  return /^\d+(\.\d{1,2})?$/.test(value.trim()) && Number(value) > 0
}

function isNonNegativeDecimal(value: string) {
  return /^\d+(\.\d{1,2})?$/.test(value.trim()) && Number(value) >= 0
}
