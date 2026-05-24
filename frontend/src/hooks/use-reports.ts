import { useQuery } from "@tanstack/react-query"

import {
  getBalance,
  getDashboard,
  getPnl,
  type DateRangeParams,
  type PnLParams,
} from "@/api/reports"

export function useBalance(params: DateRangeParams = {}) {
  return useQuery({
    queryKey: ["reports", "balance", params],
    queryFn: () => getBalance(params),
    staleTime: 5 * 60 * 1000,
  })
}

export function usePnl(params: PnLParams = {}) {
  return useQuery({
    queryKey: ["reports", "pnl", params],
    queryFn: () => getPnl(params),
    staleTime: 5 * 60 * 1000,
  })
}

export function useDashboard() {
  return useQuery({
    queryKey: ["reports", "dashboard"],
    queryFn: getDashboard,
    staleTime: 60 * 1000,
  })
}

export type { DateRangeParams, PnLParams }
