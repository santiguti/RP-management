import { Download } from "lucide-react"

import { Button } from "@/components/ui/button"

export function DownloadCsvButton({ href }: { href: string }) {
  return (
    <Button asChild variant="outline">
      <a href={href} download>
        <Download />
        Descargar CSV
      </a>
    </Button>
  )
}
