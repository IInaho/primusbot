interface NewSessionButtonProps {
  onClick: () => void
  collapsed?: boolean
}

export function NewSessionButton({ onClick, collapsed }: NewSessionButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      data-tooltip="新建会话"
      className="tooltip-anchor flex items-center justify-center gap-1.5 pill-radius bg-primary/10 px-2.5 py-1.5 text-[12px] font-medium text-primary transition-all hover:bg-primary hover:text-black"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="14"
        height="14"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden
      >
        <path d="M12 5v14M5 12h14" />
      </svg>
      {!collapsed && <span>新会话</span>}
    </button>
  )
}
