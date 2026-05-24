import { useMemo, useState } from "react"
import { AxiosError } from "axios"
import { Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import type { Supplier, SupplierInput } from "@/api/suppliers"
import { DataTable, type Column } from "@/components/data-table"
import { Button } from "@/components/ui/button"
import {
  useCreateSupplier,
  useDeleteSupplier,
  useSuppliers,
  useUpdateSupplier,
} from "@/hooks/use-suppliers"
import { formatDate } from "@/pages/clients/list"
import { SupplierForm } from "./supplier-form"

export function SuppliersListPage() {
  const suppliers = useSuppliers()
  const createSupplier = useCreateSupplier()
  const updateSupplier = useUpdateSupplier()
  const deleteSupplier = useDeleteSupplier()
  const [formOpen, setFormOpen] = useState(false)
  const [selected, setSelected] = useState<Supplier | undefined>()

  const rows = suppliers.data ?? []
  const columns = useMemo<Column<Supplier>[]>(
    () => [
      { header: "Nombre", cell: (row) => <span className="font-medium">{row.name}</span> },
      { header: "Teléfono", cell: (row) => row.phone ?? "-" },
      { header: "Email", cell: (row) => row.email ?? "-" },
      { header: "Creado", cell: (row) => formatDate(row.created_ts) },
      {
        header: "",
        className: "w-14 text-right",
        cell: (row) => (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={(event) => {
              event.stopPropagation()
              void onDelete(row)
            }}
          >
            <Trash2 />
          </Button>
        ),
      },
    ],
    [],
  )

  const onCreate = async (input: SupplierInput) => {
    try {
      await createSupplier.mutateAsync(input)
      toast.success("Proveedor creado")
      setFormOpen(false)
    } catch (error) {
      showSupplierError(error)
    }
  }

  const onUpdate = async (input: SupplierInput) => {
    if (!selected) return
    try {
      await updateSupplier.mutateAsync({ ucode: selected.ucode, input })
      toast.success("Proveedor actualizado")
      setSelected(undefined)
    } catch (error) {
      showSupplierError(error)
    }
  }

  const onDelete = async (supplier: Supplier) => {
    if (!window.confirm("¿Eliminar proveedor?")) return
    try {
      await deleteSupplier.mutateAsync(supplier.ucode)
      toast.success("Proveedor eliminado")
      if (selected?.ucode === supplier.ucode) setSelected(undefined)
    } catch {
      toast.error("No se pudo eliminar el proveedor")
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Proveedores</h1>
          <p className="text-sm text-muted-foreground">Contactos para compras, insumos y servicios.</p>
        </div>
        <Button
          type="button"
          onClick={() => {
            setSelected(undefined)
            setFormOpen(true)
          }}
        >
          <Plus />
          Nuevo proveedor
        </Button>
      </div>

      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(row) => row.ucode}
        onRowClick={(row) => setSelected(row)}
        isLoading={suppliers.isLoading}
        emptyMessage="No hay proveedores"
        page={1}
        pageSize={Math.max(rows.length, 1)}
        total={rows.length}
        onPageChange={() => undefined}
        searchValue=""
        onSearchChange={() => undefined}
        showSearch={false}
      />

      <SupplierForm
        open={formOpen}
        onOpenChange={setFormOpen}
        onSubmit={onCreate}
        isSubmitting={createSupplier.isPending}
      />
      <SupplierForm
        open={Boolean(selected)}
        onOpenChange={(open) => {
          if (!open) setSelected(undefined)
        }}
        supplier={selected}
        onSubmit={onUpdate}
        isSubmitting={updateSupplier.isPending}
      />
    </div>
  )
}

function showSupplierError(error: unknown) {
  if (error instanceof AxiosError) {
    const code = (error.response?.data as { error?: string } | undefined)?.error
    if (code === "already_exists") {
      toast.error("Ya existe un proveedor con ese nombre")
      return
    }
  }
  toast.error("No se pudo guardar el proveedor")
}
