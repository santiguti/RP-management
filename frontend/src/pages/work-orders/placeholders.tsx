import { Link, useParams } from "react-router-dom"
import { Plus } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"

export function WorkOrdersListPlaceholder() {
  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Órdenes de trabajo</h1>
          <p className="text-sm text-muted-foreground">La vista de listado llega en el próximo paso.</p>
        </div>
        <Button asChild>
          <Link to="/work-orders/new">
            <Plus />
            Nueva orden
          </Link>
        </Button>
      </div>
      <Card>
        <CardContent className="py-8 text-sm text-muted-foreground">
          Usá Nueva orden para registrar un ingreso.
        </CardContent>
      </Card>
    </div>
  )
}

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
