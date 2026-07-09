import { lazy, Suspense } from "react"
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { Toaster } from "@/components/ui/sonner"

import { LoginPage } from "@/pages/login"
import { DashboardPage } from "@/pages/dashboard"
import { ProtectedRoute } from "@/components/protected-route"
import { AppLayout } from "@/components/layout"
import { ClientsListPage } from "@/pages/clients/list"
import { ClientDetailPage } from "@/pages/clients/detail"
import { RequireOwner } from "@/components/require-owner"
import { LookupsPage } from "@/pages/settings/lookups"
import { IntakeWorkOrderPage } from "@/pages/work-orders/intake"
import { WorkOrdersListPage } from "@/pages/work-orders/list"
import { WorkOrderDetailPage } from "@/pages/work-orders/detail"
import { TransactionsListPage } from "@/pages/transactions/list"
import { SuppliersListPage } from "@/pages/suppliers/list"
import { RecurringExpensesPage } from "@/pages/settings/recurring-expenses"
import { PartsListPage } from "@/pages/parts/list"
import { PartDetailPage } from "@/pages/parts/detail"
import { ImportPage } from "@/pages/import"
import { AuditLogPage } from "@/pages/settings/audit-log"

const qc = new QueryClient()
const ReportsPage = lazy(() =>
  import("@/pages/reports").then((m) => ({ default: m.ReportsPage })),
)

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/" element={<ProtectedRoute><AppLayout /></ProtectedRoute>}>
            <Route index element={<DashboardPage />} />
            <Route path="clients" element={<ClientsListPage />} />
            <Route path="clients/:ucode" element={<ClientDetailPage />} />
            <Route path="work-orders" element={<WorkOrdersListPage />} />
            <Route path="work-orders/new" element={<IntakeWorkOrderPage />} />
            <Route path="work-orders/:ucode" element={<WorkOrderDetailPage />} />
            <Route path="parts" element={<PartsListPage />} />
            <Route path="parts/new" element={<PartsListPage createOnMount />} />
            <Route path="parts/:ucode" element={<PartDetailPage />} />
            <Route path="transactions" element={<TransactionsListPage />} />
            <Route path="suppliers" element={<SuppliersListPage />} />
            <Route
              path="reports"
              element={
                <Suspense
                  fallback={<div className="text-sm text-muted-foreground">Cargando reportes...</div>}
                >
                  <ReportsPage />
                </Suspense>
              }
            />
            <Route
              path="import"
              element={
                <RequireOwner>
                  <ImportPage />
                </RequireOwner>
              }
            />
            <Route
              path="settings/lookups"
              element={
                <RequireOwner>
                  <LookupsPage />
                </RequireOwner>
              }
            />
            <Route
              path="settings/recurring-expenses"
              element={
                <RequireOwner>
                  <RecurringExpensesPage />
                </RequireOwner>
              }
            />
            <Route
              path="settings/audit-log"
              element={
                <RequireOwner>
                  <AuditLogPage />
                </RequireOwner>
              }
            />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
      <Toaster richColors closeButton />
    </QueryClientProvider>
  )
}
