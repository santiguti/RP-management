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
import { Textarea } from "@/components/ui/textarea"

const moneyPattern = /^\d+(\.\d{1,2})?$/

type QuoteDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (input: TransitionInput) => void | Promise<void>
  isSubmitting?: boolean
}

export function QuoteDialog({ open, onOpenChange, onSubmit, isSubmitting = false }: QuoteDialogProps) {
  const [quoteAmount, setQuoteAmount] = useState("")
  const [quoteCurrency, setQuoteCurrency] = useState("ARS")
  const [diagnosis, setDiagnosis] = useState("")

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!moneyPattern.test(quoteAmount.trim())) {
      toast.error("Ingresá un monto válido")
      return
    }
    await onSubmit({
      quote_amount: quoteAmount.trim(),
      quote_currency: quoteCurrency.trim().toUpperCase() || "ARS",
      diagnosis: emptyToUndefined(diagnosis),
    })
    setQuoteAmount("")
    setQuoteCurrency("ARS")
    setDiagnosis("")
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form className="space-y-5" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>Cargar presupuesto</DialogTitle>
            <DialogDescription>Registrá el monto informado al cliente.</DialogDescription>
          </DialogHeader>
          <Field label="Monto">
            <Input value={quoteAmount} onChange={(event) => setQuoteAmount(event.target.value)} placeholder="15000.00" />
          </Field>
          <Field label="Moneda">
            <Input
              value={quoteCurrency}
              onChange={(event) => setQuoteCurrency(event.target.value.toUpperCase())}
              maxLength={3}
            />
          </Field>
          <Field label="Diagnóstico">
            <Textarea value={diagnosis} onChange={(event) => setDiagnosis(event.target.value)} maxLength={4000} />
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

function emptyToUndefined(value: string) {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}
