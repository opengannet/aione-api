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
import { RefreshCw, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { useAccessToken } from '../../hooks'

// ============================================================================
// Access Token Dialog Component
// ============================================================================

interface AccessTokenDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AccessTokenDialog({
  open,
  onOpenChange,
}: AccessTokenDialogProps) {
  const { t } = useTranslation()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const { token, loading, generating, load, generate } = useAccessToken()
  const hasToken = token.length > 0
  const actionLabel = hasToken ? t('Regenerate') : t('Generate')

  useEffect(() => {
    if (open) {
      load()
    }
  }, [open, load])

  const handleOpenChange = (nextOpen: boolean) => {
    if (generating) return
    if (!nextOpen) {
      setConfirmOpen(false)
    }
    onOpenChange(nextOpen)
  }

  const handleConfirm = async () => {
    if (await generate()) {
      setConfirmOpen(false)
    }
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={handleOpenChange}
        title={t('Access Token')}
        description={t(
          "Your system access token for API authentication. Keep it secure and don't share it with others."
        )}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={() => handleOpenChange(false)}
            >
              {t('Close')}
            </Button>
            <Button
              type='button'
              onClick={() => setConfirmOpen(true)}
              disabled={loading || generating}
              className='gap-2'
            >
              {loading ? (
                <Loader2 className='h-4 w-4 animate-spin' aria-hidden='true' />
              ) : (
                <RefreshCw className='h-4 w-4' aria-hidden='true' />
              )}
              {loading ? t('Loading...') : actionLabel}
            </Button>
          </>
        }
      >
        <div className='my-6 space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='token'>{t('Token')}</Label>
            <div className='flex gap-2'>
              <Input
                id='token'
                type='text'
                value={token}
                readOnly
                disabled={loading}
                className='font-mono text-xs'
                placeholder={
                  loading
                    ? t('Loading...')
                    : t('No access token has been generated')
                }
              />
              <div className='size-9 shrink-0'>
                {hasToken && (
                  <CopyButton
                    value={token}
                    variant='outline'
                    className='size-9'
                    iconClassName='size-4'
                    tooltip={t('Copy token')}
                    aria-label={t('Copy token')}
                  />
                )}
              </div>
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('Use this token for API authentication')}
            </p>
          </div>
        </div>
      </Dialog>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(nextOpen) => {
          if (!generating) setConfirmOpen(nextOpen)
        }}
        title={
          hasToken ? t('Regenerate access token?') : t('Generate access token?')
        }
        desc={
          hasToken
            ? t(
                'Generating a new access token will immediately invalidate the current token. Any systems using it must be updated.'
              )
            : t('Generate a system access token for system API authentication?')
        }
        confirmText={generating ? t('Generating...') : actionLabel}
        destructive={hasToken}
        isLoading={generating}
        handleConfirm={handleConfirm}
      />
    </>
  )
}
