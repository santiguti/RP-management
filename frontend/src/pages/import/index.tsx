import { useMemo, useState } from "react"
import { AxiosError } from "axios"
import { CheckCircle2, FileSpreadsheet, Upload } from "lucide-react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"

import type { ImportKind, ImportResult, ImportRowError } from "@/api/import"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useCommitImport, usePreviewImport } from "@/hooks/use-import"
import { cn } from "@/lib/utils"

const kindLabels: Record<ImportKind, string> = {
  clients: "Clientes",
  parts: "Repuestos",
  transactions: "Movimientos",
}

const destinationByKind: Record<ImportKind, string> = {
  clients: "/clients",
  parts: "/parts",
  transactions: "/transactions",
}

const previewColumns: Record<ImportKind, Array<{ key: string; label: string; render?: (row: Record<string, unknown>) => string }>> = {
  clients: [
    { key: "name", label: "Nombre" },
    { key: "client_type", label: "Tipo" },
    { key: "phone", label: "Teléfono" },
    { key: "email", label: "Email" },
    { key: "dni_cuit", label: "DNI/CUIT" },
  ],
  parts: [
    { key: "name", label: "Nombre" },
    { key: "sku", label: "SKU" },
    { key: "unit", label: "Unidad" },
    { key: "reorder_level", label: "Punto de reposición" },
    { key: "default_cost", label: "Costo" },
    { key: "default_sale_price", label: "Precio venta" },
  ],
  transactions: [
    { key: "transaction_date", label: "Fecha" },
    { key: "transaction_type", label: "Tipo" },
    { key: "category", label: "Categoría" },
    { key: "payment_method", label: "Método" },
    { key: "counterparty", label: "Cliente/Proveedor", render: (row) => value(row.client_phone) || value(row.supplier_name) },
    { key: "wo_number", label: "OT" },
    { key: "amount", label: "Monto" },
  ],
}

export function ImportPage() {
  const nav = useNavigate()
  const [kind, setKind] = useState<ImportKind>("clients")
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<ImportResult | null>(null)
  const preview = usePreviewImport()
  const commit = useCommitImport()

  const previewRows = useMemo(
    () => (result?.preview ?? []).filter(isRecord).slice(0, 20),
    [result],
  )
  const visibleErrors = result?.errors.slice(0, 100) ?? []
  const extraErrors = Math.max((result?.errors.length ?? 0) - visibleErrors.length, 0)

  const onFile = (nextFile: File | undefined) => {
    if (!nextFile) return
    setFile(nextFile)
    setResult(null)
  }

  const onPreview = async () => {
    if (!file) {
      toast.error("Seleccioná un archivo .xlsx")
      return
    }
    try {
      setResult(await preview.mutateAsync({ kind, file }))
    } catch {
      toast.error("No se pudo leer el archivo")
    }
  }

  const onCommit = async () => {
    if (!file || !result) return
    try {
      const committed = await commit.mutateAsync({ kind, file })
      toast.success(`Importadas ${committed.valid} filas`)
      nav(destinationByKind[kind])
    } catch (error) {
      const errors = importFailureErrors(error)
      if (errors) {
        setResult({
          ...result,
          errors,
          invalid: errors.length,
          committed: false,
        })
      }
      toast.error("No se pudo importar — revisá los errores y volvé a intentar")
    }
  }

  const busy = preview.isPending || commit.isPending

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-normal">Importar desde Excel</h1>
      </div>

      <Card className="rounded-md">
        <CardContent className="space-y-5">
          <div className="grid gap-4 lg:grid-cols-[280px_1fr_auto] lg:items-end">
            <div className="space-y-2">
              <div className="text-sm font-medium">Tipo de datos</div>
              <RadioGroup
                value={kind}
                onValueChange={(value) => {
                  setKind(value as ImportKind)
                  setResult(null)
                }}
                className="grid grid-cols-3 gap-2 lg:grid-cols-1"
              >
                {(Object.keys(kindLabels) as ImportKind[]).map((option) => (
                  <label
                    key={option}
                    className={cn(
                      "flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm",
                      option === kind ? "border-primary bg-primary/5" : "bg-background",
                    )}
                  >
                    <RadioGroupItem value={option} />
                    {kindLabels[option]}
                  </label>
                ))}
              </RadioGroup>
            </div>

            <label
              className="flex min-h-28 cursor-pointer flex-col items-center justify-center gap-2 rounded-md border border-dashed bg-background px-4 py-5 text-center text-sm transition-colors hover:bg-accent"
              onDragOver={(event) => {
                event.preventDefault()
              }}
              onDrop={(event) => {
                event.preventDefault()
                onFile(event.dataTransfer.files[0])
              }}
            >
              <Upload className="size-5 text-muted-foreground" />
              <span className="font-medium">Soltá un archivo .xlsx acá o hacé clic para seleccionar</span>
              <span className="text-xs text-muted-foreground">Seleccionar archivo .xlsx</span>
              <input
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                className="sr-only"
                onChange={(event) => onFile(event.target.files?.[0])}
              />
            </label>

            <Button type="button" onClick={onPreview} disabled={busy || !file}>
              <FileSpreadsheet />
              Previsualizar
            </Button>
          </div>

          {file ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <FileSpreadsheet className="size-4" />
              <span className="font-medium text-foreground">{file.name}</span>
              <span>{formatBytes(file.size)}</span>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {result ? (
        <>
          <Card className="rounded-md">
            <CardContent className="flex flex-wrap items-center justify-between gap-3">
              <div className="text-sm font-medium">
                Total filas: {result.total_rows} · Válidas: {result.valid} · Con errores: {result.invalid}
              </div>
              <Button type="button" onClick={onCommit} disabled={busy || result.valid === 0}>
                <CheckCircle2 />
                Confirmar importación
              </Button>
            </CardContent>
          </Card>

          <ErrorCard errors={visibleErrors} extraErrors={extraErrors} />
          <PreviewCard kind={kind} rows={previewRows} />
        </>
      ) : null}
    </div>
  )
}

