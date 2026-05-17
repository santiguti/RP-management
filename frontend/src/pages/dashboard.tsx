import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useAuth } from "@/hooks/use-auth"

export function DashboardPage() {
  const { data: user } = useAuth()

  return (
    <Card className="max-w-2xl">
      <CardHeader>
        <CardTitle>¡Hola, {user?.full_name ?? user?.username}!</CardTitle>
      </CardHeader>
      <CardContent>
        <span className="text-sm text-muted-foreground">Rol: {user?.role}</span>
      </CardContent>
    </Card>
  )
}
