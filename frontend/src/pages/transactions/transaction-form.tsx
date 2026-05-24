import { useEffect, useState } from "react"
import { AxiosError } from "axios"
import { toast } from "sonner"

import { searchClients, type Client } from "@/api/clients"
import { listSuppliers, type Supplier } from "@/api/suppliers"
import {
  type CounterpartyType,
  type PaymentMethod,
  type Transaction,
  type TransactionCategory,
  type TransactionInput,
  type TransactionType,
  type UpdateTransactionInput,
} from "@/api/transactions"
import { searchWorkOrders, type WorkOrder } from "@/api/work-orders"
import { EntityCombobox } from "@/components/entity-combobox"
import { FormDialog } from "@/components/form-dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
  categoryLabels,
  expenseCategories,
  incomeCategories,
  paymentMethodLabels,
  paymentMethods,
} from "@/lib/money"

type TransactionFormProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  transaction?: Transaction
  defaults?: Partial<TransactionInput>
  onSubmit: (input: TransactionInput | UpdateTransactionInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function TransactionForm({
  open,
  onOpenChange,
  transaction,
  defaults,
  onSubmit,
  isSubmitting,
}: TransactionFormProps) {
  const isEdit = Boolean(transaction)
  const [transactionType, setTransactionType] = useState<TransactionType>("expense")
  const [amount, setAmount] = useState("")
  const [currency, setCurrency] = useState("ARS")
  const [transactionDate, setTransactionDate] = useState(todayISO())
  const [paymentMethod, setPaymentMethod] = useState<PaymentMethod>("transfer")
  const [category, setCategory] = useState<TransactionCategory>("rent")
  const [counterpartyType, setCounterpartyType] = useState<CounterpartyType>("none")
  const [clientUcode, setClientUcode] = useState<string | null>(null)
  const [supplierUcode, setSupplierUcode] = useState<string | null>(null)
  const [workOrderUcode, setWorkOrderUcode] = useState<string | null>(null)
  const [description, setDescription] = useState("")

  useEffect(() => {
    if (!open) return
    setTransactionType(transaction?.transaction_type ?? defaults?.transaction_type ?? "expense")
    setAmount(transaction?.amount ?? defaults?.amount ?? "")
    setCurrency(transaction?.currency ?? defaults?.currency ?? "ARS")
    setTransactionDate(transaction?.transaction_date ?? defaults?.transaction_date ?? todayISO())
    setPaymentMethod(transaction?.payment_method ?? defaults?.payment_method ?? "transfer")
    setCategory(transaction?.category ?? defaults?.category ?? "rent")
    setCounterpartyType(
      transaction?.counterparty_type ?? defaults?.counterparty_type ?? inferCounterparty(defaults?.category ?? "rent")
    )
    setClientUcode(transaction?.client?.ucode ?? defaults?.client_ucode ?? null)
    setSupplierUcode(transaction?.supplier?.ucode ?? defaults?.supplier_ucode ?? null)
    setWorkOrderUcode(transaction?.work_order?.ucode ?? defaults?.work_order_ucode ?? null)
    setDescription(transaction?.description ?? defaults?.description ?? "")
  }, [defaults, open, transaction])

  const categories = transactionType === "income" ? incomeCategories : expenseCategories

  useEffect(() => {
    if (!categories.includes(category)) {
      const nextCategory = categories[0]
      setCategory(nextCategory)
      if (!isEdit) setCounterpartyType(inferCounterparty(nextCategory))
    }
  }, [categories, category, isEdit])

  const title = isEdit ? "Editar movimiento" : "Nuevo movimiento"
  const selectedCategoryLabel = categoryLabels[category]

  const submit = async () => {
    if (!isEdit && !/^\d+(\.\d{1,2})?$/.test(amount.trim())) {
      toast.error("Monto inválido")
      return
    }
    if (isEdit) {
      await onSubmit({
        transaction_date: transactionDate,
        payment_method: paymentMethod,
        category,
        description: emptyToUndefined(description),
      })
      return
    }
    await onSubmit({
      transaction_type: transactionType,
      amount: amount.trim(),
      currency,
      transaction_date: transactionDate,
      payment_method: paymentMethod,
      category,
      counterparty_type: counterpartyType,
      client_ucode: counterpartyType === "client" ? clientUcode ?? undefined : undefined,
      supplier_ucode: counterpartyType === "supplier" ? supplierUcode ?? undefined : undefined,
      work_order_ucode: workOrderUcode ?? undefined,
      description: emptyToUndefined(description),
    })
  }

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      onSubmit={submit}
      isSubmitting={isSubmitting}
    >
      {!isEdit ? (
        <Field label="Tipo">
          <SegmentedRadio
            value={transactionType}
            onValueChange={(value) => setTransactionType(value as TransactionType)}
            options={[
              { value: "income", label: "Ingreso" },
              { value: "expense", label: "Egreso" },
            ]}
          />
        </Field>
      ) : null}

      {!isEdit ? (
        <div className="grid gap-4 sm:grid-cols-[1fr_7rem]">
          <Field label="Monto">
            <Input value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="15000.00" />
          </Field>
          <Field label="Moneda">
            <Input value={currency} onChange={(event) => setCurrency(event.target.value.toUpperCase())} maxLength={3} />
          </Field>
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Fecha">
          <Input
            type="date"
            value={transactionDate}
            onChange={(event) => setTransactionDate(event.target.value)}
          />
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

      <Field label="Categoría">
        <NativeSelect
          value={category}
          onChange={(value) => {
            const next = value as TransactionCategory
            setCategory(next)
            if (!isEdit) setCounterpartyType(inferCounterparty(next))
          }}
        >
          {categories.map((item) => (
            <option key={item} value={item}>
              {categoryLabels[item]}
            </option>
          ))}
        </NativeSelect>
      </Field>

      {!isEdit ? (
        <Field label="Contraparte">
          <SegmentedRadio
            value={counterpartyType}
            onValueChange={(value) => setCounterpartyType(value as CounterpartyType)}
            options={[
              { value: "client", label: "Cliente" },
              { value: "supplier", label: "Proveedor" },
              { value: "none", label: "Sin contraparte" },
            ]}
          />
        </Field>
      ) : null}

      {!isEdit && counterpartyType === "client" ? (
        <Field label="Cliente">
          <EntityCombobox<Client>
            queryKey={["clients"]}
            value={clientUcode}
            onChange={setClientUcode}
            fetchOptions={(q) => searchClients(q, 1, 20).then((data) => data.clients)}
            getKey={(option) => option.ucode}
            getLabel={(option) => option.name}
            placeholder="Seleccionar cliente"
          />
        </Field>
      ) : null}

      {!isEdit && counterpartyType === "supplier" ? (
        <Field label="Proveedor">
          <EntityCombobox<Supplier>
            queryKey={["suppliers"]}
            value={supplierUcode}
            onChange={setSupplierUcode}
            fetchOptions={() => listSuppliers()}
            getKey={(option) => option.ucode}
            getLabel={(option) => option.name}
            placeholder="Seleccionar proveedor"
          />
        </Field>
      ) : null}

      {!isEdit ? (
        <Field label="Orden de trabajo (opcional)">
          <EntityCombobox<WorkOrder>
            queryKey={["work-orders"]}
            value={workOrderUcode}
            onChange={setWorkOrderUcode}
            fetchOptions={(q) => searchWorkOrders({ q, page: 1, page_size: 20 }).then((data) => data.work_orders)}
            getKey={(option) => option.ucode}
            getLabel={(option) => `OT ${option.wo_number} · ${option.client.name}`}
            placeholder="Buscar OT"
          />
        </Field>
      ) : null}

      <Field label="Descripción">
        <Textarea
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          placeholder={selectedCategoryLabel}
          maxLength={2000}
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

function SegmentedRadio({
  value,
  onValueChange,
  options,
}: {
  value: string
  onValueChange: (value: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <div className="grid gap-2 sm:grid-cols-3" role="radiogroup">
      {options.map((option) => (
        <Button
          key={option.value}
          type="button"
          role="radio"
          aria-checked={value === option.value}
          variant={value === option.value ? "default" : "outline"}
          className="justify-start"
          onClick={() => onValueChange(option.value)}
        >
          {option.label}
        </Button>
      ))}
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

function inferCounterparty(category: TransactionCategory): CounterpartyType {
  if (category === "wo_payment" || category === "wo_deposit") return "client"
  if (category === "part_purchase" || category === "supplies") return "supplier"
  return "none"
}

function todayISO() {
  return new Date().toISOString().slice(0, 10)
}

function emptyToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}

export function showTransactionError(error: unknown) {
  if (error instanceof AxiosError) {
    const code = (error.response?.data as { error?: string } | undefined)?.error
    if (code === "counterparty_mismatch") {
      toast.error("Contraparte inválida")
      return
    }
    if (code === "invalid_amount") {
      toast.error("Monto inválido")
      return
    }
  }
  toast.error("No se pudo registrar el movimiento")
}
