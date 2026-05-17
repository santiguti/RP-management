import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { Toaster } from "@/components/ui/sonner"

import { LoginPage } from "@/pages/login"
import { DashboardPage } from "@/pages/dashboard"
import { ProtectedRoute } from "@/components/protected-route"
import { AppLayout } from "@/components/layout"
import { ClientsListPage } from "@/pages/clients/list"
import { ClientDetailPage } from "@/pages/clients/detail"

const qc = new QueryClient()

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
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
      <Toaster richColors closeButton />
    </QueryClientProvider>
  )
}
