import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"

import type { Part, PartInput } from "@/api/parts"
import { FormDialog } from "@/components/form-dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

const decimal = z.string().trim().regex(/^\d+(\.\d{1,2})?$/, "Ingresá un número válido")
const optionalDecimal = z.union([z.literal(""), decimal]).optional()

const schema = z.object({
  sku: z.string().trim().max(64, "Máximo 64 caracteres").optional(),
  name: z.string().trim().min(1, "El nombre es requerido").max(200, "Máximo 200 caracteres"),
  description: z.string().trim().max(4000, "Máximo 4000 caracteres").optional(),
  unit: z.string().trim().min(1, "La unidad es requerida").max(32, "Máximo 32 caracteres"),
  reorder_level: optionalDecimal,
  default_cost: optionalDecimal,
  default_sale_price: optionalDecimal,
})

type FormValues = z.infer<typeof schema>

type PartFormProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  part?: Part
  onSubmit: (input: PartInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function PartForm({ open, onOpenChange, part, onSubmit, isSubmitting }: PartFormProps) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: valuesFromPart(part),
  })

  useEffect(() => {
    if (open) reset(valuesFromPart(part))
  }, [open, part, reset])

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={part ? "Editar repuesto" : "Nuevo repuesto"}
      onSubmit={handleSubmit(async (values) => {
        await onSubmit({
          sku: emptyToUndefined(values.sku),
          name: values.name.trim(),
          description: emptyToUndefined(values.description),
          unit: values.unit.trim(),
          reorder_level: emptyToUndefined(values.reorder_level),
          default_cost: emptyToUndefined(values.default_cost),
          default_sale_price: emptyToUndefined(values.default_sale_price),
        })
      })}
      isSubmitting={isSubmitting}
    >
      <div className="grid gap-4 sm:grid-cols-[1fr_10rem]">
        <Field label="Nombre" error={errors.name?.message}>
          <Input {...register("name")} autoFocus />
        </Field>
        <Field label="SKU" error={errors.sku?.message}>
          <Input {...register("sku")} />
        </Field>
      </div>
      <Field label="Descripción" error={errors.description?.message}>
        <Textarea {...register("description")} />
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Unidad" error={errors.unit?.message}>
          <Input {...register("unit")} />
        </Field>
        <Field label="Punto de reposición" error={errors.reorder_level?.message}>
          <Input inputMode="decimal" placeholder="2.00" {...register("reorder_level")} />
        </Field>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Costo" error={errors.default_cost?.message}>
          <Input inputMode="decimal" placeholder="20000.00" {...register("default_cost")} />
        </Field>
        <Field label="Precio venta" error={errors.default_sale_price?.message}>
          <Input inputMode="decimal" placeholder="30000.00" {...register("default_sale_price")} />
        </Field>
      </div>
    </FormDialog>
  )
}

function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  )
}

function valuesFromPart(part?: Part): FormValues {
  return {
    sku: part?.sku ?? "",
    name: part?.name ?? "",
    description: part?.description ?? "",
    unit: part?.unit ?? "unidad",
    reorder_level: part?.reorder_level ?? "",
    default_cost: part?.default_cost ?? "",
    default_sale_price: part?.default_sale_price ?? "",
  }
}

function emptyToUndefined(value?: string) {
  const trimmed = value?.trim()
  return trimmed ? trimmed : undefined
}
