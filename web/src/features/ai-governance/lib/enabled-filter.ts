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
/**
 * 把列表筛选的 enabled 值（'true' / 'false' / undefined）解析为后端查询参数。
 * 各主数据分区共用同一语义（§11-B §10）。
 */
export function parseEnabledFilter(
  value: string | undefined
): boolean | undefined {
  if (value === 'true') return true
  if (value === 'false') return false
  return undefined
}