function ErrorCard({ errors, extraErrors }: { errors: ImportRowError[]; extraErrors: number }) {
  if (errors.length === 0) {
    return (
      <Card className="rounded-md border-emerald-200 bg-emerald-50">
        <CardContent className="text-sm text-emerald-900">Sin errores</CardContent>
      </Card>
    )
  }
  return (
    <Card className="rounded-md border-destructive/30">
      <CardHeader>
        <CardTitle className="text-base text-destructive">Errores encontrados</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Fila</TableHead>
              <TableHead>Columna</TableHead>
              <TableHead>Error</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {errors.map((error, index) => (
              <TableRow key={`${error.row}-${error.column}-${index}`}>
                <TableCell>{error.row}</TableCell>
                <TableCell>{error.column}</TableCell>
                <TableCell>{error.message}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {extraErrors > 0 ? <div className="text-sm text-muted-foreground">+ {extraErrors} más</div> : null}
      </CardContent>
    </Card>
  )
}

function PreviewCard({ kind, rows }: { kind: ImportKind; rows: Array<Record<string, unknown>> }) {
  const columns = previewColumns[kind]
  return (
    <Card className="rounded-md border-emerald-200">
      <CardHeader>
        <CardTitle className="text-base">Filas válidas</CardTitle>
      </CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <div className="text-sm text-muted-foreground">No hay filas válidas para importar</div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                {columns.map((column) => (
                  <TableHead key={column.key}>{column.label}</TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row, index) => (
                <TableRow key={index}>
                  {columns.map((column) => (
                    <TableCell key={column.key}>{column.render ? column.render(row) : value(row[column.key])}</TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

function importFailureErrors(error: unknown): ImportRowError[] | null {
  if (!(error instanceof AxiosError)) return null
  const data = error.response?.data as { error?: string; errors?: ImportRowError[] } | undefined
  if (data?.error !== "commit_failed") return null
  return data.errors ?? []
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

function value(input: unknown) {
  if (input === null || input === undefined || input === "") return "-"
  return String(input)
}

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}
