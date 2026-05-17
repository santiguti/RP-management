import { useParams } from "react-router-dom"

import { Card, CardContent } from "@/components/ui/card"

export function WorkOrderDetailPlaceholder() {
  const { ucode } = useParams()
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-normal">Detalle de orden</h1>
        <p className="text-sm text-muted-foreground">{ucode}</p>
      </div>
      <Card>
        <CardContent className="py-8 text-sm text-muted-foreground">
          El detalle completo llega en el paso 9.
        </CardContent>
      </Card>
    </div>
  )
}
