import { useEffect, useState } from "react"
import { toast } from "sonner"

import type { TransitionInput, WorkOrder } from "@/api/work-orders"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { centsToMoney, moneyToCents } from "@/lib/money"

const moneyPattern = /^\d+(\.\d{1,2})?$/

type ReadyDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (input: TransitionInput) => void | Promise<void>
  isSubmitting?: boolean
  workOrder: Pick<WorkOrder, "quote_amount" | "parts_amount">
}

export function ReadyDialog({ open, onOpenChange, onSubmit, isSubmitting = false, workOrder }: ReadyDialogProps) {
  const [laborAmount, setLaborAmount] = useState("")
  const [partsAmount, setPartsAmount] = useState("")
  const [finalAmount, setFinalAmount] = useState("")

  // Prefill on open: Repuestos = sale price of the parts used on the WO,
  // Total final = the client-approved quote, Mano de obra = the difference.
  // Everything stays editable — these are starting values, not locks.
  useEffect(() => {
    if (!open) return
    const partsCents = moneyToCents(workOrder.parts_amount) ?? 0
    const quoteCents = moneyToCents(workOrder.quote_amount)
    setPartsAmount(centsToMoney(partsCents))
    if (quoteCents !== null) {
      setFinalAmount(centsToMoney(quoteCents))
      setLaborAmount(centsToMoney(Math.max(quoteCents - partsCents, 0)))
    } else {
      setLaborAmount("")
      setFinalAmount(partsCents > 0 ? centsToMoney(partsCents) : "")
    }
  }, [open, workOrder.parts_amount, workOrder.quote_amount])

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (![laborAmount, partsAmount, finalAmount].every(validOptionalMoney)) {
      toast.error("Ingresá montos válidos")
      return
    }
    await onSubmit({
      labor_amount: emptyToUndefined(laborAmount),
      parts_amount: emptyToUndefined(partsAmount),
      final_amount: emptyToUndefined(finalAmount),
    })
    setLaborAmount("")
    setPartsAmount("")
    setFinalAmount("")
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form className="space-y-5" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>Cerrar reparación</DialogTitle>
            <DialogDescription>
              Montos precargados según el presupuesto aprobado y los repuestos usados — ajustalos si hace falta.
            </DialogDescription>
          </DialogHeader>
          <Field label="Mano de obra">
            <Input value={laborAmount} onChange={(event) => setLaborAmount(event.target.value)} placeholder="10000.00" />
          </Field>
          <Field label="Repuestos">
            <Input value={partsAmount} onChange={(event) => setPartsAmount(event.target.value)} placeholder="5000.00" />
          </Field>
          <Field label="Total final">
            <Input value={finalAmount} onChange={(event) => setFinalAmount(event.target.value)} placeholder="15000.00" />
          </Field>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
              Cancelar
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
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

function validOptionalMoney(value: string) {
  const trimmed = value.trim()
  return trimmed === "" || moneyPattern.test(trimmed)
}

function emptyToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}
