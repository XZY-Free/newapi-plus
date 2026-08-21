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
import { Circle, CircleOff, Pencil } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'

/**
 * 治理主数据行操作（§11-B §10）。
 *
 * 统一「编辑」+「启用/停用」两个操作；无删除能力。启停带确认弹窗，由调用方
 * `onToggle` 执行真实的更新请求（成功后 refresh），后端拒绝停用时错误由调用方
 * 透出真实 message，绝不吞成 Operation failed。
 */
export function MasterDataRowActions({
  isEnabled,
  onEdit,
  onToggle,
  enableDesc,
  disableDesc,
}: {
  isEnabled: boolean
  onEdit: () => void
  onToggle: (enabled: boolean) => Promise<void>
  enableDesc: string
  disableDesc: string
}) {
  const { t } = useTranslation()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [isToggling, setIsToggling] = useState(false)

  const handleConfirm = async () => {
    setIsToggling(true)
    try {
      await onToggle(!isEnabled)
    } finally {
      setIsToggling(false)
      setConfirmOpen(false)
    }
  }

  return (
    <>
      <div className='flex items-center justify-end gap-0.5'>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          onClick={onEdit}
          aria-label={t('Edit')}
        >
          <Pencil className='size-4' />
        </Button>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          onClick={() => setConfirmOpen(true)}
          aria-label={t(isEnabled ? 'Disable' : 'Enable')}
        >
          {isEnabled ? (
            <CircleOff className='size-4 text-muted-foreground' />
          ) : (
            <Circle className='size-4 text-muted-foreground' />
          )}
        </Button>
      </div>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t(isEnabled ? 'Disable this item?' : 'Enable this item?')}
        desc={isEnabled ? disableDesc : enableDesc}
        confirmText={t(isEnabled ? 'Disable' : 'Enable')}
        destructive={isEnabled}
        handleConfirm={handleConfirm}
        isLoading={isToggling}
      />
    </>
  )
}
