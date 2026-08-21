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
import {
  createBusinessDomain,
  listBusinessDomains,
  updateBusinessDomain,
} from '../../api'
import type {
  CreateBusinessDomainPayload,
  GovernanceBusinessDomain,
  UpdateBusinessDomainPayload,
} from '../../types'
import { MasterDataCodeCrudPage } from '../master-data-code-crud-page'

/**
 * 业务领域分区页（§11-B §3）。
 *
 * domain_code 创建后只读；无删除；前端预校验编号规则；停用需确认。
 */
export function BusinessDomainsPage() {
  return (
    <MasterDataCodeCrudPage<
      GovernanceBusinessDomain,
      CreateBusinessDomainPayload,
      UpdateBusinessDomainPayload
    >
      queryKey={['ai-governance', 'business-domains']}
      list={listBusinessDomains}
      create={createBusinessDomain}
      update={updateBusinessDomain}
      getCode={(row) => row.domain_code}
      getName={(row) => row.domain_name}
      getUpdatedAt={(row) => row.updated_at}
      toCreate={(form) => ({
        domain_code: form.code,
        domain_name: form.name,
      })}
      toUpdate={(form) => ({ domain_name: form.name })}
      toToggle={(row, enabled) => ({
        domain_name: row.domain_name,
        enabled,
      })}
      toDefaults={(row) => ({
        code: row.domain_code,
        name: row.domain_name,
      })}
      codeLabel='Domain Code'
      nameLabel='Domain Name'
      codePlaceholder='e.g. hr, finance, production'
      namePlaceholder='Enter a domain name'
      codeHelp='Starts with a lowercase letter; lowercase letters, numbers, dots, underscores or hyphens; 2-64 characters. Cannot be changed after creation.'
      pageDescription='Business domains group keys, principals and applications by line of business (HR, Finance, Production, Marketing).'
      emptyTitle='No business domains found'
      emptyDescription='No business domains match the current search and filters.'
      searchPlaceholder='Search by code or name...'
      addButton='Add Business Domain'
      createTitle='Create Business Domain'
      updateTitle='Update Business Domain'
      createDescription='Add a new business domain for grouping keys, principals and applications.'
      updateDescription='Update the business domain name or status.'
      entityCreated='Business domain created'
      entityUpdated='Business domain updated'
      entityEnabled='Business domain enabled'
      entityDisabled='Business domain disabled'
      enableDesc='This business domain will be available for assignment to principals and applications.'
      disableDesc='This business domain will no longer be selectable. Existing references are preserved.'
    />
  )
}
