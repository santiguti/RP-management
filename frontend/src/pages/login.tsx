import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useLogin } from "@/hooks/use-auth"

const schema = z.object({
  username: z.string().min(1, "Requerido"),
  password: z.string().min(1, "Requerido"),
})
type Form = z.infer<typeof schema>

export function LoginPage() {
  const { register, handleSubmit, formState: { errors, isSubmitting } } =
    useForm<Form>({ resolver: zodResolver(schema) })
  const login = useLogin()
  const nav = useNavigate()

  const onSubmit = async (data: Form) => {
    try {
      await login.mutateAsync(data)
      nav("/", { replace: true })
    } catch {
      toast.error("Credenciales inválidas")
    }
  }

  return (
    <div className="grid min-h-dvh place-items-center bg-slate-50 p-4">
      <Card className="w-full max-w-sm">
        <CardHeader><CardTitle>Iniciar sesión</CardTitle></CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-1">
              <Label htmlFor="username">Usuario</Label>
              <Input id="username" autoComplete="username" {...register("username")} />
              {errors.username && <p className="text-xs text-red-600">{errors.username.message}</p>}
            </div>
            <div className="space-y-1">
              <Label htmlFor="password">Contraseña</Label>
              <Input id="password" type="password" autoComplete="current-password" {...register("password")} />
              {errors.password && <p className="text-xs text-red-600">{errors.password.message}</p>}
            </div>
            <Button type="submit" className="w-full" disabled={isSubmitting}>Entrar</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
