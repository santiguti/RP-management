import { useState } from "react"
import { toast } from "sonner"

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
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

const moneyPattern = /^\d+(\.\d{1,2})?$/

type ReadyDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (input: TransitionInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function ReadyDialog({ open, onOpenChange, onSubmit, isSubmitting = false }: ReadyDialogProps) {
  const [laborAmount, setLaborAmount] = useState("")
  const [partsAmount, setPartsAmount] = useState("")
  const [finalAmount, setFinalAmount] = useState("")

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
            <DialogDescription>Podés completar uno, varios o ningún monto.</DialogDescription>
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
