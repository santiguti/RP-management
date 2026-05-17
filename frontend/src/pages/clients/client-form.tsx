import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"

import type { Client, ClientInput } from "@/api/clients"
import { FormDialog } from "@/components/form-dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

const schema = z.object({
  name: z.string().trim().min(1, "El nombre es requerido").max(200, "Máximo 200 caracteres"),
  phone: z.string().trim().max(32, "Máximo 32 caracteres").optional(),
  email: z.union([z.literal(""), z.string().email("Email inválido")]).optional(),
  dni_cuit: z.string().trim().max(32, "Máximo 32 caracteres").optional(),
  address: z.string().trim().max(400, "Máximo 400 caracteres").optional(),
  notes: z.string().trim().max(2000, "Máximo 2000 caracteres").optional(),
  client_type: z.enum(["particular", "empresa"]),
})

type FormValues = z.infer<typeof schema>

type ClientFormProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  client?: Client
  onSubmit: (input: ClientInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function ClientForm({ open, onOpenChange, client, onSubmit, isSubmitting }: ClientFormProps) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: valuesFromClient(client),
  })

  useEffect(() => {
    if (open) reset(valuesFromClient(client))
  }, [client, open, reset])

  const submit = handleSubmit(async (values) => {
    await onSubmit({
      name: values.name.trim(),
      phone: emptyToUndefined(values.phone),
      email: emptyToUndefined(values.email),
      dni_cuit: emptyToUndefined(values.dni_cuit),
      address: emptyToUndefined(values.address),
      notes: emptyToUndefined(values.notes),
      client_type: values.client_type,
    })
  })

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={client ? "Editar cliente" : "Nuevo cliente"}
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
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="DNI/CUIT" error={errors.dni_cuit?.message}>
          <Input {...register("dni_cuit")} />
        </Field>
        <Field label="Tipo" error={errors.client_type?.message}>
          <select
            className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
            {...register("client_type")}
          >
            <option value="particular">Particular</option>
            <option value="empresa">Empresa</option>
          </select>
        </Field>
      </div>
      <Field label="Dirección" error={errors.address?.message}>
        <Input {...register("address")} />
      </Field>
      <Field label="Notas" error={errors.notes?.message}>
        <textarea
          className="border-input bg-background min-h-24 w-full rounded-md border px-3 py-2 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          {...register("notes")}
        />
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

function valuesFromClient(client?: Client): FormValues {
  return {
    name: client?.name ?? "",
    phone: client?.phone ?? "",
    email: client?.email ?? "",
    dni_cuit: client?.dni_cuit ?? "",
    address: client?.address ?? "",
    notes: client?.notes ?? "",
    client_type: client?.client_type ?? "particular",
  }
}

function emptyToUndefined(value?: string) {
  const trimmed = value?.trim()
  return trimmed ? trimmed : undefined
}
