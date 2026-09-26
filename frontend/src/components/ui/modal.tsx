import * as Dialog from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'

interface ModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  children: ReactNode
  /** "sheet" slides up from the bottom (mobile adjustments panel). */
  variant?: 'center' | 'sheet'
}

export function Modal({
  open,
  onOpenChange,
  title,
  description,
  children,
  variant = 'center',
}: ModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content
          className={cn(
            'fixed z-50 border border-border bg-surface p-6 shadow-xl focus:outline-none',
            variant === 'center' &&
              'top-1/2 left-1/2 w-[calc(100vw-32px)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-[var(--radius-modal)]',
            variant === 'sheet' &&
              'inset-x-0 bottom-0 max-h-[75vh] overflow-y-auto rounded-t-[var(--radius-modal)]',
          )}
        >
          <div className="mb-4 flex items-start justify-between gap-4">
            <div>
              <Dialog.Title className="font-display text-lg font-semibold">{title}</Dialog.Title>
              {description ? (
                <Dialog.Description className="mt-1 text-sm text-muted">
                  {description}
                </Dialog.Description>
              ) : (
                <Dialog.Description className="sr-only">{title}</Dialog.Description>
              )}
            </div>
            <Dialog.Close
              className="rounded-md p-1 text-muted hover:bg-surface-2 hover:text-text"
              aria-label="Tutup"
            >
              <X className="size-4" />
            </Dialog.Close>
          </div>
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
