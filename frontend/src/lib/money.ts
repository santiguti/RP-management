import type {
  PaymentMethod,
  TransactionCategory,
  TransactionType,
} from "@/api/transactions"

export const paymentMethodLabels: Record<PaymentMethod, string> = {
  cash: "Efectivo",
  transfer: "Transferencia",
  card: "Tarjeta",
  mercadopago: "MercadoPago",
  other: "Otro",
}

export const categoryLabels: Record<TransactionCategory, string> = {
  wo_payment: "Pago de OT",
  wo_deposit: "Seña de cliente",
  part_purchase: "Compra de repuesto",
  supplies: "Insumos",
  rent: "Alquiler",
  utilities: "Servicios",
  salary: "Sueldos",
  taxes: "Impuestos",
  food: "Comida",
  transport: "Transporte",
  other_income: "Otros ingresos",
  other_expense: "Otros egresos",
}

export const incomeCategories: TransactionCategory[] = [
  "wo_payment",
  "wo_deposit",
  "other_income",
]

export const expenseCategories: TransactionCategory[] = [
  "part_purchase",
  "supplies",
  "rent",
  "utilities",
  "salary",
  "taxes",
  "food",
  "transport",
  "other_expense",
]

export const allCategories: TransactionCategory[] = [...incomeCategories, ...expenseCategories]

export const paymentMethods = Object.keys(paymentMethodLabels) as PaymentMethod[]

export function formatARS(amount: string, type: TransactionType) {
  const formatted = formatARSValue(amount)
  return `${type === "income" ? "+" : "-"} ${formatted}`
}

export function formatARSValue(amount: string | number) {
  const value = Number(amount)
  return new Intl.NumberFormat("es-AR", {
    style: "currency",
    currency: "ARS",
  }).format(Number.isFinite(value) ? value : 0)
}

export function formatDateOnly(value?: string) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("es-AR", { dateStyle: "short" }).format(new Date(`${value}T00:00:00`))
}

// Cents-safe arithmetic over the backend's NUMERIC(14,2) decimal strings.
// Never go through floats: 0.1 + 0.2 problems are exactly what the
// string-serialized money convention exists to avoid.
export function moneyToCents(amount?: string): number | null {
  if (!amount) return null
  const match = amount.trim().match(/^(\d+)(?:\.(\d{1,2}))?$/)
  if (!match) return null
  return Number(match[1]) * 100 + Number((match[2] ?? "").padEnd(2, "0"))
}

export function centsToMoney(cents: number): string {
  const sign = cents < 0 ? "-" : ""
  const abs = Math.abs(cents)
  return `${sign}${Math.floor(abs / 100)}.${String(abs % 100).padStart(2, "0")}`
}
