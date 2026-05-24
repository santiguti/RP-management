import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"

import type { Supplier, SupplierInput } from "@/api/suppliers"
import { FormDialog } from "@/components/form-dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

const schema = z.object({
  name: z.string().trim().min(1, "El nombre es requerido").max(200, "Máximo 200 caracteres"),
  phone: z.string().trim().max(32, "Máximo 32 caracteres").optional(),
  email: z.union([z.literal(""), z.string().email("Email inválido")]).optional(),
  notes: z.string().trim().max(2000, "Máximo 2000 caracteres").optional(),
})

type FormValues = z.infer<typeof schema>

type SupplierFormProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  supplier?: Supplier
  onSubmit: (input: SupplierInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function SupplierForm({
  open,
  onOpenChange,
  supplier,
  onSubmit,
  isSubmitting,
}: SupplierFormProps) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: valuesFromSupplier(supplier),
  })

  useEffect(() => {
    if (open) reset(valuesFromSupplier(supplier))
  }, [open, reset, supplier])

  const submit = handleSubmit(async (values) => {
    await onSubmit({
      name: values.name.trim(),
      phone: emptyToUndefined(values.phone),
      email: emptyToUndefined(values.email),
      notes: emptyToUndefined(values.notes),
    })
  })

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={supplier ? "Editar proveedor" : "Nuevo proveedor"}
      onSubmit={submit}
      isSubmitting={isSubmitting}
    >
      <Field label="Nombre" error={errors.name?.message}>
        <Input {...register("name")} autoFocus />
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Teléfono" error={errors.phone?.message}>
          <Input {...register("phone")} />
        </Field>
        <Field label="Email" error={errors.email?.message}>
          <Input type="email" {...register("email")} />
        </Field>
      </div>
      <Field label="Notas" error={errors.notes?.message}>
        <Textarea {...register("notes")} />
      </Field>
    </FormDialog>
  )
}

function Field({
  label,
  error,
  children,
}: {
  label: string
  error?: string
  children: React.ReactNode
}) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
    </div>
  )
}

function valuesFromSupplier(supplier?: Supplier): FormValues {
  return {
    name: supplier?.name ?? "",
    phone: supplier?.phone ?? "",
    email: supplier?.email ?? "",
    notes: supplier?.notes ?? "",
  }
}

function emptyToUndefined(value?: string) {
  const trimmed = value?.trim()
  return trimmed ? trimmed : undefined
}
