import { useEffect, useState } from "react"
import { toast } from "sonner"

import type { MovementInput, Part } from "@/api/parts"
import { listSuppliers, type Supplier } from "@/api/suppliers"
import type { PaymentMethod } from "@/api/transactions"
import { EntityCombobox } from "@/components/entity-combobox"
import { FormDialog } from "@/components/form-dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { paymentMethodLabels, paymentMethods } from "@/lib/money"

type MovementFormProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  part: Part
  onSubmit: (input: MovementInput) => void | Promise<void>
  isSubmitting?: boolean
}

type MovementType = MovementInput["movement_type"]

export function MovementForm({ open, onOpenChange, part, onSubmit, isSubmitting }: MovementFormProps) {
  const [movementType, setMovementType] = useState<MovementType>("purchase")
  const [quantity, setQuantity] = useState("")
  const [adjustmentOut, setAdjustmentOut] = useState(false)
  const [unitCost, setUnitCost] = useState("")
  const [supplierUcode, setSupplierUcode] = useState<string | null>(null)
  const [notes, setNotes] = useState("")
  const [linkTransaction, setLinkTransaction] = useState(false)
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>("cash")

  useEffect(() => {
    if (!open) return
    setMovementType("purchase")
    setQuantity("")
    setAdjustmentOut(false)
    setUnitCost("")
    setSupplierUcode(null)
    setNotes("")
    setLinkTransaction(false)
    setPaymentMethod("cash")
  }, [open])

  const submit = async () => {
    if (!/^\d+(\.\d{1,2})?$/.test(quantity.trim()) || Number(quantity) <= 0) {
      toast.error("Cantidad inválida")
      return
    }
    if (movementType === "purchase" && unitCost.trim() && !/^\d+(\.\d{1,2})?$/.test(unitCost.trim())) {
      toast.error("Costo unitario inválido")
      return
    }
    await onSubmit({
      movement_type: movementType,
      quantity: quantity.trim(),
      adjustment_out: movementType === "adjustment" ? adjustmentOut : undefined,
      unit_cost: movementType === "purchase" ? emptyToUndefined(unitCost) : undefined,
      payment_method: movementType === "purchase" && linkTransaction ? paymentMethod : undefined,
      supplier_ucode: movementType === "purchase" ? supplierUcode ?? undefined : undefined,
      notes: emptyToUndefined(notes),
      link_transaction: movementType === "purchase" && linkTransaction ? true : undefined,
    })
  }

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Registrar movimiento"
      onSubmit={submit}
      isSubmitting={isSubmitting}
      submitLabel="Registrar"
    >
      <Field label="Tipo">
        <div className="grid gap-2 sm:grid-cols-3" role="radiogroup">
          <Choice name="movement-type" value="purchase" label="Compra" selected={movementType === "purchase"} onChange={() => setMovementType("purchase")} />
          <Choice name="movement-type" value="adjustment" label="Ajuste" selected={movementType === "adjustment"} onChange={() => setMovementType("adjustment")} />
          <Choice name="movement-type" value="return" label="Devolución" selected={movementType === "return"} onChange={() => setMovementType("return")} />
        </div>
      </Field>
      <Field label="Cantidad">
        <div className="flex items-center gap-2">
          <Input inputMode="decimal" value={quantity} onChange={(event) => setQuantity(event.target.value)} autoFocus />
          <span className="min-w-20 text-sm text-muted-foreground">{part.unit}</span>
        </div>
      </Field>
      {movementType === "adjustment" ? (
        <Field label="Dirección">
          <div className="grid gap-2 sm:grid-cols-2" role="radiogroup">
            <Choice name="adjustment-direction" value="in" label="Entrada" selected={!adjustmentOut} onChange={() => setAdjustmentOut(false)} />
            <Choice name="adjustment-direction" value="out" label="Salida" selected={adjustmentOut} onChange={() => setAdjustmentOut(true)} />
          </div>
        </Field>
      ) : null}
      {movementType === "purchase" ? (
        <>
          <Field label="Costo unitario">
            <Input inputMode="decimal" value={unitCost} onChange={(event) => setUnitCost(event.target.value)} placeholder="20000.00" />
          </Field>
          <Field label="Proveedor">
            <EntityCombobox<Supplier>
              queryKey={["suppliers"]}
              value={supplierUcode}
              onChange={setSupplierUcode}
              fetchOptions={() => listSuppliers()}
              getKey={(supplier) => supplier.ucode}
              getLabel={(supplier) => supplier.name}
              placeholder="Seleccionar proveedor"
            />
          </Field>
          <label className="flex items-center gap-2 text-sm font-medium">
            <input
              type="checkbox"
              checked={linkTransaction}
              onChange={(event) => setLinkTransaction(event.target.checked)}
              className="size-4 accent-primary"
            />
            Registrar movimiento monetario también
          </label>
          {linkTransaction ? (
            <Field label="Método de pago">
              <NativeSelect value={paymentMethod} onChange={(value) => setPaymentMethod(value as PaymentMethod)}>
                {paymentMethods.map((method) => (
                  <option key={method} value={method}>
                    {paymentMethodLabels[method]}
                  </option>
                ))}
              </NativeSelect>
            </Field>
          ) : null}
        </>
      ) : null}
      <Field label="Notas">
        <Textarea value={notes} onChange={(event) => setNotes(event.target.value)} maxLength={2000} />
      </Field>
    </FormDialog>
  )
}

function Choice({
  name,
  value,
  label,
  selected,
  onChange,
}: {
  name: string
  value: string
  label: string
  selected: boolean
  onChange: () => void
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm has-[:checked]:border-primary has-[:checked]:bg-primary/5">
      <input type="radio" name={name} value={value} checked={selected} onChange={onChange} className="size-4 accent-primary" />
      {label}
    </label>
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

function NativeSelect({
  value,
  onChange,
  children,
}: {
  value: string
  onChange: (value: string) => void
  children: React.ReactNode
}) {
  return (
    <select
      className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
      value={value}
      onChange={(event) => onChange(event.target.value)}
    >
      {children}
    </select>
  )
}

function emptyToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}
