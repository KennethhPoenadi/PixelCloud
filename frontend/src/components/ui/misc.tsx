import type { InputHTMLAttributes, ReactNode } from 'react'
import { cn } from '@/lib/cn'

export function Skeleton({ className }: { className?: string }) {
  return (
    <div className={cn('animate-pulse rounded-[var(--radius-control)] bg-surface-2', className)} />
  )
}

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div
      className={cn('rounded-[var(--radius-card)] border border-border bg-surface p-6', className)}
    >
      {children}
    </div>
  )
}

interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string
  error?: string
}

export function Field({ label, error, id, className, ...props }: FieldProps) {
  const inputId = id ?? props.name
  return (
    <div className="space-y-1.5">
      <label htmlFor={inputId} className="text-xs font-medium text-muted">
        {label}
      </label>
      <input
        id={inputId}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${inputId}-error` : undefined}
        className={cn(
          'h-10 w-full rounded-[var(--radius-control)] border border-border bg-surface-2 px-3 text-sm text-text placeholder:text-muted focus:border-accent focus:outline-none',
          error && 'border-accent-r',
          className,
        )}
        {...props}
      />
      {error && (
        <p id={`${inputId}-error`} className="text-xs text-accent-r">
          {error}
        </p>
      )}
    </div>
  )
}
