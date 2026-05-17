import { useMemo, useState } from "react"
import { toast } from "sonner"

import { listArticleTypes, listBrands, listDeviceModelsByBrand, type DeviceModel, type Lookup } from "@/api/lookups"
import type { DeviceInput } from "@/api/devices"
import { EntityCombobox } from "@/components/entity-combobox"
import { FormDialog } from "@/components/form-dialog"
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
  const [brandUcode, setBrandUcode] = useState<string | null>(null)
  const [modelUcode, setModelUcode] = useState<string | null>(null)
  const [articleTypeUcode, setArticleTypeUcode] = useState<string | null>(null)
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
        </Field>
        <Field label="Modelo">
          <EntityCombobox
            value={modelUcode}
            onChange={setModelUcode}
            fetchOptions={modelFetcher}
            getKey={(model: DeviceModel) => model.ucode}
            getLabel={(model: DeviceModel) => model.name}
            placeholder="Seleccionar modelo"
          />
        </Field>
      </div>
      <Field label="Tipo de artículo">
        <EntityCombobox
          value={articleTypeUcode}
          onChange={setArticleTypeUcode}
          fetchOptions={(q) => listArticleTypes().then((rows) => filterByName(rows, q))}
          getKey={(type) => type.ucode}
          getLabel={(type) => type.name}
          placeholder="Seleccionar tipo"
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
