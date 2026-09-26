import { ImageUp } from 'lucide-react'
import { useRef, useState, type DragEvent } from 'react'
import { cn } from '@/lib/cn'

interface DropzoneProps {
  onFiles: (files: File[]) => void
  busy?: boolean
  hint?: string
  className?: string
}

export function Dropzone({ onFiles, busy, hint, className }: DropzoneProps) {
  const input = useRef<HTMLInputElement>(null)
  const [over, setOver] = useState(false)

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setOver(false)
    const files = Array.from(e.dataTransfer.files)
    if (files.length) onFiles(files)
  }

  return (
    <button
      type="button"
      onClick={() => input.current?.click()}
      onDragOver={(e) => {
        e.preventDefault()
        setOver(true)
      }}
      onDragLeave={() => setOver(false)}
      onDrop={onDrop}
      disabled={busy}
      className={cn(
        'flex w-full flex-col items-center justify-center gap-2 rounded-[var(--radius-card)] border-2 border-dashed border-border bg-surface px-6 py-10 text-center transition-colors duration-150',
        over ? 'border-accent bg-accent/5' : 'hover:border-muted',
        busy && 'opacity-60',
        className,
      )}
    >
      <ImageUp className="size-8 text-accent" aria-hidden />
      <span className="font-medium">
        {busy ? 'Mengunggah…' : 'Tarik & lepas foto di sini, atau klik untuk memilih'}
      </span>
      <span className="text-xs text-muted">{hint ?? 'JPEG, PNG, atau WebP'}</span>
      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        multiple
        hidden
        data-testid="file-input"
        onChange={(e) => {
          const files = Array.from(e.target.files ?? [])
          if (files.length) onFiles(files)
          e.target.value = ''
        }}
      />
    </button>
  )
}
