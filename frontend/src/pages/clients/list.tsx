import { useMemo, useState } from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { AxiosError } from "axios"
import { Plus } from "lucide-react"
import { toast } from "sonner"

import type { Client, ClientInput } from "@/api/clients"
import { DataTable, type Column } from "@/components/data-table"
import { DownloadCsvButton } from "@/components/download-csv-button"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { useClients, useCreateClient } from "@/hooks/use-clients"
import { ClientForm } from "./client-form"

const PAGE_SIZE = 25

export function ClientsListPage() {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const [formOpen, setFormOpen] = useState(false)
  const q = params.get("q") ?? ""
  const page = Number(params.get("page") ?? "1") || 1
  const clients = useClients(q, page, PAGE_SIZE)
  const createClient = useCreateClient()

  const columns = useMemo<Column<Client>[]>(
    () => [
      { header: "Nombre", cell: (row) => <span className="font-medium">{row.name}</span> },
      { header: "Teléfono", cell: (row) => row.phone ?? "-" },
      { header: "Email", cell: (row) => row.email ?? "-" },
      {
        header: "Tipo",
        cell: (row) => <Badge variant="outline">{row.client_type === "empresa" ? "Empresa" : "Particular"}</Badge>,
      },
      { header: "Creado", cell: (row) => formatDate(row.created_ts) },
    ],
    [],
  )

  const updateParams = (next: { q?: string; page?: number }) => {
    const out = new URLSearchParams(params)
    if (next.q !== undefined) {
      if (next.q) out.set("q", next.q)
      else out.delete("q")
      out.set("page", "1")
    }
    if (next.page !== undefined) out.set("page", String(next.page))
    setParams(out)
  }

  const onCreate = async (input: ClientInput) => {
    try {
      const client = await createClient.mutateAsync(input)
      toast.success("Cliente creado")
      setFormOpen(false)
      navigate(`/clients/${client.ucode}`)
    } catch (error) {
      showClientError(error)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Clientes</h1>
          <p className="text-sm text-muted-foreground">Administrá datos de contacto y dispositivos.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <DownloadCsvButton href="/api/v1/clients.csv" />
          <Button type="button" onClick={() => setFormOpen(true)}>
            <Plus />
            Nuevo cliente
          </Button>
        </div>
      </div>

      <DataTable
        columns={columns}
        rows={clients.data?.clients ?? []}
        rowKey={(row) => row.ucode}
        onRowClick={(row) => navigate(`/clients/${row.ucode}`)}
        isLoading={clients.isLoading}
        emptyMessage="No hay clientes"
        page={clients.data?.page ?? page}
        pageSize={clients.data?.page_size ?? PAGE_SIZE}
        total={clients.data?.total ?? 0}
        onPageChange={(nextPage) => updateParams({ page: nextPage })}
        searchValue={q}
        onSearchChange={(nextQ) => updateParams({ q: nextQ })}
        searchPlaceholder="Buscar por nombre, teléfono o email"
      />

      <ClientForm
        open={formOpen}
        onOpenChange={setFormOpen}
        onSubmit={onCreate}
        isSubmitting={createClient.isPending}
      />
    </div>
  )
}

export function formatDate(value?: string) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("es-AR", { dateStyle: "short", timeStyle: "short" }).format(new Date(value))
}

export function showClientError(error: unknown) {
  if (error instanceof AxiosError) {
    const code = (error.response?.data as { error?: string } | undefined)?.error
    if (code === "invalid_phone") {
      toast.error("Teléfono inválido")
      return
    }
    if (code === "phone_already_exists") {
      toast.error("Ya existe un cliente con ese teléfono")
      return
    }
  }
  toast.error("No se pudo guardar el cliente")
}
