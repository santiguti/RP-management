package i18n

var TransactionType = map[string]string{
	"income":  "Ingreso",
	"expense": "Egreso",
}

var Category = map[string]string{
	"wo_payment":    "Pago de OT",
	"wo_deposit":    "Seña de cliente",
	"part_purchase": "Compra de repuesto",
	"supplies":      "Insumos",
	"rent":          "Alquiler",
	"utilities":     "Servicios",
	"salary":        "Sueldos",
	"taxes":         "Impuestos",
	"food":          "Comida",
	"transport":     "Transporte",
	"other_income":  "Otros ingresos",
	"other_expense": "Otros egresos",
}

var PaymentMethod = map[string]string{
	"cash":        "Efectivo",
	"transfer":    "Transferencia",
	"card":        "Tarjeta",
	"mercadopago": "MercadoPago",
	"other":       "Otro",
}

var WorkOrderStatus = map[string]string{
	"received":      "Recibido",
	"diagnosing":    "Diagnóstico",
	"quoted":        "Presupuestado",
	"approved":      "Aprobado",
	"rejected":      "Rechazado",
	"in_repair":     "En reparación",
	"waiting_parts": "Esperando repuestos",
	"ready":         "Listo",
	"delivered":     "Entregado",
	"cancelled":     "Cancelado",
}

var CounterpartyType = map[string]string{
	"client":   "Cliente",
	"supplier": "Proveedor",
	"none":     "Sin contraparte",
}

var ServiceType = map[string]string{
	"in_shop": "En taller",
	"on_site": "A domicilio",
}

func Lookup(m map[string]string, k string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return k
}
