import { useEffect, useRef, useState } from 'react'
import { cn } from '../lib/classnames'

export interface SelectOption {
  value: string
  label: string
}

interface SelectProps {
  value: string
  options: SelectOption[]
  onChange: (value: string) => void
  className?: string
}

export function Select({ value, options, onChange, className }: SelectProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    const keyHandler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    document.addEventListener('keydown', keyHandler)
    return () => {
      document.removeEventListener('mousedown', handler)
      document.removeEventListener('keydown', keyHandler)
    }
  }, [open])

  const selectedLabel = options.find((o) => o.value === value)?.label ?? value

  return (
    <div ref={containerRef} className={cn('relative', className)}>
      <button
        type="button"
        className={cn(
          'field flex items-center justify-between text-left',
          open && 'border-primary',
        )}
        onClick={() => setOpen((v) => !v)}
      >
        <span className="truncate">{selectedLabel || ' '}</span>
        <Chevron open={open} />
      </button>
      {open && (
        <div className="absolute left-0 top-full z-50 mt-1 max-h-52 w-full overflow-y-auto rounded-md border border-border/70 bg-surface p-1 surface-shadow animate-slide-in">
          {options.map((opt) => (
            <button
              key={opt.value}
              type="button"
              className={cn(
                'block w-full rounded px-2 py-1.5 text-left text-xs transition-colors',
                opt.value === value
                  ? 'bg-primary/14 text-text font-semibold'
                  : 'text-text-2 hover:bg-surface-3 hover:text-text',
              )}
              onClick={() => {
                onChange(opt.value)
                setOpen(false)
              }}
            >
              {opt.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      className={cn('ml-1 shrink-0 text-text-3 transition-transform', open && 'rotate-180')}
      aria-hidden
    >
      <path d="m6 9 6 6 6-6" />
    </svg>
  )
}
