import { useMemo, useState } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import type {
  ListTransactionsParams,
  Transaction,
  TransactionCategory,
  TransactionInput,
  TransactionType,
  UpdateTransactionInput,
} from "@/api/transactions"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DataTable, type Column } from "@/components/data-table"
import {
  useCreateTransaction,
  useDeleteTransaction,
  useTransactions,
  useUpdateTransaction,
} from "@/hooks/use-transactions"
import {
  allCategories,
  categoryLabels,
  formatARS,
  formatDateOnly,
} from "@/lib/money"
import { TransactionForm, showTransactionError } from "./transaction-form"

const PAGE_SIZE = 25

export function TransactionsListPage() {
  const [params, setParams] = useSearchParams()
  const [formOpen, setFormOpen] = useState(false)
  const [selected, setSelected] = useState<Transaction | undefined>()
  const page = Number(params.get("page") ?? "1") || 1
  const filters: ListTransactionsParams = {
    from: params.get("from") ?? undefined,
    to: params.get("to") ?? undefined,
    type: parseTypeFilter(params.get("type")),
    category: parseCategoryFilter(params.get("category")),
    page,
    page_size: PAGE_SIZE,
  }
  const transactions = useTransactions(filters)
  const createTransaction = useCreateTransaction()
  const updateTransaction = useUpdateTransaction()
  const deleteTransaction = useDeleteTransaction()

  const columns = useMemo<Column<Transaction>[]>(
    () => [
      { header: "Fecha", cell: (row) => formatDateOnly(row.transaction_date) },
      {
        header: "Tipo",
        cell: (row) => (
          <Badge variant={row.transaction_type === "income" ? "default" : "outline"}>
            {row.transaction_type === "income" ? "Ingreso" : "Egreso"}
          </Badge>
        ),
      },
      { header: "Categoría", cell: (row) => categoryLabels[row.category] },
      { header: "Contraparte", cell: (row) => row.client?.name ?? row.supplier?.name ?? "-" },
      {
        header: "OT",
        cell: (row) =>
          row.work_order ? (
            <Link
              className="text-primary underline-offset-4 hover:underline"
              to={`/work-orders/${row.work_order.ucode}`}
              onClick={(event) => event.stopPropagation()}
            >
              {row.work_order.wo_number}
            </Link>
          ) : (
            "-"
          ),
      },
      {
        header: "Monto",
        className: "text-right",
        cell: (row) => (
          <span className={row.transaction_type === "income" ? "text-emerald-700" : "text-destructive"}>
            {formatARS(row.amount, row.transaction_type)}
          </span>
        ),
      },
      {
        header: "Descripción",
        cell: (row) => <span className="line-clamp-1 max-w-56">{row.description ?? "-"}</span>,
      },
      {
        header: "",
        className: "w-14 text-right",
        cell: (row) => (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={(event) => {
              event.stopPropagation()
              void onDelete(row)
            }}
          >
            <Trash2 />
          </Button>
        ),
      },
    ],
    [],
  )

  const updateParams = (next: Record<string, string | number | undefined>) => {
    const out = new URLSearchParams(params)
    for (const [key, value] of Object.entries(next)) {
      if (value === undefined || value === "") out.delete(key)
      else out.set(key, String(value))
    }
    if (!("page" in next)) out.set("page", "1")
    setParams(out)
  }

  const onCreate = async (input: TransactionInput | UpdateTransactionInput) => {
    try {
      await createTransaction.mutateAsync(input as TransactionInput)
      toast.success("Movimiento registrado")
      setFormOpen(false)
    } catch (error) {
      showTransactionError(error)
    }
  }

  const onUpdate = async (input: TransactionInput | UpdateTransactionInput) => {
    if (!selected) return
    try {
      await updateTransaction.mutateAsync({ ucode: selected.ucode, input: input as UpdateTransactionInput })
      toast.success("Movimiento actualizado")
      setSelected(undefined)
    } catch {
      toast.error("No se pudo actualizar el movimiento")
    }
  }

  const onDelete = async (transaction: Transaction) => {
    if (!window.confirm("¿Eliminar movimiento?\nEl movimiento se quitará de los reportes.")) return
    try {
      await deleteTransaction.mutateAsync({
        ucode: transaction.ucode,
        workOrderUcode: transaction.work_order?.ucode,
      })
      toast.success("Movimiento eliminado")
      if (selected?.ucode === transaction.ucode) setSelected(undefined)
    } catch {
      toast.error("No se pudo eliminar el movimiento")
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Movimientos</h1>
          <p className="text-sm text-muted-foreground">Ingresos, egresos y pagos asociados a órdenes.</p>
        </div>
        <Button type="button" onClick={() => setFormOpen(true)}>
          <Plus />
          Nuevo movimiento
        </Button>
      </div>

      <div className="grid gap-3 rounded-md border bg-background p-3 sm:grid-cols-2 lg:grid-cols-5">
        <FilterField label="Desde">
          <input
            type="date"
            className={inputClassName}
            value={filters.from ?? ""}
            onChange={(event) => updateParams({ from: event.target.value })}
          />
        </FilterField>
        <FilterField label="Hasta">
          <input
            type="date"
            className={inputClassName}
            value={filters.to ?? ""}
            onChange={(event) => updateParams({ to: event.target.value })}
          />
        </FilterField>
        <FilterField label="Tipo">
          <select
            className={inputClassName}
            value={filters.type}
            onChange={(event) => updateParams({ type: event.target.value })}
          >
            <option value="">Todos</option>
            <option value="income">Ingresos</option>
            <option value="expense">Egresos</option>
          </select>
        </FilterField>
        <FilterField label="Categoría">
          <select
            className={inputClassName}
            value={filters.category}
            onChange={(event) => updateParams({ category: event.target.value })}
          >
            <option value="">Todas</option>
            {allCategories.map((category) => (
              <option key={category} value={category}>
                {categoryLabels[category]}
              </option>
            ))}
          </select>
        </FilterField>
        <div className="flex items-end">
          <Button
            type="button"
            variant="outline"
            className="w-full"
            onClick={() => setParams(new URLSearchParams())}
          >
            Limpiar filtros
          </Button>
        </div>
      </div>

      <DataTable
        columns={columns}
        rows={transactions.data?.transactions ?? []}
        rowKey={(row) => row.ucode}
        onRowClick={setSelected}
        isLoading={transactions.isLoading}
        emptyMessage="No hay movimientos en este rango"
        page={transactions.data?.page ?? page}
        pageSize={transactions.data?.page_size ?? PAGE_SIZE}
        total={transactions.data?.total ?? 0}
        onPageChange={(nextPage) => updateParams({ page: nextPage })}
        searchValue=""
        onSearchChange={() => undefined}
        showSearch={false}
      />

      <TransactionForm
        open={formOpen}
        onOpenChange={setFormOpen}
        onSubmit={onCreate}
        isSubmitting={createTransaction.isPending}
      />
      <TransactionForm
        open={Boolean(selected)}
        onOpenChange={(open) => {
          if (!open) setSelected(undefined)
        }}
        transaction={selected}
        onSubmit={onUpdate}
        isSubmitting={updateTransaction.isPending}
      />
    </div>
  )
}

function FilterField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="space-y-1 text-sm">
      <span className="font-medium">{label}</span>
      {children}
    </label>
  )
}

const inputClassName =
  "border-input bg-background h-9 w-full rounded-md border px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"

function parseTypeFilter(value: string | null): TransactionType | "" {
  return value === "income" || value === "expense" ? value : ""
}

function parseCategoryFilter(value: string | null): TransactionCategory | "" {
  return allCategories.includes(value as TransactionCategory) ? (value as TransactionCategory) : ""
}
