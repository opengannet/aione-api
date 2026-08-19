/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const accessTokenState = vi.hoisted(() => ({
  token: '',
  loading: false,
  generating: false,
  load: vi.fn(async () => true),
  generate: vi.fn(async () => true),
  clearToken: vi.fn(),
}))

vi.mock('../../hooks', () => ({
  useAccessToken: () => accessTokenState,
}))

vi.mock('@/components/dialog', () => ({
  Dialog: (props: {
    open: boolean
    title: string
    footer?: ReactNode
    children?: ReactNode
  }) =>
    props.open ? (
      <section aria-label={props.title}>
        {props.children}
        {props.footer}
      </section>
    ) : null,
}))

vi.mock('@/components/confirm-dialog', () => ({
  ConfirmDialog: (props: {
    open: boolean
    title: string
    handleConfirm: () => void | Promise<void>
  }) =>
    props.open ? (
      <button type='button' onClick={() => void props.handleConfirm()}>
        {props.title}
      </button>
    ) : null,
}))

vi.mock('@/components/copy-button', () => ({
  CopyButton: (props: { value: string; 'aria-label'?: string }) => (
    <button
      type='button'
      aria-label={props['aria-label']}
      data-token={props.value}
    />
  ),
}))

const { api } = await import('@/lib/api')
const { generateAccessToken, getAccessToken } = await import('../../api')
const { AccessTokenDialog } = await import('../dialogs/access-token-dialog')

type ApiMethod = (url: string) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  post: ApiMethod
}

const apiClient = api as unknown as MockableApi

beforeEach(() => {
  accessTokenState.token = 'existing-system-access-token'
  accessTokenState.loading = false
  accessTokenState.generating = false
  accessTokenState.load.mockClear()
  accessTokenState.generate.mockClear()
})

describe('access token API contract', () => {
  test('reads the current token with GET and rotates it with POST', async () => {
    const get = vi
      .spyOn(apiClient, 'get')
      .mockResolvedValue({ data: { success: true, data: 'existing' } })
    const post = vi
      .spyOn(apiClient, 'post')
      .mockResolvedValue({ data: { success: true, data: 'rotated' } })

    await getAccessToken()
    await generateAccessToken()

    expect(get).toHaveBeenCalledWith('/api/user/token')
    expect(post).toHaveBeenCalledWith('/api/user/token')
  })
})

describe('AccessTokenDialog', () => {
  test('loads and displays the existing token when opened', async () => {
    render(<AccessTokenDialog open onOpenChange={() => undefined} />)

    await waitFor(() => expect(accessTokenState.load).toHaveBeenCalledOnce())
    expect(screen.getByRole('textbox', { name: 'Token' })).toHaveValue(
      'existing-system-access-token'
    )
    expect(screen.getByRole('button', { name: 'Copy token' })).toHaveAttribute(
      'data-token',
      'existing-system-access-token'
    )
  })

  test('requires confirmation before regenerating the token', async () => {
    render(<AccessTokenDialog open onOpenChange={() => undefined} />)

    fireEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
    expect(accessTokenState.generate).not.toHaveBeenCalled()

    fireEvent.click(
      screen.getByRole('button', { name: 'Regenerate access token?' })
    )
    await waitFor(() =>
      expect(accessTokenState.generate).toHaveBeenCalledOnce()
    )
  })
})
