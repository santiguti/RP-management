import { useMemo, useState } from "react"
import type { FormEvent, ReactNode } from "react"
import { useNavigate } from "react-router-dom"
import { Plus } from "lucide-react"
import { toast } from "sonner"

import { listClientDevices, searchClients, type Client } from "@/api/clients"
import type { Device, DeviceInput } from "@/api/devices"
import { EntityCombobox } from "@/components/entity-combobox"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Textarea } from "@/components/ui/textarea"
import { useCreateDevice } from "@/hooks/use-devices"
import { useIntakeWorkOrder } from "@/hooks/use-work-orders"
import { DeviceForm } from "@/pages/clients/device-form"

export function IntakeWorkOrderPage() {
  const navigate = useNavigate()
  const [clientUcode, setClientUcode] = useState<string | null>(null)
  const [deviceUcode, setDeviceUcode] = useState<string | null>(null)
  const [serviceType, setServiceType] = useState<"in_shop" | "on_site">("in_shop")
  const [reportedIssue, setReportedIssue] = useState("")
  const [intakeNotes, setIntakeNotes] = useState("")
  const [accessories, setAccessories] = useState("")
  const [devicePin, setDevicePin] = useState("")
  const [deviceFormOpen, setDeviceFormOpen] = useState(false)
  const intake = useIntakeWorkOrder()
  const createDevice = useCreateDevice(clientUcode ?? undefined)

  const deviceFetcher = useMemo(
    () => async (q: string) => {
      if (!clientUcode) return []
      const rows = await listClientDevices(clientUcode)
      return filterDevices(rows, q)
    },
    [clientUcode]
  )

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!clientUcode || !deviceUcode || !reportedIssue.trim()) {
      toast.error("Completá cliente, dispositivo y problema reportado")
      return
    }
    try {
      const wo = await intake.mutateAsync({
        client_ucode: clientUcode,
        device_ucode: deviceUcode,
        service_type: serviceType,
        reported_issue: reportedIssue.trim(),
        intake_notes: emptyToUndefined(intakeNotes),
        accessories: emptyToUndefined(accessories),
        device_pin: emptyToUndefined(devicePin),
      })
      toast.success(`Orden de trabajo creada (#${wo.wo_number})`)
      navigate(`/work-orders/${wo.ucode}`)
    } catch {
      toast.error("No se pudo crear la orden")
    }
  }

  const onCreateDevice = async (input: DeviceInput) => {
    try {
      const device = await createDevice.mutateAsync(input)
      setDeviceUcode(device.ucode)
      setDeviceFormOpen(false)
      toast.success("Dispositivo creado")
    } catch {
      toast.error("No se pudo crear el dispositivo")
    }
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-normal">Nueva orden de trabajo</h1>
        <p className="text-sm text-muted-foreground">Registrá el ingreso de un equipo al servicio técnico.</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Ingreso</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="space-y-5" onSubmit={submit}>
            <Field label="Cliente">
              <EntityCombobox
                queryKey={["clients"]}
                value={clientUcode}
                onChange={(value) => {
                  setClientUcode(value)
                  setDeviceUcode(null)
                }}
                fetchOptions={(q) => searchClients(q, 1, 20).then((data) => data.clients)}
                getKey={(client) => client.ucode}
                getLabel={clientLabel}
                placeholder="Buscar cliente"
                emptyMessage="Sin clientes"
              />
            </Field>

            <Field label="Dispositivo">
              <EntityCombobox
                queryKey={["client-devices", clientUcode ?? "none"]}
                value={deviceUcode}
                onChange={setDeviceUcode}
                fetchOptions={deviceFetcher}
                getKey={(device) => device.ucode}
                getLabel={deviceLabel}
                placeholder={clientUcode ? "Seleccionar dispositivo" : "Elegí un cliente"}
                emptyMessage={clientUcode ? "Sin dispositivos" : "Elegí un cliente"}
                disabled={!clientUcode}
                actionLabel="+ Nuevo dispositivo"
                onAction={() => setDeviceFormOpen(true)}
              />
            </Field>

            <Field label="Tipo de servicio">
              <RadioGroup
                value={serviceType}
                onValueChange={(value) => setServiceType(value as "in_shop" | "on_site")}
                className="grid gap-3 sm:grid-cols-2"
              >
                <RadioOption
                  value="in_shop"
                  label="En taller"
                  checked={serviceType === "in_shop"}
                  onSelect={() => setServiceType("in_shop")}
                />
                <RadioOption
                  value="on_site"
                  label="Domicilio"
                  checked={serviceType === "on_site"}
                  onSelect={() => setServiceType("on_site")}
                />
              </RadioGroup>
            </Field>

            <Field label="Problema reportado">
              <Textarea
                value={reportedIssue}
                onChange={(event) => setReportedIssue(event.target.value)}
                required
                maxLength={2000}
              />
            </Field>

            <Field label="Notas de ingreso" helper="Daños visibles, estado físico, etc.">
              <Textarea
                value={intakeNotes}
                onChange={(event) => setIntakeNotes(event.target.value)}
                maxLength={4000}
              />
            </Field>

            <Field label="Accesorios" helper="Cargador, funda, SIM, etc.">
              <Textarea
                value={accessories}
                onChange={(event) => setAccessories(event.target.value)}
                maxLength={2000}
              />
            </Field>

            <Field label="PIN / contraseña del dispositivo" helper="Solo si el cliente lo provee.">
              <Input
                type="password"
                value={devicePin}
                onChange={(event) => setDevicePin(event.target.value)}
                maxLength={64}
              />
            </Field>

            <div className="flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={() => navigate("/work-orders")}>
                Cancelar
              </Button>
              <Button type="submit" disabled={intake.isPending}>
                <Plus />
                Crear orden
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {clientUcode ? (
        <DeviceForm
          open={deviceFormOpen}
          onOpenChange={setDeviceFormOpen}
          clientUcode={clientUcode}
          onSubmit={onCreateDevice}
          isSubmitting={createDevice.isPending}
        />
      ) : null}
    </div>
  )
}

function Field({ label, helper, children }: { label: string; helper?: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
      {helper ? <p className="text-xs text-muted-foreground">{helper}</p> : null}
    </div>
  )
}

function RadioOption({
  value,
  label,
  checked,
  onSelect,
}: {
  value: string
  label: string
  checked: boolean
  onSelect: () => void
}) {
  return (
    <label
      className="flex min-h-11 cursor-pointer items-center gap-3 rounded-md border px-3 py-2 text-sm"
      onClick={onSelect}
    >
      <RadioGroupItem value={value} checked={checked} onCheckedChange={onSelect} />
      <span>{label}</span>
    </label>
  )
}

function clientLabel(client: Client) {
  return client.phone ? `${client.name} · ${client.phone}` : client.name
}

function deviceLabel(device: Device) {
  if (device.serial_number) return `Serie ${device.serial_number}`
  if (device.description) return device.description
  return `Dispositivo ${device.ucode.slice(0, 8)}`
}

function filterDevices(rows: Device[], q: string) {
  const needle = q.trim().toLowerCase()
  if (!needle) return rows
  return rows.filter((device) =>
    [device.serial_number, device.color, device.description]
      .filter(Boolean)
      .some((value) => value?.toLowerCase().includes(needle))
  )
}

function emptyToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}
