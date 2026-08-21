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
 * 无 JSX 的事实源模块：唯一保存 9 个分区元数据（titleKey/descriptionKey/icon）。
 * `GovernanceSectionShell` 为纯展示组件，由本注册表在 `build` 时按分区注入元数据；
 * 因此不存在 shell ↔ registry 循环依赖。
 *
 * §11-A 阶段先挂载共享外壳，后续子批（§11-B～E）逐区替换为完整实现。
 */
import { createElement, type ComponentType } from 'react'
import {
  Boxes,
  Building2,
  Fingerprint,
  KeySquare,
  ListChecks,
  ScanSearch,
  ShieldCheck,
  Tags,
  type LucideIcon,
  Users,
} from 'lucide-react'

import {
  createSectionRegistry,
  type SectionDefinition,
} from '@/features/system-settings/utils/section-registry'

import { ApplicationsPage } from './components/applications/applications-page'
import { BusinessDomainsPage } from './components/business-domains/business-domains-page'
import { CredentialPurposesPage } from './components/credential-purposes/credential-purposes-page'
import { GovernanceSectionShell } from './components/governance-section-shell'
import { PrincipalsPage } from './components/principals/principals-page'
import { OwnerTeamsPage } from './components/owner-teams/owner-teams-page'
import { UsageTeamsPage } from './components/usage-teams/usage-teams-page'

export type GovernanceSectionId =
  | 'business-domains'
  | 'usage-teams'
  | 'principals'
  | 'credential-purposes'
  | 'owner-teams'
  | 'applications'
  | 'identity-profiles'
  | 'identity-audit'
  | 'usage'

export interface GovernanceSectionMeta {
  titleKey: string
  descriptionKey: string
  icon: LucideIcon
}

/**
 * 分区元数据唯一事实源（九宫格与钻取侧边栏共用，不得另建第二份）。
 */
export const AI_GOVERNANCE_SECTION_META: Record<GovernanceSectionId, GovernanceSectionMeta> = {
  'business-domains': {
    titleKey: 'Business Domains',
    icon: Building2,
    descriptionKey:
      'Business domains group keys, principals and applications by line of business (HR, Finance, Production, Marketing)',
  },
  'usage-teams': {
    titleKey: 'Usage Teams',
    icon: Users,
    descriptionKey: 'Usage teams are the organization groups a key owner belongs to',
  },
  'principals': {
    titleKey: 'Principals',
    icon: Fingerprint,
    descriptionKey: 'Principals identify the person accountable for weak-identity keys',
  },
  'credential-purposes': {
    titleKey: 'Credential Purposes',
    icon: Tags,
    descriptionKey: 'Credential purposes declare what a key is approved for (WorkBuddy, IDE, script)',
  },
  'owner-teams': {
    titleKey: 'Application Owner Teams',
    icon: Boxes,
    descriptionKey: 'Application owner teams are responsible for building and operating AI apps',
  },
  'applications': {
    titleKey: 'AI Applications',
    icon: KeySquare,
    descriptionKey: 'AI applications carry a stable app code used as root_app_id',
  },
  'identity-profiles': {
    titleKey: 'API Key Identity',
    icon: ShieldCheck,
    descriptionKey: 'API Key identity binds a NewAPI token to a governed identity profile',
  },
  'identity-audit': {
    titleKey: 'Identity Audit',
    icon: ListChecks,
    descriptionKey: 'Identity audit records verification failures and downgrades',
  },
  'usage': {
    titleKey: 'Enterprise Usage',
    icon: ScanSearch,
    descriptionKey: 'Enterprise usage projects consumption by domain, team, person, purpose, app and caller',
  },
}

/**
 * 已实现真实页面的分区。identity-profiles / identity-audit / usage 三区
 * 在后续子批（§11-C/D/E）实现，保持 `GovernanceSectionShell`。
 */
const SECTION_PAGE: Partial<Record<GovernanceSectionId, ComponentType>> = {
  'business-domains': BusinessDomainsPage,
  'usage-teams': UsageTeamsPage,
  'owner-teams': OwnerTeamsPage,
  'credential-purposes': CredentialPurposesPage,
  'principals': PrincipalsPage,
  'applications': ApplicationsPage,
}

const AI_GOVERNANCE_SECTIONS: readonly SectionDefinition<Record<string, never>, []>[] = (
  Object.keys(AI_GOVERNANCE_SECTION_META) as GovernanceSectionId[]
).map((id) => {
  const meta = AI_GOVERNANCE_SECTION_META[id]
  const PageComponent = SECTION_PAGE[id]
  return {
    id,
    titleKey: meta.titleKey,
    build: () =>
      PageComponent
        ? createElement(PageComponent)
        : createElement(GovernanceSectionShell, {
            titleKey: meta.titleKey,
            descriptionKey: meta.descriptionKey,
            icon: meta.icon,
          }),
  }
})

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
