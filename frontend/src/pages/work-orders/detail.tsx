import { useState } from "react"
import { Link, useParams } from "react-router-dom"
import { AxiosError } from "axios"
import { Eye, EyeOff, Pencil } from "lucide-react"
import { toast } from "sonner"
import { useQueryClient } from "@tanstack/react-query"

import type { TransitionInput, UpdateWorkOrderInput, WorkOrder, WoEvent } from "@/api/work-orders"
import { WoStatusBadge } from "@/components/wo-status-badge"
import { FormDialog } from "@/components/form-dialog"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { useTransitionWorkOrder, useUpdateWorkOrder, useWorkOrder } from "@/hooks/use-work-orders"
import { CancelDialog } from "@/pages/work-orders/cancel-dialog"
import { QuoteDialog } from "@/pages/work-orders/quote-dialog"
import { ReadyDialog } from "@/pages/work-orders/ready-dialog"

const eventLabels: Record<WoEvent, string> = {
  start_diagnosis: "Iniciar diagnóstico",
  quote: "Presupuestar",
  approve: "Aprobar presupuesto",
  reject: "Rechazar presupuesto",
  start_repair: "Iniciar reparación",
  mark_waiting_parts: "Esperar repuestos",
  resume_repair: "Reanudar reparación",
  mark_ready: "Marcar listo",
  deliver: "Entregar al cliente",
  cancel: "Cancelar",
}

const outlineEvents = new Set<WoEvent>(["reject", "mark_waiting_parts"])

