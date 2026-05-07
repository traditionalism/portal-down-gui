import * as React from "react";
import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

interface CheckboxProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "type" | "onChange"> {
  checked?: boolean;
  onCheckedChange?: (checked: boolean) => void;
}

export function Checkbox({ className, checked, onCheckedChange, disabled, ...props }: CheckboxProps) {
  return (
    <label
      className={cn(
        "relative inline-flex h-4 w-4 cursor-pointer items-center justify-center rounded-[4px] border border-input bg-background transition-colors",
        "has-[input:checked]:bg-primary has-[input:checked]:border-primary has-[input:checked]:text-primary-foreground",
        "has-[input:focus-visible]:ring-ring/50 has-[input:focus-visible]:ring-[3px]",
        disabled && "cursor-not-allowed opacity-50",
        className,
      )}
    >
      <input
        type="checkbox"
        className="absolute inset-0 cursor-pointer opacity-0"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onCheckedChange?.(e.target.checked)}
        {...props}
      />
      {checked && <Check className="size-3 stroke-[3]" />}
    </label>
  );
}
