import * as React from "react"

import { cn } from "@/lib/utils"

type RadioGroupProps = React.ComponentProps<"div"> & {
  value: string
  onValueChange: (value: string) => void
}

function RadioGroup({ className, value, onValueChange, children, ...props }: RadioGroupProps) {
  return (
    <div role="radiogroup" className={cn("grid gap-2", className)} {...props}>
      {React.Children.map(children, (child) => {
        if (!React.isValidElement<RadioGroupItemProps>(child)) return child
        return React.cloneElement(child, {
          checked: child.props.value === value,
          onCheckedChange: () => onValueChange(child.props.value),
        })
      })}
    </div>
  )
}

type RadioGroupItemProps = Omit<React.ComponentProps<"button">, "value" | "onChange"> & {
  value: string
  checked?: boolean
  onCheckedChange?: () => void
}

function RadioGroupItem({
  className,
  value,
  checked = false,
  onCheckedChange,
  ...props
}: RadioGroupItemProps) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      data-value={value}
      className={cn(
        "border-input text-primary focus-visible:border-ring focus-visible:ring-ring/50 flex size-4 shrink-0 items-center justify-center rounded-full border shadow-xs outline-none transition-[color,box-shadow] focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50",
        className
      )}
      onClick={onCheckedChange}
      {...props}
    >
      {checked ? <span className="bg-primary size-2 rounded-full" /> : null}
    </button>
  )
}

export { RadioGroup, RadioGroupItem }