export function WorkOrderDetailPage() {
  const { ucode } = useParams()
  const qc = useQueryClient()
  const workOrder = useWorkOrder(ucode)
  const transition = useTransitionWorkOrder()
  const update = useUpdateWorkOrder()
  const [quoteOpen, setQuoteOpen] = useState(false)
  const [readyOpen, setReadyOpen] = useState(false)
  const [cancelOpen, setCancelOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [showPin, setShowPin] = useState(false)
  const wo = workOrder.data

  const runTransition = async (event: WoEvent, input: TransitionInput = {}) => {
    if (!ucode) return
    try {
      await transition.mutateAsync({ ucode, event, input })
      toast.success("Orden actualizada")
      setQuoteOpen(false)
      setReadyOpen(false)
      setCancelOpen(false)
    } catch (error) {
      handleMutationError(error, () => qc.invalidateQueries({ queryKey: ["work-orders", ucode] }))
    }
  }

  const onUpdate = async (input: UpdateWorkOrderInput) => {
    if (!ucode) return
    try {
      await update.mutateAsync({ ucode, input })
      toast.success("Orden actualizada")
      setEditOpen(false)
    } catch (error) {
      handleMutationError(error, () => qc.invalidateQueries({ queryKey: ["work-orders", ucode] }))
    }
  }

  if (workOrder.isLoading) {
    return <div className="rounded-md border p-6 text-sm text-muted-foreground">Cargando orden...</div>
  }

  if (!wo) {
    return <div className="rounded-md border p-6 text-sm text-muted-foreground">Orden no encontrada</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <WoStatusBadge status={wo.status} />
          <div>
            <h1 className="text-2xl font-semibold tracking-normal">OT {wo.wo_number}</h1>
            <p className="text-sm text-muted-foreground">Recibido el {formatDate(wo.received_ts)}</p>
          </div>
        </div>
        <TransitionButtons
          allowedEvents={wo.allowed_events}
          isPending={transition.isPending}
          onSimpleTransition={(event) => void runTransition(event)}
          onQuote={() => setQuoteOpen(true)}
          onReady={() => setReadyOpen(true)}
          onCancel={() => setCancelOpen(true)}
        />
      </div>

      <SummaryCard wo={wo} />

      <Tabs defaultValue="details">
        <TabsList className="max-w-full flex-wrap">
          <TabsTrigger value="details">Detalles</TabsTrigger>
          <TabsTrigger value="quote">Presupuesto</TabsTrigger>
          <TabsTrigger value="close">Cierre</TabsTrigger>
          <TabsTrigger value="parts">Repuestos</TabsTrigger>
          <TabsTrigger value="attachments">Adjuntos</TabsTrigger>
          <TabsTrigger value="payments">Pagos</TabsTrigger>
        </TabsList>
        <TabsContent value="details">
          <DetailsCard wo={wo} showPin={showPin} onTogglePin={() => setShowPin((value) => !value)} onEdit={() => setEditOpen(true)} />
        </TabsContent>
        <TabsContent value="quote">
          <QuoteCard wo={wo} />
        </TabsContent>
        <TabsContent value="close">
          <CloseCard wo={wo} />
        </TabsContent>
        <TabsContent value="parts">
          <PlaceholderCard text="Disponible a partir del Milestone 5" />
        </TabsContent>
        <TabsContent value="attachments">
          <PlaceholderCard text="Disponible a partir del Milestone 5" />
        </TabsContent>
        <TabsContent value="payments">
          <PlaceholderCard text="Disponible a partir del Milestone 4" />
        </TabsContent>
      </Tabs>

      <TimelineCard wo={wo} />

      <EditDetailsDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        workOrder={wo}
        onSubmit={onUpdate}
        isSubmitting={update.isPending}
      />
      <QuoteDialog
        open={quoteOpen}
        onOpenChange={setQuoteOpen}
        onSubmit={(input) => runTransition("quote", input)}
        isSubmitting={transition.isPending}
      />
      <ReadyDialog
        open={readyOpen}
        onOpenChange={setReadyOpen}
        onSubmit={(input) => runTransition("mark_ready", input)}
        isSubmitting={transition.isPending}
      />
      <CancelDialog
        open={cancelOpen}
        onOpenChange={setCancelOpen}
        onSubmit={(input) => runTransition("cancel", input)}
        isSubmitting={transition.isPending}
      />
    </div>
  )
}

function TransitionButtons({
  allowedEvents,
  isPending,
  onSimpleTransition,
  onQuote,
  onReady,
  onCancel,
}: {
  allowedEvents: WoEvent[]
  isPending: boolean
  onSimpleTransition: (event: WoEvent) => void
  onQuote: () => void
  onReady: () => void
  onCancel: () => void
}) {
  return (
    <div className="flex flex-wrap gap-2 lg:justify-end">
      {allowedEvents.map((event) => {
        if (event === "quote") {
          return (
            <Button key={event} type="button" onClick={onQuote} disabled={isPending}>
              {eventLabels[event]}
            </Button>
          )
        }
        if (event === "mark_ready") {
          return (
            <Button key={event} type="button" onClick={onReady} disabled={isPending}>
              {eventLabels[event]}
            </Button>
          )
        }
        if (event === "cancel") {
          return (
            <Button key={event} type="button" variant="destructive" onClick={onCancel} disabled={isPending}>
              {eventLabels[event]}
            </Button>
          )
        }
        return (
          <Button
            key={event}
            type="button"
            variant={outlineEvents.has(event) ? "outline" : "default"}
            onClick={() => onSimpleTransition(event)}
            disabled={isPending}
          >
            {eventLabels[event]}
          </Button>
        )
      })}
    </div>
  )
}

function SummaryCard({ wo }: { wo: WorkOrder }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Resumen</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-5 md:grid-cols-3">
        <SummaryBlock title="Cliente">
          <div className="font-medium">{wo.client.name}</div>
          {wo.client.phone ? (
            <a className="text-sm text-primary underline-offset-4 hover:underline" href={`tel:${wo.client.phone}`}>
              {wo.client.phone}
            </a>
          ) : (
            <div className="text-sm text-muted-foreground">Sin teléfono</div>
          )}
          <Link className="block text-sm text-primary underline-offset-4 hover:underline" to={`/clients/${wo.client.ucode}`}>
            Ver cliente
          </Link>
        </SummaryBlock>
        <SummaryBlock title="Dispositivo">
          <div className="font-medium">{deviceName(wo)}</div>
          <div className="text-sm text-muted-foreground">{wo.device.serial_number ?? "Sin serie"}</div>
        </SummaryBlock>
        <SummaryBlock title="Servicio">
          <div className="font-medium">{wo.service_type === "on_site" ? "Domicilio" : "En taller"}</div>
        </SummaryBlock>
      </CardContent>
    </Card>
  )
}

