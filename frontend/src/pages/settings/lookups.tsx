import { useMemo, useState } from "react"
import { Pencil, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import { listBrands, type Brand, type DeviceModel, type Lookup } from "@/api/lookups"
import { DataTable, type Column } from "@/components/data-table"
import { EntityCombobox } from "@/components/entity-combobox"
import { FormDialog } from "@/components/form-dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  useArticleTypes,
  useBrands,
  useCreateArticleType,
  useCreateBrand,
  useCreateDeviceModel,
  useDeleteArticleType,
  useDeleteBrand,
  useDeleteDeviceModel,
  useDeviceModels,
  useUpdateArticleType,
  useUpdateBrand,
  useUpdateDeviceModel,
} from "@/hooks/use-lookups"

export function LookupsPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-normal">Ajustes · Catálogos</h1>
        <p className="text-sm text-muted-foreground">Mantené marcas, modelos y tipos de artículo.</p>
      </div>

      <Tabs defaultValue="brands">
        <TabsList>
          <TabsTrigger value="brands">Marcas</TabsTrigger>
          <TabsTrigger value="models">Modelos</TabsTrigger>
          <TabsTrigger value="types">Tipos de artículo</TabsTrigger>
        </TabsList>
        <TabsContent value="brands" className="mt-4">
          <BrandsPane />
        </TabsContent>
        <TabsContent value="models" className="mt-4">
          <ModelsPane />
        </TabsContent>
        <TabsContent value="types" className="mt-4">
          <ArticleTypesPane />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function BrandsPane() {
  const brands = useBrands()
  const createBrand = useCreateBrand()
  const updateBrand = useUpdateBrand()
  const deleteBrand = useDeleteBrand()
  const crud = useLookupCrud<Brand>({
    createTitle: "Nueva marca",
    editTitle: "Editar marca",
    deleteTitle: "¿Eliminar marca?",
    deleteBody: "La marca no aparecerá en listados nuevos.",
    createLabel: "Nueva marca",
    successCreate: "Marca creada",
    successUpdate: "Marca actualizada",
    successDelete: "Marca eliminada",
    create: (name) => createBrand.mutateAsync(name),
    update: (item, name) => updateBrand.mutateAsync({ ucode: item.ucode, name }),
    remove: (item) => deleteBrand.mutateAsync(item.ucode),
    isSubmitting: createBrand.isPending || updateBrand.isPending || deleteBrand.isPending,
  })

  const columns = useLookupColumns(crud.openEdit, crud.openDelete)

  return (
    <>
      <LookupToolbar label="Nueva marca" onClick={crud.openCreate} />
      <LookupTable rows={brands.data ?? []} columns={columns} isLoading={brands.isLoading} emptyMessage="Sin marcas" />
      {crud.dialogs}
    </>
  )
}

function ModelsPane() {
  const [brandUcode, setBrandUcode] = useState<string | null>(null)
  const models = useDeviceModels(brandUcode)
  const createModel = useCreateDeviceModel(brandUcode)
  const updateModel = useUpdateDeviceModel(brandUcode)
  const deleteModel = useDeleteDeviceModel(brandUcode)
  const crud = useLookupCrud<DeviceModel>({
    createTitle: "Nuevo modelo",
    editTitle: "Editar modelo",
    deleteTitle: "¿Eliminar modelo?",
    deleteBody: "El modelo no aparecerá en listados nuevos.",
    createLabel: "Nuevo modelo",
    successCreate: "Modelo creado",
    successUpdate: "Modelo actualizado",
    successDelete: "Modelo eliminado",
    create: async (name) => {
      if (!brandUcode) throw new Error("missing brand")
      return createModel.mutateAsync({ brand_ucode: brandUcode, name })
    },
    update: (item, name) => updateModel.mutateAsync({ ucode: item.ucode, name }),
    remove: (item) => deleteModel.mutateAsync(item.ucode),
    isSubmitting: createModel.isPending || updateModel.isPending || deleteModel.isPending,
  })
  const columns = useLookupColumns(crud.openEdit, crud.openDelete)

  return (
    <div className="space-y-4">
      <div className="max-w-sm space-y-2">
        <Label>Marca</Label>
        <EntityCombobox
          queryKey={["brands"]}
          value={brandUcode}
          onChange={setBrandUcode}
          fetchOptions={(q) => listBrands().then((rows) => filterByName(rows, q))}
          getKey={(brand) => brand.ucode}
          getLabel={(brand) => brand.name}
          placeholder="Seleccionar marca"
        />
      </div>
      <LookupToolbar label="Nuevo modelo" onClick={crud.openCreate} disabled={!brandUcode} />
      <LookupTable
        rows={brandUcode ? models.data ?? [] : []}
        columns={columns}
        isLoading={models.isLoading}
        emptyMessage={brandUcode ? "Sin modelos" : "Seleccioná una marca"}
      />
      {crud.dialogs}
    </div>
  )
}

