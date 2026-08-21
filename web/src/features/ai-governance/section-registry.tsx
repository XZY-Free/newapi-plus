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
 * 企业 AI 治理页面分区注册表（V1.1 §11.3）。
 *
 * 一个“企业 AI 治理”一级入口，内部 9 个分区以 `$section` 路径路由驱动。
 * `build` 返回各分区页面组件；§11-A 阶段先挂载共享外壳，后续子批（§11-B～E）
 * 逐区替换为完整实现。
 */
import {
  Boxes,
  Building2,
  Fingerprint,
  KeySquare,
  ListChecks,
  ScanSearch,
  ShieldCheck,
  Tags,
  Users,
  type LucideIcon,
} from 'lucide-react'

import { createSectionRegistry } from '@/features/system-settings/utils/section-registry'

import { GovernanceSectionShell } from './components/governance-section-shell'

const AI_GOVERNANCE_SECTIONS = [
  { id: 'business-domains', titleKey: 'Business Domains', build: () => <GovernanceSectionShell /> },
  { id: 'usage-teams', titleKey: 'Usage Teams', build: () => <GovernanceSectionShell /> },
  { id: 'principals', titleKey: 'Principals', build: () => <GovernanceSectionShell /> },
  { id: 'credential-purposes', titleKey: 'Credential Purposes', build: () => <GovernanceSectionShell /> },
  { id: 'owner-teams', titleKey: 'Application Owner Teams', build: () => <GovernanceSectionShell /> },
  { id: 'applications', titleKey: 'AI Applications', build: () => <GovernanceSectionShell /> },
  { id: 'identity-profiles', titleKey: 'API Key Identity', build: () => <GovernanceSectionShell /> },
  { id: 'identity-audit', titleKey: 'Identity Audit', build: () => <GovernanceSectionShell /> },
  { id: 'usage', titleKey: 'Enterprise Usage', build: () => <GovernanceSectionShell /> },
] as const

export type GovernanceSectionId = (typeof AI_GOVERNANCE_SECTIONS)[number]['id']

const governanceRegistry = createSectionRegistry<
  GovernanceSectionId,
  Record<string, never>,
  []
>({
  sections: AI_GOVERNANCE_SECTIONS,
  defaultSection: 'business-domains',
  basePath: '/ai-governance',
  urlStyle: 'path',
})

export const AI_GOVERNANCE_SECTION_IDS = governanceRegistry.sectionIds
export const AI_GOVERNANCE_DEFAULT_SECTION = governanceRegistry.defaultSection

export function isGovernanceSectionId(s: string): s is GovernanceSectionId {
  return (AI_GOVERNANCE_SECTION_IDS as readonly string[]).includes(s)
}

export const getAIGovernanceSectionNavItems =
  governanceRegistry.getSectionNavItems

export const getGovernanceSectionContent =
  governanceRegistry.getSectionContent

export const getGovernanceSectionMeta = governanceRegistry.getSectionMeta

/**
 * 分区卡片元数据：用于治理首页九宫格导航。
 */
export const AI_GOVERNANCE_SECTION_META: Record<
  GovernanceSectionId,
  { icon: LucideIcon; descriptionKey: string }
> = {
  'business-domains': {
    icon: Building2,
    descriptionKey:
      'Business domains group keys, principals and applications by line of business (HR, Finance, Production, Marketing)',
  },
  'usage-teams': { icon: Users, descriptionKey: 'Usage teams are the organization groups a key owner belongs to' },
  'principals': { icon: Fingerprint, descriptionKey: 'Principals identify the person accountable for weak-identity keys' },
  'credential-purposes': {
    icon: Tags,
    descriptionKey: 'Credential purposes declare what a key is approved for (WorkBuddy, IDE, script)',
  },
  'owner-teams': { icon: Boxes, descriptionKey: 'Application owner teams are responsible for building and operating AI apps' },
  'applications': {
    icon: KeySquare,
    descriptionKey: 'AI applications carry a stable app code used as root_app_id',
  },
  'identity-profiles': {
    icon: ShieldCheck,
    descriptionKey: 'API Key identity binds a NewAPI token to a governed identity profile',
  },
  'identity-audit': {
    icon: ListChecks,
    descriptionKey: 'Identity audit records verification failures and downgrades',
  },
  'usage': {
    icon: ScanSearch,
    descriptionKey: 'Enterprise usage projects consumption by domain, team, person, purpose, app and caller',
  },
}
