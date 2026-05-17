import * as React from "react"
import { keepPreviousData, useQuery } from "@tanstack/react-query"
import { Check, ChevronsUpDown, X } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { useDebounce } from "@/hooks/use-debounce"
import { cn } from "@/lib/utils"

type EntityComboboxProps<T> = {
  queryKey: readonly unknown[]
  value: string | null
  onChange: (ucode: string | null) => void
  fetchOptions: (q: string) => Promise<T[]>
  getKey: (option: T) => string
  getLabel: (option: T) => string
  placeholder?: string
  emptyMessage?: string
  disabled?: boolean
  actionLabel?: string
  onAction?: () => void
}

export function EntityCombobox<T>({
  queryKey,
  value,
  onChange,
  fetchOptions,
  getKey,
  getLabel,
  placeholder = "Seleccionar...",
  emptyMessage = "Sin opciones",
  disabled = false,
  actionLabel,
  onAction,
}: EntityComboboxProps<T>) {
  const [open, setOpen] = React.useState(false)
  const [q, setQ] = React.useState("")
  const debouncedQ = useDebounce(q, 250)

  const { data = [], isFetching } = useQuery({
    queryKey: ["entity-combobox", ...queryKey, debouncedQ],
    queryFn: () => fetchOptions(debouncedQ),
    placeholderData: keepPreviousData,
  })

  const selected = React.useMemo(
    () => data.find((option) => getKey(option) === value) ?? null,
    [data, getKey, value],
  )
  const selectedLabel = selected ? getLabel(selected) : ""

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-full justify-between"
          disabled={disabled}
        >
          <span className={cn("truncate", !selectedLabel && "text-muted-foreground")}>
            {selectedLabel || placeholder}
          </span>
          <ChevronsUpDown className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput value={q} onValueChange={setQ} placeholder={placeholder} />
          <CommandList>
            <CommandEmpty>{isFetching ? "Buscando..." : emptyMessage}</CommandEmpty>
            <CommandGroup>
              {actionLabel && onAction ? (
                <CommandItem
                  value="__action__"
                  onSelect={() => {
                    onAction()
                    setOpen(false)
                  }}
                >
                  {actionLabel}
                </CommandItem>
              ) : null}
              {value ? (
                <CommandItem
                  value="__clear__"
                  onSelect={() => {
                    onChange(null)
                    setOpen(false)
                  }}
                >
                  <X />
                  Limpiar selección
                </CommandItem>
              ) : null}
              {data.map((option) => {
                const key = getKey(option)
                return (
                  <CommandItem
                    key={key}
                    value={key}
                    onSelect={() => {
                      onChange(key)
                      setOpen(false)
                    }}
                  >
                    <Check className={cn(key === value ? "opacity-100" : "opacity-0")} />
                    {getLabel(option)}
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
