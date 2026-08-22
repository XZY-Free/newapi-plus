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
import type { ReactNode } from 'react'

/**
 * 详情只读字段（label + value）。
 *
 * 独立成模块以被 detail sheet 与各 Tab（Security & Risk / Reconfigure 等）复用，
 * 避免它们互相 import 造成的依赖环。
 */
export function DetailField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className='flex flex-col gap-0.5'>
      <dt className='text-muted-foreground text-xs'>{label}</dt>
      <dd className='text-sm'>{children}</dd>
    </div>
  )
}
