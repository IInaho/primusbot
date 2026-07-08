// ActivityRow: one tool call with a compact call header and an explicit result body.
import { memo, useCallback, useId, useMemo, useRef } from 'react'
import type { ToolStep } from '../../types/events'
import { useScrollContainer } from '../MessageList'
import { isUnifiedDiffContent } from '../../lib/diffFormat'
import { cn } from '../../lib/classnames'
import { compactArgs, editSummary, isMCPTool, pathFromArgs, prettyTool, toolDetail } from './helpers'
import { UnifiedDiff } from './UnifiedDiff'

interface ActivityRowProps {
  step: ToolStep
  toggleStep: (stepId: string) => void
}

export const ActivityRow = memo(function ActivityRow({ step, toggleStep }: ActivityRowProps) {
  const rowRef = useRef<HTMLDivElement>(null)
  const scrollRef = useScrollContainer()
  const bodyId = useId()
  const expanded = !step.collapsed
  const content = contentForStep(step)
  const canExpand = !!content

  const handleToggle = useCallback(() => {
    if (!canExpand) return
    const rowEl = rowRef.current
    const scrollEl = scrollRef.current
    let offsetBefore = 0
    if (rowEl && scrollEl) {
      offsetBefore = rowEl.getBoundingClientRect().top - scrollEl.getBoundingClientRect().top
    }
    toggleStep(step.id)
    if (rowEl && scrollEl) {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          const offsetAfter = rowEl.getBoundingClientRect().top - scrollEl.getBoundingClientRect().top
          const delta = offsetAfter - offsetBefore
          if (delta !== 0) {
            scrollEl.scrollTop += delta
          }
        })
      })
    }
  }, [canExpand, toggleStep, step.id, scrollRef])

  // hook 顺序: useMemo 必须无条件调用。
  const argsLabel = useMemo(() => compactArgs(step.args), [step.args])
  const editSum = useMemo(() => editSummary(content), [content])
  const detailLabel = useMemo(() => toolDetail(step.toolName), [step.toolName])

  const isBlocked = step.status === 'blocked'
  const isExecutionError = step.isError && !isBlocked
  const badgeCls = isBlocked
    ? 'text-warning'
    : isExecutionError
    ? 'text-danger'
    : step.status === 'running'
      ? 'text-primary' // 去除 animate-pulse-soft 避免持续合成重绘
      : step.status === 'done'
        ? 'text-success'
        : 'text-text-3'

  const statusText = statusLabel(step)

  return (
    <div
      ref={rowRef}
      className={cn(
        'w-full min-w-0 rounded-md border bg-surface transition-colors',
        expanded ? 'border-border/60' : 'border-border/35 hover:border-border/60',
        isBlocked ? 'border-warning/30' : isExecutionError ? 'border-danger/30' : '',
      )}
    >
      <button
        type="button"
        onClick={handleToggle}
        disabled={!canExpand}
        aria-expanded={canExpand ? expanded : undefined}
        aria-controls={canExpand ? bodyId : undefined}
        className={cn(
          'group flex w-full min-w-0 flex-col gap-1.5 px-3 py-2 text-left text-[12px] transition-colors',
          canExpand ? 'hover:bg-surface-3/35 active:bg-surface-3/55' : 'cursor-default',
        )}
      >
        <span className="flex w-full min-w-0 items-center gap-2">
          <span className="w-3 shrink-0 text-center text-[10px] leading-none text-text-3">
            {canExpand ? (expanded ? '▾' : '▸') : ' '}
          </span>
          <span className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-surface-2 text-[13px] leading-none ${badgeCls}`}>
            <ToolGlyph name={step.toolName} />
          </span>
          <span className={`min-w-0 truncate font-semibold ${isExecutionError ? 'text-danger' : 'text-text'}`}>
            {prettyTool(step.toolName)}
          </span>
          {detailLabel && (
            <span className="min-w-0 max-w-[14rem] truncate rounded-sm bg-surface-2 px-1.5 py-0.5 font-mono text-[11px] text-text-2">
              {detailLabel}
            </span>
          )}
          {step.toolName === 'edit' && editSum && (
            <span className="shrink-0 font-mono text-[11px] text-success">{editSum}</span>
          )}
          {statusText && (
            <span className={`ml-auto shrink-0 rounded-md bg-surface-2 px-1.5 py-0.5 font-mono text-[10px] tabular-nums ${badgeCls}`}>
              {statusText}
            </span>
          )}
        </span>
        {argsLabel && (
          <span className="grid w-full min-w-0 grid-cols-[34px_minmax(0,1fr)] items-start gap-2 pl-5">
            <span className="text-[10px] font-medium text-text-3">调用</span>
            <span className={cn('min-w-0 truncate font-mono text-[11px]', step.toolName === 'bash' ? 'text-text-2' : 'text-text-3')}>
              {argsLabel}
            </span>
          </span>
        )}
        {canExpand && !expanded && <span className="sr-only">展开查看工具输出</span>}
      </button>
      {expanded && content && <RowBody id={bodyId} step={step} />}
    </div>
  )
})

function statusLabel(s: ToolStep): string {
  if (s.status === 'blocked') return '阻止'
  if (s.isError) return '' // red tape + glyph already signal the error
  switch (s.status) {
    case 'running': return '运行中'
    case 'done': return '完成'
    case 'pending': return '等待'
  }
  return '等待'
}

function ToolGlyph({ name }: { name: string }) {
  const common = { width: 14, height: 14, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2.1, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, 'aria-hidden': true }
  if (isMCPTool(name)) {
    return <svg {...common}><path d="M5 8h14" /><path d="M5 16h14" /><path d="M8 5v14" /><path d="M16 5v14" /></svg>
  }
  switch (name) {
    case 'read':
    case 'tsread':
      return <svg {...common}><path d="M4 19.5V5a2 2 0 0 1 2-2h11a1 1 0 0 1 1 1v16H6a2 2 0 0 1-2-2Z" /><path d="M8 7h6M8 11h7" /></svg>
    case 'edit':
      return <svg {...common}><path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" /></svg>
    case 'write':
      return <svg {...common}><path d="M5 4h10l4 4v12H5Z" /><path d="M14 4v5h5" /><path d="M8 14h8M8 17h5" /></svg>
    case 'bash':
      return <svg {...common}><path d="m7 8 4 4-4 4" /><path d="M13 16h4" /></svg>
    case 'grep':
    case 'glob':
    case 'searchfiles':
      return <svg {...common}><circle cx="10.5" cy="10.5" r="5.5" /><path d="m15 15 5 5" /></svg>
    case 'todo':
      return <svg {...common}><path d="m4 7 2 2 4-4" /><path d="M12 8h8" /><path d="m4 17 2 2 4-4" /><path d="M12 18h8" /></svg>
    case 'webfetch':
    case 'fetch':
      return <svg {...common}><circle cx="12" cy="12" r="9" /><path d="M3 12h18" /><path d="M12 3a14 14 0 0 1 0 18" /><path d="M12 3a14 14 0 0 0 0 18" /></svg>
    case 'think':
      return <svg {...common}><path d="M8 14a5 5 0 1 1 8 0c-.7.6-1 1.3-1 2H9c0-.7-.3-1.4-1-2Z" /><path d="M9 20h6" /></svg>
    default:
      return <svg {...common}><path d="M12 3v18M3 12h18" /></svg>
  }
}

function RowBody({ id, step }: { id: string; step: ToolStep }) {
  const isDiffTool = isDiffPreviewTool(step.toolName)
  const content = contentForStep(step)
  if (step.toolName === 'edit' && step.isError) {
    return <TextResult id={id} step={step} text={content || 'edit failed'} />
  }
  if (step.toolName === 'diff' && step.isError) {
    return <TextResult id={id} step={step} text={content || 'diff failed'} />
  }
  if (step.toolName === 'write' && step.isError) {
    return <TextResult id={id} step={step} text={content || 'write failed'} />
  }
  if (isDiffTool && isUnifiedDiffContent(content)) {
    return (
      <div id={id} className="w-full min-w-0 border-t border-border/35 bg-surface-2/25 px-3 pb-3 pt-2">
        <ResultHeader label="变更结果" step={step} />
        <div className="mt-2 w-full min-w-0 overflow-hidden rounded-md border border-border/35 bg-surface">
          <UnifiedDiff content={content} filePath={pathFromArgs(step.args)} defaultCollapsed={false} skipHeader />
        </div>
      </div>
    )
  }
  if (step.isError) {
    return <TextResult id={id} step={step} text={content} />
  }
  return <TextResult id={id} step={step} text={content} />
}

function TextResult({ id, step, text }: { id: string; step: ToolStep; text: string }) {
  const scrollable = step.toolName !== 'write'
  const isBlocked = step.status === 'blocked'
  const isExecutionError = step.isError && !isBlocked
  return (
    <div id={id} className="w-full min-w-0 border-t border-border/35 bg-surface-2/25 px-3 pb-3 pt-2">
      <ResultHeader label={isBlocked ? '阻止原因' : isExecutionError ? '错误结果' : '工具结果'} step={step} />
      <pre
        className={cn(
          'mt-2 min-w-0 whitespace-pre-wrap break-words rounded-md border px-3 py-2 font-mono text-[11.5px] leading-relaxed [overflow-wrap:break-word]',
          isBlocked
            ? 'border-warning/25 bg-warning/8 text-warning'
            : isExecutionError
              ? 'border-danger/25 bg-danger/8 text-danger'
              : 'border-border/35 bg-surface text-text-2',
          scrollable ? 'max-h-[320px] overflow-y-auto overflow-x-hidden' : '',
        )}
      >
        {text}
      </pre>
    </div>
  )
}

function ResultHeader({ label, step }: { label: string; step: ToolStep }) {
  const content = contentForStep(step)
  const lines = resultLineCount(content)
  return (
    <div className="flex items-center gap-2 text-[10.5px] text-text-3">
      <span className="font-semibold text-text-2">{label}</span>
      {lines > 0 ? (
        <span className="font-mono tabular-nums">{lines} 行</span>
      ) : null}
    </div>
  )
}

function resultLineCount(content: string): number {
  return content
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .split('\n')
    .filter((line) => line.trim() !== '')
    .length
}

function contentForStep(step: ToolStep): string {
  if (isDiffPreviewTool(step.toolName)) return step.preview || step.output || ''
  if (step.status === 'running') return step.preview || ''
  return step.output || ''
}

function isDiffPreviewTool(toolName: string): boolean {
  return toolName === 'edit' || toolName === 'diff' || toolName === 'write'
}
