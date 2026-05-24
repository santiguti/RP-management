import { useMemo, useState } from "react"
import { Pencil, Play, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import type { RecurringExpense, RecurringExpenseInput } from "@/api/recurring-expenses"
import { DataTable, type Column } from "@/components/data-table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  useCreateRecurringExpense,
  useDeleteRecurringExpense,
  useRecurringExpenses,
  useRunRecurringNow,
  useUpdateRecurringExpense,
} from "@/hooks/use-recurring-expenses"
import { categoryLabels, formatARSValue, formatDateOnly } from "@/lib/money"
import { RecurringExpenseForm } from "./recurring-form"

export function RecurringExpensesPage() {
  const [q, setQ] = useState("")
  const [formOpen, setFormOpen] = useState(false)
  const [selected, setSelected] = useState<RecurringExpense | undefined>()
  const recurringExpenses = useRecurringExpenses()
  const createRecurringExpense = useCreateRecurringExpense()
  const updateRecurringExpense = useUpdateRecurringExpense()
  const deleteRecurringExpense = useDeleteRecurringExpense()
  const runNow = useRunRecurringNow()

  const rows = useMemo(() => {
    const needle = q.trim().toLowerCase()
    const data = recurringExpenses.data ?? []
    if (!needle) return data
    return data.filter((row) => row.name.toLowerCase().includes(needle))
  }, [q, recurringExpenses.data])

  const openCreate = () => {
    setSelected(undefined)
    setFormOpen(true)
  }

  const openEdit = (expense: RecurringExpense) => {
    setSelected(expense)
    setFormOpen(true)
  }

  const onSubmit = async (input: RecurringExpenseInput) => {
    try {
      if (selected) {
        await updateRecurringExpense.mutateAsync({ ucode: selected.ucode, input })
        toast.success("Gasto fijo actualizado")
      } else {
        await createRecurringExpense.mutateAsync(input)
        toast.success("Gasto fijo creado")
      }
      setFormOpen(false)
      setSelected(undefined)
    } catch {
      toast.error("No se pudo guardar el gasto fijo")
    }
  }

  const onRunNow = async (expense: RecurringExpense) => {
    try {
      const result = await runNow.mutateAsync(expense.ucode)
      if (result.transaction) {
        toast.success(`Movimiento generado por ${formatDateOnly(result.transaction.transaction_date)}`)
      } else {
        toast.info("Ya se había generado este mes")
      }
    } catch {
      toast.error("No se pudo ejecutar la regla")
    }
  }

  const onDelete = async (expense: RecurringExpense) => {
    if (!window.confirm("¿Eliminar gasto fijo?\nNo se generarán más movimientos a partir de esta regla.")) return
    try {
      await deleteRecurringExpense.mutateAsync(expense.ucode)
      toast.success("Gasto fijo eliminado")
      if (selected?.ucode === expense.ucode) setSelected(undefined)
    } catch {
      toast.error("No se pudo eliminar el gasto fijo")
    }
  }

  const columns = useMemo<Column<RecurringExpense>[]>(
    () => [
      { header: "Nombre", cell: (row) => <span className="font-medium">{row.name}</span> },
      {
        header: "Monto",
        className: "text-right",
        cell: (row) => formatARSValue(row.amount),
      },
      { header: "Día del mes", cell: (row) => row.day_of_month },
      { header: "Categoría", cell: (row) => categoryLabels[row.category] },
      {
        header: "Próxima ejecución",
        cell: (row) => {
          const due = dueDate(row.day_of_month)
          return (
            <div className="flex items-center gap-2">
              <span>{formatDateOnly(due)}</span>
              {isPastDate(due) ? <Badge variant="destructive">Vencido</Badge> : null}
            </div>
          )
        },
      },
      {
        header: "Estado",
        cell: (row) => <Badge variant={row.active ? "default" : "outline"}>{row.active ? "Activo" : "Inactivo"}</Badge>,
      },
      {
        header: "Acciones",
        className: "w-64 text-right",
        cell: (row) => (
          <div className="flex justify-end gap-1">
            <Button type="button" variant="ghost" size="sm" onClick={() => openEdit(row)}>
              <Pencil />
              Editar
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={() => void onRunNow(row)}>
              <Play />
              Ejecutar ahora
            </Button>
            <Button type="button" variant="ghost" size="icon" onClick={() => void onDelete(row)} aria-label="Eliminar">
              <Trash2 />
            </Button>
          </div>
        ),
      },
    ],
    [openEdit, onDelete, onRunNow],
  )

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Gastos fijos</h1>
          <p className="text-sm text-muted-foreground">Reglas mensuales para egresos recurrentes.</p>
        </div>
        <Button type="button" onClick={openCreate}>
          <Plus />
          Nueva regla
        </Button>
      </div>

      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(row) => row.ucode}
        isLoading={recurringExpenses.isLoading}
        emptyMessage="Sin gastos fijos"
        page={1}
        pageSize={rows.length || 25}
        total={rows.length}
        onPageChange={() => undefined}
        searchValue={q}
        onSearchChange={setQ}
        searchPlaceholder="Buscar por nombre"
      />

      <RecurringExpenseForm
        open={formOpen}
        onOpenChange={(open) => {
          setFormOpen(open)
          if (!open) setSelected(undefined)
        }}
        expense={selected}
        onSubmit={onSubmit}
        isSubmitting={
          createRecurringExpense.isPending ||
          updateRecurringExpense.isPending ||
          deleteRecurringExpense.isPending ||
          runNow.isPending
        }
      />
    </div>
  )
}

function dueDate(dayOfMonth: number) {
  const today = new Date()
  const due = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), dayOfMonth))
  if (today.getUTCDate() >= dayOfMonth) return due.toISOString().slice(0, 10)
  due.setUTCMonth(due.getUTCMonth() - 1)
  return due.toISOString().slice(0, 10)
}

function isPastDate(value: string) {
  return value < new Date().toISOString().slice(0, 10)
}
