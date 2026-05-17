import { useMemo, useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import {
  createArticleType,
  createBrand,
  createDeviceModel,
  listArticleTypes,
  listBrands,
  listDeviceModelsByBrand,
  type DeviceModel,
  type Lookup,
} from "@/api/lookups"
import type { DeviceInput } from "@/api/devices"
import { EntityCombobox } from "@/components/entity-combobox"
import { FormDialog } from "@/components/form-dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

type DeviceFormProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  clientUcode: string
  onSubmit: (input: DeviceInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function DeviceForm({ open, onOpenChange, clientUcode, onSubmit, isSubmitting }: DeviceFormProps) {
  const qc = useQueryClient()
  const [brandUcode, setBrandUcode] = useState<string | null>(null)
  const [modelUcode, setModelUcode] = useState<string | null>(null)
  const [articleTypeUcode, setArticleTypeUcode] = useState<string | null>(null)
  const [newBrandName, setNewBrandName] = useState("")
  const [newModelName, setNewModelName] = useState("")
  const [newArticleTypeName, setNewArticleTypeName] = useState("")
  const [serialNumber, setSerialNumber] = useState("")
  const [color, setColor] = useState("")
  const [description, setDescription] = useState("")

  const modelFetcher = useMemo(
    () => async (q: string) => {
      if (!brandUcode) return []
      const rows = await listDeviceModelsByBrand(brandUcode)
      return filterByName(rows, q)
    },
    [brandUcode],
  )

  const addBrand = async () => {
    const name = newBrandName.trim()
    if (!name) return
    try {
      const brand = await createBrand(name)
      setBrandUcode(brand.ucode)
      setModelUcode(null)
      setNewBrandName("")
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "brands"] })
      toast.success("Marca creada")
    } catch {
      toast.error("No se pudo crear la marca")
    }
  }

  const addModel = async () => {
    const name = newModelName.trim()
    if (!name || !brandUcode) return
    try {
      const model = await createDeviceModel({ brand_ucode: brandUcode, name })
      setModelUcode(model.ucode)
      setNewModelName("")
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "device-models", brandUcode] })
      toast.success("Modelo creado")
    } catch {
      toast.error("No se pudo crear el modelo")
    }
  }

  const addArticleType = async () => {
    const name = newArticleTypeName.trim()
    if (!name) return
    try {
      const articleType = await createArticleType(name)
      setArticleTypeUcode(articleType.ucode)
      setNewArticleTypeName("")
      await qc.invalidateQueries({ queryKey: ["entity-combobox", "article-types"] })
      toast.success("Tipo de artículo creado")
    } catch {
      toast.error("No se pudo crear el tipo de artículo")
    }
  }

  const submit = async () => {
    if (!brandUcode || !articleTypeUcode) {
      toast.error("Completá marca y tipo de artículo")
      return
    }
    await onSubmit({
      client_ucode: clientUcode,
      brand_ucode: brandUcode,
      model_ucode: modelUcode ?? undefined,
      article_type_ucode: articleTypeUcode,
      serial_number: emptyToUndefined(serialNumber),
      color: emptyToUndefined(color),
      description: emptyToUndefined(description),
    })
    setBrandUcode(null)
    setModelUcode(null)
    setArticleTypeUcode(null)
    setNewBrandName("")
    setNewModelName("")
    setNewArticleTypeName("")
    setSerialNumber("")
    setColor("")
    setDescription("")
  }

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Nuevo dispositivo"
      onSubmit={submit}
      isSubmitting={isSubmitting}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Marca">
          <EntityCombobox
            queryKey={["brands"]}
            value={brandUcode}
            onChange={(value) => {
              setBrandUcode(value)
              setModelUcode(null)
            }}
            fetchOptions={(q) => listBrands().then((rows) => filterByName(rows, q))}
            getKey={(brand) => brand.ucode}
            getLabel={(brand) => brand.name}
            placeholder="Seleccionar marca"
          />
          <InlineCreate
            value={newBrandName}
            onChange={setNewBrandName}
            onCreate={addBrand}
            placeholder="Nueva marca"
          />
        </Field>
        <Field label="Modelo">
          <EntityCombobox
            queryKey={["device-models", brandUcode ?? "none"]}
            value={modelUcode}
            onChange={setModelUcode}
            fetchOptions={modelFetcher}
            getKey={(model: DeviceModel) => model.ucode}
            getLabel={(model: DeviceModel) => model.name}
            placeholder="Seleccionar modelo"
            emptyMessage={brandUcode ? "Sin modelos" : "Elegí una marca"}
          />
          <InlineCreate
            value={newModelName}
            onChange={setNewModelName}
            onCreate={addModel}
            placeholder="Nuevo modelo"
            disabled={!brandUcode}
          />
        </Field>
      </div>
      <Field label="Tipo de artículo">
        <EntityCombobox
          queryKey={["article-types"]}
          value={articleTypeUcode}
          onChange={setArticleTypeUcode}
          fetchOptions={(q) => listArticleTypes().then((rows) => filterByName(rows, q))}
          getKey={(type) => type.ucode}
          getLabel={(type) => type.name}
          placeholder="Seleccionar tipo"
        />
        <InlineCreate
          value={newArticleTypeName}
          onChange={setNewArticleTypeName}
          onCreate={addArticleType}
          placeholder="Nuevo tipo de artículo"
        />
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Número de serie">
          <Input value={serialNumber} onChange={(event) => setSerialNumber(event.target.value)} />
        </Field>
        <Field label="Color">
          <Input value={color} onChange={(event) => setColor(event.target.value)} />
        </Field>
      </div>
      <Field label="Descripción">
        <textarea
          className="border-input bg-background min-h-24 w-full rounded-md border px-3 py-2 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </Field>
    </FormDialog>
  )
}

function InlineCreate({
  value,
  onChange,
  onCreate,
  placeholder,
  disabled = false,
}: {
  value: string
  onChange: (value: string) => void
  onCreate: () => void | Promise<void>
  placeholder: string
  disabled?: boolean
}) {
  return (
    <div className="mt-2 flex gap-2">
      <Input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        disabled={disabled}
      />
      <Button type="button" variant="outline" onClick={() => void onCreate()} disabled={disabled || !value.trim()}>
        Agregar
      </Button>
    </div>
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

function filterByName<T extends Lookup>(rows: T[], q: string) {
  const needle = q.trim().toLowerCase()
  if (!needle) return rows
  return rows.filter((row) => row.name.toLowerCase().includes(needle))
}

function emptyToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}