function ArticleTypesPane() {
  const articleTypes = useArticleTypes()
  const createArticleType = useCreateArticleType()
  const updateArticleType = useUpdateArticleType()
  const deleteArticleType = useDeleteArticleType()
  const crud = useLookupCrud<Lookup>({
    createTitle: "Nuevo tipo",
    editTitle: "Editar tipo",
    deleteTitle: "¿Eliminar tipo de artículo?",
    deleteBody: "El tipo de artículo no aparecerá en listados nuevos.",
    createLabel: "Nuevo tipo",
    successCreate: "Tipo creado",
    successUpdate: "Tipo actualizado",
    successDelete: "Tipo eliminado",
    create: (name) => createArticleType.mutateAsync(name),
    update: (item, name) => updateArticleType.mutateAsync({ ucode: item.ucode, name }),
    remove: (item) => deleteArticleType.mutateAsync(item.ucode),
    isSubmitting: createArticleType.isPending || updateArticleType.isPending || deleteArticleType.isPending,
  })
  const columns = useLookupColumns(crud.openEdit, crud.openDelete)

  return (
    <>
      <LookupToolbar label="Nuevo tipo" onClick={crud.openCreate} />
      <LookupTable
        rows={articleTypes.data ?? []}
        columns={columns}
        isLoading={articleTypes.isLoading}
        emptyMessage="Sin tipos de artículo"
      />
      {crud.dialogs}
    </>
  )
}

function useLookupColumns<T extends Lookup>(
  onEdit: (item: T) => void,
  onDelete: (item: T) => void,
) {
  return useMemo<Column<T>[]>(
    () => [
      { header: "Nombre", cell: (row) => <span className="font-medium">{row.name}</span> },
      {
        header: "",
        className: "w-32 text-right",
        cell: (row) => (
          <div className="flex justify-end gap-1">
            <Button type="button" variant="ghost" size="icon" onClick={() => onEdit(row)} aria-label="Editar">
              <Pencil />
            </Button>
            <Button type="button" variant="ghost" size="icon" onClick={() => onDelete(row)} aria-label="Eliminar">
              <Trash2 />
            </Button>
          </div>
        ),
      },
    ],
    [onDelete, onEdit],
  )
}

function LookupTable<T extends Lookup>({
  rows,
  columns,
  isLoading,
  emptyMessage,
}: {
  rows: T[]
  columns: Column<T>[]
  isLoading?: boolean
  emptyMessage: string
}) {
  const [q, setQ] = useState("")
  const filteredRows = useMemo(() => filterByName(rows, q), [q, rows])

  return (
    <DataTable
      columns={columns}
      rows={filteredRows}
      rowKey={(row) => row.ucode}
      isLoading={isLoading}
      emptyMessage={emptyMessage}
      page={1}
      pageSize={filteredRows.length || 25}
      total={filteredRows.length}
      onPageChange={() => undefined}
      searchValue={q}
      onSearchChange={setQ}
      searchPlaceholder="Buscar por nombre"
    />
  )
}

function LookupToolbar({ label, onClick, disabled = false }: { label: string; onClick: () => void; disabled?: boolean }) {
  return (
    <div className="mb-4 flex justify-end">
      <Button type="button" onClick={onClick} disabled={disabled}>
        <Plus />
        {label}
      </Button>
    </div>
  )
}

function useLookupCrud<T extends Lookup>({
  createTitle,
  editTitle,
  deleteTitle,
  deleteBody,
  createLabel,
  successCreate,
  successUpdate,
  successDelete,
  create,
  update,
  remove,
  isSubmitting,
}: {
  createTitle: string
  editTitle: string
  deleteTitle: string
  deleteBody: string
  createLabel: string
  successCreate: string
  successUpdate: string
  successDelete: string
  create: (name: string) => Promise<unknown>
  update: (item: T, name: string) => Promise<unknown>
  remove: (item: T) => Promise<unknown>
  isSubmitting: boolean
}) {
  const [mode, setMode] = useState<"create" | "edit" | null>(null)
  const [name, setName] = useState("")
  const [selected, setSelected] = useState<T | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)

  const openCreate = () => {
    setSelected(null)
    setName("")
    setMode("create")
  }
  const openEdit = (item: T) => {
    setSelected(item)
    setName(item.name)
    setMode("edit")
  }
  const openDelete = (item: T) => {
    setSelected(item)
    setDeleteOpen(true)
  }

  const submit = async () => {
    const trimmed = name.trim()
    if (!trimmed) {
      toast.error("El nombre es requerido")
      return
    }
    try {
      if (mode === "edit" && selected) {
        await update(selected, trimmed)
        toast.success(successUpdate)
      } else {
        await create(trimmed)
        toast.success(successCreate)
      }
      setMode(null)
      setName("")
    } catch {
      toast.error("No se pudo guardar")
    }
  }

  const confirmDelete = async () => {
    if (!selected) return
    try {
      await remove(selected)
      toast.success(successDelete)
      setDeleteOpen(false)
      setSelected(null)
    } catch {
      toast.error("No se pudo eliminar")
    }
  }

  return {
    openCreate,
    openEdit,
    openDelete,
    dialogs: (
      <>
        <FormDialog
          open={mode !== null}
          onOpenChange={(open) => {
            if (!open) setMode(null)
          }}
          title={mode === "edit" ? editTitle : createTitle}
          submitLabel="Guardar"
          onSubmit={submit}
          isSubmitting={isSubmitting}
        >
          <div className="space-y-2">
            <Label>Nombre</Label>
            <Input value={name} onChange={(event) => setName(event.target.value)} autoFocus />
          </div>
        </FormDialog>
        <FormDialog
          open={deleteOpen}
          onOpenChange={setDeleteOpen}
          title={deleteTitle}
          submitLabel="Eliminar"
          onSubmit={confirmDelete}
          isSubmitting={isSubmitting}
        >
          <p className="text-sm text-muted-foreground">{deleteBody}</p>
          {selected ? <p className="text-sm font-medium">{selected.name}</p> : null}
        </FormDialog>
      </>
    ),
    createLabel,
  }
}

function filterByName<T extends Lookup>(rows: T[], q: string) {
  const needle = q.trim().toLowerCase()
  if (!needle) return rows
  return rows.filter((row) => row.name.toLowerCase().includes(needle))
}
