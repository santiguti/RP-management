import { useEffect, useMemo, useState } from "react"
import { AxiosError } from "axios"
import { Plus } from "lucide-react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { toast } from "sonner"

import type { Part, PartInput } from "@/api/parts"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DataTable, type Column } from "@/components/data-table"
import { useCreatePart, useParts } from "@/hooks/use-parts"
import { formatARSValue } from "@/lib/money"
import { PartForm } from "./part-form"

const PAGE_SIZE = 25

export function PartsListPage({ createOnMount = false }: { createOnMount?: boolean }) {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const [formOpen, setFormOpen] = useState(createOnMount)
  const q = params.get("q") ?? ""
  const page = Number(params.get("page") ?? "1") || 1
  const lowStock = params.get("low_stock") === "true"
  const parts = useParts({ q, low_stock: lowStock, page, page_size: PAGE_SIZE })
  const createPart = useCreatePart()

  useEffect(() => {
    if (createOnMount) setFormOpen(true)
  }, [createOnMount])

  const columns = useMemo<Column<Part>[]>(
    () => [
      { header: "Nombre", cell: (row) => <span className="font-medium">{row.name}</span> },
      { header: "SKU", cell: (row) => row.sku ?? "-" },
      {
        header: "Stock",
        className: "text-right",
        cell: (row) => (
          <span className="inline-flex items-center justify-end gap-2">
            {row.current_stock} {row.low_stock ? <Badge variant="destructive">Bajo</Badge> : null}
          </span>
        ),
      },
      { header: "Punto de reposición", className: "text-right", cell: (row) => row.reorder_level ?? "-" },
      {
        header: "Precio venta",
        className: "text-right",
        cell: (row) => (row.default_sale_price ? formatARSValue(row.default_sale_price) : "-"),
      },
    ],
    [],
  )

  const updateParams = (next: { q?: string; page?: number; low_stock?: boolean }) => {
    const out = new URLSearchParams(params)
    if (next.q !== undefined) {
      if (next.q) out.set("q", next.q)
      else out.delete("q")
      out.set("page", "1")
    }
    if (next.page !== undefined) out.set("page", String(next.page))
    if (next.low_stock !== undefined) {
      if (next.low_stock) out.set("low_stock", "true")
      else out.delete("low_stock")
      out.set("page", "1")
    }
    setParams(out)
  }

  const onCreate = async (input: PartInput) => {
    try {
      const part = await createPart.mutateAsync(input)
      toast.success("Repuesto creado")
      setFormOpen(false)
      navigate(`/parts/${part.ucode}`)
    } catch (error) {
      showPartError(error)
    }
  }

  const closeForm = (open: boolean) => {
    setFormOpen(open)
    if (!open && createOnMount) navigate("/parts", { replace: true })
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-2xl font-semibold tracking-normal">Repuestos</h1>
        <Button type="button" onClick={() => setFormOpen(true)}>
          <Plus />
          Nuevo repuesto
        </Button>
      </div>

      <label className="flex w-fit cursor-pointer items-center gap-2 text-sm font-medium">
        <input
          type="checkbox"
          role="switch"
          checked={lowStock}
          onChange={(event) => updateParams({ low_stock: event.target.checked })}
          className="size-4 accent-primary"
        />
        Solo bajo stock
      </label>

      <DataTable
        columns={columns}
        rows={parts.data?.parts ?? []}
        rowKey={(row) => row.ucode}
        onRowClick={(row) => navigate(`/parts/${row.ucode}`)}
        isLoading={parts.isLoading}
        emptyMessage="No hay repuestos cargados"
        page={parts.data?.page ?? page}
        pageSize={parts.data?.page_size ?? PAGE_SIZE}
        total={parts.data?.total ?? 0}
        onPageChange={(nextPage) => updateParams({ page: nextPage })}
        searchValue={q}
        onSearchChange={(nextQ) => updateParams({ q: nextQ })}
        searchPlaceholder="Buscar por nombre o SKU"
      />

      <PartForm open={formOpen} onOpenChange={closeForm} onSubmit={onCreate} isSubmitting={createPart.isPending} />
    </div>
  )
}

export function showPartError(error: unknown) {
  if (error instanceof AxiosError) {
    const code = (error.response?.data as { error?: string } | undefined)?.error
    if (code === "already_exists") {
      toast.error("Ya existe un repuesto con ese SKU")
      return
    }
  }
  toast.error("No se pudo guardar el repuesto")
}
