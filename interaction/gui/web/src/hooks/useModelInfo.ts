import { useEffect, useState } from 'react'
import { safeCurrentModel } from '../lib/wails'

export function useModelInfo(refreshKey = 0): string {
  const [model, setModel] = useState('')

  useEffect(() => {
    let cancelled = false
    safeCurrentModel()
      .then((v: string) => {
        if (cancelled) return
        if (!v) return
        const [provider, name] = v.split('|')
        if (provider && name) {
          setModel(`${provider} / ${name}`)
        }
      })
      .catch(() => {
        /* ignore */
      })
    return () => {
      cancelled = true
    }
  }, [refreshKey])

  return model
}
