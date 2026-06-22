import { useRef, useState } from "react"
import { ImagePlus, Trash2 } from "lucide-react"
import { toast } from "sonner"

import type { Attachment, AttachmentPhase } from "@/api/attachments"
import type { WorkOrder } from "@/api/work-orders"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog"
import { useAttachments, useDeleteAttachment, useUploadAttachment } from "@/hooks/use-attachments"

const phases: { value: AttachmentPhase; label: string }[] = [
  { value: "intake", label: "Recepción" },
  { value: "diagnosis", label: "Diagnóstico" },
  { value: "during_repair", label: "Durante reparación" },
  { value: "delivery", label: "Entrega" },
]

const allowedMimes = new Set(["image/jpeg", "image/png", "image/webp"])
const maxUploadBytes = 10 * 1024 * 1024

type UploadProgress = { name: string; state: "subiendo" | "listo" | "error" }

export function AttachmentsTab({ workOrder }: { workOrder: WorkOrder }) {
  const attachments = useAttachments(workOrder.ucode)
  const upload = useUploadAttachment()
  const remove = useDeleteAttachment()
  const inputRef = useRef<HTMLInputElement>(null)
  const [phase, setPhase] = useState<AttachmentPhase>("intake")
  const [progress, setProgress] = useState<UploadProgress[]>([])
  const [selected, setSelected] = useState<Attachment | null>(null)

  const uploadFiles = async (files: File[]) => {
    const next: UploadProgress[] = files.map((file) => ({ name: file.name, state: "subiendo" }))
    setProgress(next)
    for (let index = 0; index < files.length; index += 1) {
      const file = files[index]
      if (!allowedMimes.has(file.type) || file.size > maxUploadBytes) {
        next[index] = { name: file.name, state: "error" }
        setProgress([...next])
        toast.error("No se pudo subir la imagen")
        continue
      }
      try {
        await upload.mutateAsync({ workOrderUcode: workOrder.ucode, file, phase })
        next[index] = { name: file.name, state: "listo" }
        toast.success("Imagen subida")
      } catch {
        next[index] = { name: file.name, state: "error" }
        toast.error("No se pudo subir la imagen")
      }
      setProgress([...next])
    }
  }

  const onDelete = async (attachment: Attachment) => {
    if (!window.confirm("¿Eliminar imagen?\nEsta acción no se puede deshacer.")) return
    try {
      await remove.mutateAsync({ workOrderUcode: workOrder.ucode, attachmentUcode: attachment.ucode })
      toast.success("Imagen eliminada")
      if (selected?.ucode === attachment.ucode) setSelected(null)
    } catch {
      toast.error("No se pudo eliminar la imagen")
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Adjuntos</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <label className="grid gap-1 text-sm font-medium">
            Etapa
            <select
              className="border-input bg-background h-9 rounded-md border px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              value={phase}
              onChange={(event) => setPhase(event.target.value as AttachmentPhase)}
            >
              {phases.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
            </select>
          </label>
          <input
            ref={inputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            multiple
            className="sr-only"
            onChange={(event) => {
              void uploadFiles(Array.from(event.target.files ?? []))
              event.target.value = ""
            }}
          />
          <button
            type="button"
            className="flex min-h-28 flex-1 flex-col items-center justify-center gap-2 rounded-md border border-dashed px-4 text-sm text-muted-foreground hover:border-primary hover:text-foreground"
            onClick={() => inputRef.current?.click()}
            onDragOver={(event) => event.preventDefault()}
            onDrop={(event) => {
              event.preventDefault()
              void uploadFiles(Array.from(event.dataTransfer.files))
            }}
          >
            <ImagePlus />
            Soltá imágenes acá o hacé clic para seleccionar
          </button>
        </div>

        {progress.length > 0 ? (
          <ul className="space-y-1 text-sm text-muted-foreground">
            {progress.map((item) => <li key={item.name}>{item.name}: {item.state === "subiendo" ? "Subiendo" : item.state === "listo" ? "Listo" : "Error"}</li>)}
          </ul>
        ) : null}

        {attachments.isLoading ? <div className="text-sm text-muted-foreground">Cargando adjuntos...</div> : null}
        {!attachments.isLoading && (attachments.data?.length ?? 0) === 0 ? <div className="py-6 text-sm text-muted-foreground">Sin adjuntos cargados</div> : null}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {(attachments.data ?? []).map((attachment) => (
            <div key={attachment.ucode} className="overflow-hidden rounded-md border bg-muted/20">
              <button type="button" className="block aspect-video w-full bg-muted" onClick={() => setSelected(attachment)}>
                <img
                  src={attachmentURL(workOrder.ucode, attachment.ucode)}
                  alt={attachment.original_filename}
                  className="size-full object-cover"
                />
              </button>
              <div className="flex items-center justify-between gap-2 border-t px-3 py-2">
                <span className="truncate text-sm font-medium">{phaseLabel(attachment.phase)}</span>
                <Button type="button" variant="ghost" size="icon" title="Eliminar imagen" onClick={() => void onDelete(attachment)}>
                  <Trash2 />
                </Button>
              </div>
            </div>
          ))}
        </div>
      </CardContent>

      <Dialog open={Boolean(selected)} onOpenChange={(open) => { if (!open) setSelected(null) }}>
        <DialogContent className="max-w-4xl p-3" showCloseButton>
          <DialogTitle className="sr-only">Imagen adjunta</DialogTitle>
          {selected ? <img src={attachmentURL(workOrder.ucode, selected.ucode)} alt={selected.original_filename} className="max-h-[80vh] w-full object-contain" /> : null}
        </DialogContent>
      </Dialog>
    </Card>
  )
}

function attachmentURL(workOrderUcode: string, attachmentUcode: string) {
  return `/api/v1/work-orders/${workOrderUcode}/attachments/${attachmentUcode}`
}

function phaseLabel(phase: AttachmentPhase) {
  return phases.find((item) => item.value === phase)?.label ?? phase
}
