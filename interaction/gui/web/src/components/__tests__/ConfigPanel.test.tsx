import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ConfigPanel } from '../ConfigPanel'
import type { ConfigView } from '../../types/config'

const config: ConfigView = {
  path: '/tmp/config.json',
  exists: true,
  active: 'main',
  context_window: 128000,
  models: [
    {
      name: 'main',
      provider: 'openai',
      api_key: '',
      model: 'gpt-5',
      base_url: '',
      protocol: 'openai',
    },
  ],
  image_gen_models: [
    {
      name: 'image',
      provider: 'jimeng',
      api_key: '',
      secret_key: '',
      model: 'jimeng_t2i_v31',
      base_url: '',
    },
  ],
  mcp_servers: {},
}

vi.mock('../../../wailsjs/go/main/App', () => ({
  GetConfig: vi.fn(() => Promise.resolve(config)),
  SaveConfig: vi.fn((cfg: ConfigView) => Promise.resolve(cfg)),
}))

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(() => () => {}),
  Quit: vi.fn(),
}))

describe('ConfigPanel', () => {
  beforeEach(() => {
    ;(window as unknown as { go?: unknown }).go = {}
  })

  afterEach(() => {
    delete (window as unknown as { go?: unknown }).go
  })

  it('keeps focus while editing model name', async () => {
    const user = userEvent.setup()
    render(<ConfigPanel open initialTab="models" onClose={vi.fn()} onSaved={vi.fn()} />)

    const input = await screen.findByDisplayValue('main')
    await user.click(input)
    await user.type(input, '-dev')

    await waitFor(() => expect(input).toHaveValue('main-dev'))
    expect(document.activeElement).toBe(input)
  })
})
