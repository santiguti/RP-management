import { useMemo, useState } from "react"
import { Link, useNavigate, useParams } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft, Pencil, Plus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import type { ClientInput } from "@/api/clients"
import type { Device } from "@/api/devices"
import { listArticleTypes, listBrands, listDeviceModelsByBrand, type Lookup } from "@/api/lookups"
import { DataTable, type Column } from "@/components/data-table"
import { FormDialog } from "@/components/form-dialog"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useClient, useClientDevices, useDeleteClient, useUpdateClient } from "@/hooks/use-clients"
import { useCreateDevice } from "@/hooks/use-devices"
import { ClientForm } from "./client-form"
import { DeviceForm } from "./device-form"
import { formatDate, showClientError } from "./list"

export function ClientDetailPage() {
  const { ucode } = useParams()
  const navigate = useNavigate()
  const client = useClient(ucode)
  const devices = useClientDevices(ucode)
  const updateClient = useUpdateClient()
  const deleteClient = useDeleteClient()
  const createDevice = useCreateDevice(ucode)
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deviceOpen, setDeviceOpen] = useState(false)

  const brands = useQuery({ queryKey: ["brands"], queryFn: listBrands })
  const articleTypes = useQuery({ queryKey: ["article-types"], queryFn: listArticleTypes })
  const deviceBrandUcodes = useMemo(
    () => Array.from(new Set((devices.data ?? []).map((device) => device.brand_ucode))),
    [devices.data],
  )
  const models = useQuery({
    queryKey: ["device-models", deviceBrandUcodes],
    queryFn: async () => {
      const chunks = await Promise.all(deviceBrandUcodes.map((brandUcode) => listDeviceModelsByBrand(brandUcode)))
      return chunks.flat()
    },
    enabled: deviceBrandUcodes.length > 0,
  })

  const lookupMaps = useMemo(
    () => ({
      brands: toMap(brands.data ?? []),
      articleTypes: toMap(articleTypes.data ?? []),
      models: toMap(models.data ?? []),
    }),
    [articleTypes.data, brands.data, models.data],
  )

  const deviceColumns = useMemo<Column<Device>[]>(
    () => [
      {
        header: "Equipo",
        cell: (row) => (
          <span className="font-medium">
            {lookupMaps.brands.get(row.brand_ucode) ?? "Marca"} {row.model_ucode ? lookupMaps.models.get(row.model_ucode) ?? "" : ""}
          </span>
        ),
      },
      { header: "Tipo", cell: (row) => lookupMaps.articleTypes.get(row.article_type_ucode) ?? "-" },
      { header: "Serie", cell: (row) => row.serial_number ?? "-" },
    ],
    [lookupMaps],
  )

  if (client.isLoading) return <div className="text-sm text-muted-foreground">Cargando cliente...</div>
  if (!client.data) return <div className="text-sm text-muted-foreground">Cliente no encontrado</div>

  const onUpdate = async (input: ClientInput) => {
    try {
      await updateClient.mutateAsync({ ucode: client.data.ucode, input })
      toast.success("Cliente actualizado")
      setEditOpen(false)
    } catch (error) {
      showClientError(error)
    }
  }

  const onDelete = async () => {
    await deleteClient.mutateAsync(client.data.ucode)
    toast.success("Cliente eliminado")
    navigate("/clients")
  }

  const onCreateDevice = async (input: Parameters<typeof createDevice.mutateAsync>[0]) => {
    try {
      await createDevice.mutateAsync(input)
      toast.success("Dispositivo creado")
      setDeviceOpen(false)
    } catch {
      toast.error("No se pudo guardar el dispositivo")
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-2">
          <Button asChild variant="ghost" size="sm" className="-ml-2">
            <Link to="/clients">
              <ArrowLeft />
              Clientes
            </Link>
          </Button>
          <div>
            <h1 className="text-2xl font-semibold tracking-normal">{client.data.name}</h1>
            <p className="text-sm text-muted-foreground">{client.data.phone ?? client.data.email ?? "Sin contacto cargado"}</p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="outline" onClick={() => setEditOpen(true)}>
            <Pencil />
            Editar
          </Button>
          <Button type="button" variant="destructive" onClick={() => setDeleteOpen(true)}>
            <Trash2 />
            Eliminar
          </Button>
        </div>
      </div>

      <Tabs defaultValue="datos">
        <TabsList>
          <TabsTrigger value="datos">Datos</TabsTrigger>
          <TabsTrigger value="dispositivos">Dispositivos</TabsTrigger>
        </TabsList>
        <TabsContent value="datos" className="mt-4">
          <ClientDataCard client={client.data} />
        </TabsContent>
        <TabsContent value="dispositivos" className="mt-4 space-y-4">
          <div className="flex justify-end">
            <Button type="button" onClick={() => setDeviceOpen(true)}>
              <Plus />
              Nuevo dispositivo
            </Button>
          </div>
          <DataTable
            columns={deviceColumns}
            rows={devices.data ?? []}
            rowKey={(row) => row.ucode}
            isLoading={devices.isLoading}
            emptyMessage="Este cliente no tiene dispositivos"
            page={1}
            pageSize={50}
            total={devices.data?.length ?? 0}
            onPageChange={() => undefined}
            searchValue=""
            onSearchChange={() => undefined}
            searchPlaceholder="Buscar dispositivo"
          />
        </TabsContent>
      </Tabs>

      <ClientForm
        open={editOpen}
        onOpenChange={setEditOpen}
        client={client.data}
        onSubmit={onUpdate}
        isSubmitting={updateClient.isPending}
      />
      <FormDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="¿Eliminar cliente?"
        submitLabel="Eliminar"
        onSubmit={onDelete}
        isSubmitting={deleteClient.isPending}
      >
        <p className="text-sm text-muted-foreground">
          El cliente no aparecerá en búsquedas pero se mantiene en el historial.
        </p>
      </FormDialog>
      <DeviceForm
        open={deviceOpen}
        onOpenChange={setDeviceOpen}
        clientUcode={client.data.ucode}
        onSubmit={onCreateDevice}
        isSubmitting={createDevice.isPending}
      />
    </div>
  )
}

function ClientDataCard({ client }: { client: NonNullable<ReturnType<typeof useClient>["data"]> }) {
  const rows = [
    ["Nombre", client.name],
    ["Teléfono", client.phone ?? "-"],
    ["Email", client.email ?? "-"],
    ["DNI/CUIT", client.dni_cuit ?? "-"],
    ["Dirección", client.address ?? "-"],
    ["Notas", client.notes ?? "-"],
    ["Tipo", client.client_type === "empresa" ? "Empresa" : "Particular"],
    ["Creado", formatDate(client.created_ts)],
  ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>Datos</CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid gap-4 sm:grid-cols-2">
          {rows.map(([label, value]) => (
            <div key={label} className="space-y-1">
              <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
              <dd className="text-sm">{value}</dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  )
}

function toMap<T extends Lookup>(rows: T[]) {
  return new Map(rows.map((row) => [row.ucode, row.name]))
}