function DetailsCard({
  wo,
  showPin,
  onTogglePin,
  onEdit,
}: {
  wo: WorkOrder
  showPin: boolean
  onTogglePin: () => void
  onEdit: () => void
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Detalles</CardTitle>
        <CardAction>
          <Button type="button" variant="outline" size="sm" onClick={onEdit}>
            <Pencil />
            Editar
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="grid gap-4 md:grid-cols-2">
        <InfoItem label="Problema reportado" value={wo.reported_issue} />
        <InfoItem label="Diagnóstico" value={wo.diagnosis} />
        <InfoItem label="Notas de ingreso" value={wo.intake_notes} />
        <InfoItem label="Accesorios" value={wo.accessories} />
        <div className="space-y-1">
          <div className="text-xs font-medium uppercase text-muted-foreground">PIN / contraseña</div>
          <div className="flex items-center gap-2 text-sm">
            <span>{wo.device_pin ? (showPin ? wo.device_pin : "••••••") : "-"}</span>
            {wo.device_pin ? (
              <Button type="button" variant="ghost" size="sm" onClick={onTogglePin}>
                {showPin ? <EyeOff /> : <Eye />}
                {showPin ? "Ocultar" : "Mostrar"}
              </Button>
            ) : null}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function QuoteCard({ wo }: { wo: WorkOrder }) {
  if (!wo.quote_amount && !wo.quote_sent_ts) {
    return <PlaceholderCard text="Sin presupuesto todavía" />
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>Presupuesto</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-4 md:grid-cols-2">
        <InfoItem label="Monto" value={wo.quote_amount ? `${wo.quote_amount} ${wo.quote_currency}` : undefined} />
        <InfoItem label="Enviado" value={formatDate(wo.quote_sent_ts)} />
        <InfoItem label="Aprobado" value={formatDate(wo.quote_approved_ts)} />
        <InfoItem label="Rechazado" value={formatDate(wo.quote_rejected_ts)} />
      </CardContent>
    </Card>
  )
}

function CloseCard({ wo }: { wo: WorkOrder }) {
  if (!wo.labor_amount && !wo.parts_amount && !wo.final_amount && !wo.ready_ts && !wo.delivered_ts && !wo.cancel_reason) {
    return <PlaceholderCard text="Sin cierre todavía" />
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>Cierre</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-4 md:grid-cols-2">
        <InfoItem label="Mano de obra" value={wo.labor_amount} />
        <InfoItem label="Repuestos" value={wo.parts_amount} />
        <InfoItem label="Total final" value={wo.final_amount} />
        <InfoItem label="Listo" value={formatDate(wo.ready_ts)} />
        <InfoItem label="Entregado" value={formatDate(wo.delivered_ts)} />
        <InfoItem label="Motivo de cancelación" value={wo.cancel_reason} />
      </CardContent>
    </Card>
  )
}

function TimelineCard({ wo }: { wo: WorkOrder }) {
  const rows = timelineRows(wo)
  return (
    <Card>
      <CardHeader>
        <CardTitle>Línea de tiempo</CardTitle>
      </CardHeader>
      <CardContent>
        <ol className="space-y-2">
          {rows.map((row) => (
            <li key={row.label} className="text-sm">
              <span className="font-medium">{formatDate(row.value)}</span>
              <span className="text-muted-foreground"> — {row.label}</span>
            </li>
          ))}
        </ol>
      </CardContent>
    </Card>
  )
}

function EditDetailsDialog({
  open,
  onOpenChange,
  workOrder,
  onSubmit,
  isSubmitting,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  workOrder: WorkOrder
  onSubmit: (input: UpdateWorkOrderInput) => void | Promise<void>
  isSubmitting: boolean
}) {
  const [reportedIssue, setReportedIssue] = useState(workOrder.reported_issue)
  const [diagnosis, setDiagnosis] = useState(workOrder.diagnosis ?? "")
  const [intakeNotes, setIntakeNotes] = useState(workOrder.intake_notes ?? "")
  const [accessories, setAccessories] = useState(workOrder.accessories ?? "")
  const [devicePin, setDevicePin] = useState(workOrder.device_pin ?? "")

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Editar detalles"
      onSubmit={() =>
        onSubmit({
          reported_issue: reportedIssue.trim(),
          diagnosis: emptyToUndefined(diagnosis),
          intake_notes: emptyToUndefined(intakeNotes),
          accessories: emptyToUndefined(accessories),
          device_pin: emptyToUndefined(devicePin),
        })
      }
      isSubmitting={isSubmitting}
    >
      <Field label="Problema reportado">
        <Textarea value={reportedIssue} onChange={(event) => setReportedIssue(event.target.value)} maxLength={2000} />
      </Field>
      <Field label="Diagnóstico">
        <Textarea value={diagnosis} onChange={(event) => setDiagnosis(event.target.value)} maxLength={4000} />
      </Field>
      <Field label="Notas de ingreso">
        <Textarea value={intakeNotes} onChange={(event) => setIntakeNotes(event.target.value)} maxLength={4000} />
      </Field>
      <Field label="Accesorios">
        <Textarea value={accessories} onChange={(event) => setAccessories(event.target.value)} maxLength={2000} />
      </Field>
      <Field label="PIN / contraseña">
        <Input value={devicePin} onChange={(event) => setDevicePin(event.target.value)} maxLength={64} />
      </Field>
    </FormDialog>
  )
}

function SummaryBlock({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <div className="text-xs font-medium uppercase text-muted-foreground">{title}</div>
      {children}
    </div>
  )
}

function InfoItem({ label, value }: { label: string; value?: string }) {
  return (
    <div className="space-y-1">
      <div className="text-xs font-medium uppercase text-muted-foreground">{label}</div>
      <div className="whitespace-pre-wrap text-sm">{value || "-"}</div>
    </div>
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

function PlaceholderCard({ text }: { text: string }) {
  return (
    <Card>
      <CardContent className="py-8 text-sm text-muted-foreground">{text}</CardContent>
    </Card>
  )
}

function handleMutationError(error: unknown, onInvalidTransition: () => void) {
  if (error instanceof AxiosError) {
    const code = (error.response?.data as { error?: string } | undefined)?.error
    if (error.response?.status === 409 && code === "invalid_transition") {
      toast.error("La orden cambió de estado, recargá la página")
      void onInvalidTransition()
      return
    }
  }
  toast.error("No se pudo actualizar la orden")
}

function timelineRows(wo: WorkOrder) {
  return [
    { label: "Orden recibida", value: wo.received_ts },
    { label: "Reparación iniciada", value: wo.started_ts },
    { label: "Presupuesto enviado", value: wo.quote_sent_ts },
    { label: "Presupuesto aprobado", value: wo.quote_approved_ts },
    { label: "Presupuesto rechazado", value: wo.quote_rejected_ts },
    { label: "Orden lista", value: wo.ready_ts },
    { label: "Orden entregada", value: wo.delivered_ts },
    { label: "Orden cancelada", value: wo.cancelled_ts },
  ].filter((row): row is { label: string; value: string } => Boolean(row.value))
}

function deviceName(wo: WorkOrder) {
  return [wo.device.brand_name, wo.device.model_name, wo.device.article_type_name].filter(Boolean).join(" · ")
}

function formatDate(value?: string) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("es-AR", { dateStyle: "short", timeStyle: "short" }).format(new Date(value))
}

function emptyToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}
