import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { login as loginCall, logout as logoutCall, me } from "@/api/auth"

export function useAuth() {
  return useQuery({
    queryKey: ["me"],
    queryFn: me,
    retry: false,
    staleTime: 5 * 60 * 1000,
  })
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (vars: { username: string; password: string }) =>
      loginCall(vars.username, vars.password),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["me"] })
    },
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: logoutCall,
    onSuccess: () => {
      qc.setQueryData(["me"], null)
      qc.invalidateQueries({ queryKey: ["me"] })
    },
  })
}
