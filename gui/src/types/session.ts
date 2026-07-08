// 与 Go ui.SessionMeta 对齐（Wails 自动生成格式）。
// createdAt / updatedAt 为 Wails 对 time.Time 的映射（any），
// 运行时实际是 ISO 字符串，由调用方按需转换。

export interface SessionMeta {
  id: string
  cwd: string
  createdAt: any
  updatedAt: any
  msgCount: number
}

export interface DisplayMessage {
  role: string
  content: string
  blocks: DisplayBlock[] | null
  images: ImageRef[] | null
}

export interface DisplayBlock {
  toolName: string
  args: string
  content: string
  isError?: boolean
}

export interface ImageRef {
  path: string
  url: string
  width: number
  height: number
}

// 兼容旧命名。
export type SessionMessage = DisplayMessage
