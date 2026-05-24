import { useEffect, useState, type ReactNode } from "react"

import type {
  RecurringExpense,
  RecurringExpenseCategory,
  RecurringExpenseInput,
} from "@/api/recurring-expenses"
import { listSuppliers, type Supplier } from "@/api/suppliers"
import type { PaymentMethod } from "@/api/transactions"
import { EntityCombobox } from "@/components/entity-combobox"
import { FormDialog } from "@/components/form-dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { categoryLabels, paymentMethodLabels, paymentMethods } from "@/lib/money"
import { toast } from "sonner"

const recurringCategories: RecurringExpenseCategory[] = [
  "rent",
  "utilities",
  "salary",
  "taxes",
  "supplies",
  "other_expense",
]

type RecurringExpenseFormProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  expense?: RecurringExpense
  onSubmit: (input: RecurringExpenseInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function RecurringExpenseForm({
  open,
  onOpenChange,
  expense,
  onSubmit,
  isSubmitting,
}: RecurringExpenseFormProps) {
  const [name, setName] = useState("")
  const [amount, setAmount] = useState("")
  const [dayOfMonth, setDayOfMonth] = useState("5")
  const [category, setCategory] = useState<RecurringExpenseCategory>("rent")
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>("transfer")
  const [supplierUcode, setSupplierUcode] = useState<string | null>(null)
  const [description, setDescription] = useState("")
  const [active, setActive] = useState(true)

  useEffect(() => {
    if (!open) return
    setName(expense?.name ?? "")
    setAmount(expense?.amount ?? "")
    setDayOfMonth(String(expense?.day_of_month ?? 5))
    setCategory(expense?.category ?? "rent")
    setPaymentMethod(expense?.payment_method ?? "transfer")
    setSupplierUcode(expense?.supplier?.ucode ?? null)
    setDescription(expense?.description ?? "")
    setActive(expense?.active ?? true)
  }, [expense, open])

  const submit = async () => {
    const parsedDay = Number(dayOfMonth)
    if (!name.trim()) {
      toast.error("El nombre es requerido")
      return
    }
    if (!/^\d+(\.\d{1,2})?$/.test(amount.trim())) {
      toast.error("Monto inválido")
      return
    }
    if (!Number.isInteger(parsedDay) || parsedDay < 1 || parsedDay > 28) {
      toast.error("El día del mes debe estar entre 1 y 28")
      return
    }
    await onSubmit({
      name: name.trim(),
      amount: amount.trim(),
      currency: "ARS",
      day_of_month: parsedDay,
      category,
      payment_method: paymentMethod,
      supplier_ucode: supplierUcode ?? undefined,
      description: emptyToUndefined(description),
      active,
    })
  }

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={expense ? "Editar gasto fijo" : "Nueva regla"}
      onSubmit={submit}
      isSubmitting={isSubmitting}
    >
      <Field label="Nombre">
        <Input value={name} onChange={(event) => setName(event.target.value)} autoFocus />
      </Field>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Monto">
          <Input value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="50000.00" />
        </Field>
        <Field label="Día del mes">
          <Input
            type="number"
            min={1}
            max={28}
            value={dayOfMonth}
            onChange={(event) => setDayOfMonth(event.target.value)}
          />
        </Field>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Categoría">
          <NativeSelect value={category} onChange={(value) => setCategory(value as RecurringExpenseCategory)}>
            {recurringCategories.map((item) => (
              <option key={item} value={item}>
                {categoryLabels[item]}
              </option>
            ))}
          </NativeSelect>
        </Field>
        <Field label="Método de pago">
          <NativeSelect value={paymentMethod} onChange={(value) => setPaymentMethod(value as PaymentMethod)}>
            {paymentMethods.map((method) => (
              <option key={method} value={method}>
                {paymentMethodLabels[method]}
              </option>
            ))}
          </NativeSelect>
        </Field>
      </div>

      <Field label="Proveedor">
        <EntityCombobox<Supplier>
          queryKey={["suppliers"]}
          value={supplierUcode}
          onChange={setSupplierUcode}
          fetchOptions={() => listSuppliers()}
          getKey={(option) => option.ucode}
          getLabel={(option) => option.name}
          placeholder="Proveedor opcional"
        />
      </Field>

      <Field label="Descripción">
        <Textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} />
      </Field>

      <label className="flex items-center justify-between rounded-md border bg-background px-3 py-2 text-sm">
        <span className="font-medium">Activo</span>
        <input
          type="checkbox"
          className="h-5 w-5 accent-primary"
          checked={active}
          onChange={(event) => setActive(event.target.checked)}
        />
      </label>
    </FormDialog>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
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
  children: ReactNode
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
