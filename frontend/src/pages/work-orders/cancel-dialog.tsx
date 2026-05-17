import { useState } from "react"

import type { TransitionInput } from "@/api/work-orders"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

type CancelDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (input: TransitionInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function CancelDialog({ open, onOpenChange, onSubmit, isSubmitting = false }: CancelDialogProps) {
  const [cancelReason, setCancelReason] = useState("")

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    await onSubmit({ cancel_reason: emptyToUndefined(cancelReason) })
    setCancelReason("")
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form className="space-y-5" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>Cancelar orden</DialogTitle>
            <DialogDescription>Esta acción deja la orden como cancelada.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label>Motivo (opcional)</Label>
            <Textarea
              value={cancelReason}
              onChange={(event) => setCancelReason(event.target.value)}
              maxLength={2000}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
              Volver
            </Button>
            <Button type="submit" variant="destructive" disabled={isSubmitting}>
              Cancelar orden
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function emptyToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}
