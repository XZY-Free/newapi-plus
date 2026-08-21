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
  createUsageTeam,
  listUsageTeams,
  updateUsageTeam,
} from '../../api'
import type {
  CreateUsageTeamPayload,
  GovernanceUsageTeam,
  UpdateUsageTeamPayload,
} from '../../types'
import { MasterDataCodeCrudPage } from '../master-data-code-crud-page'

/**
 * 使用团队分区页（§11-B §4）。
 *
 * team_code 创建后只读；帮助文本明确「Usage Team = 凭证使用人所属团队」，
 * 与 Application Owner Team 不是一回事。
 */
export function UsageTeamsPage() {
  return (
    <MasterDataCodeCrudPage<
      GovernanceUsageTeam,
      CreateUsageTeamPayload,
      UpdateUsageTeamPayload
    >
      queryKey={['ai-governance', 'usage-teams']}
      list={listUsageTeams}
      create={createUsageTeam}
      update={updateUsageTeam}
      getCode={(row) => row.team_code}
      getName={(row) => row.team_name}
      getUpdatedAt={(row) => row.updated_at}
      toCreate={(form) => ({ team_code: form.code, team_name: form.name })}
      toUpdate={(form) => ({ team_name: form.name })}
      toToggle={(row, enabled) => ({ team_name: row.team_name, enabled })}
      toDefaults={(row) => ({ code: row.team_code, name: row.team_name })}
      codeLabel='Team Code'
      nameLabel='Usage Team Name'
      codePlaceholder='e.g. hr-eng, finance-ops'
      namePlaceholder='Enter a usage team name'
      codeHelp='Starts with a lowercase letter; lowercase letters, numbers, dots, underscores or hyphens; 2-64 characters. Cannot be changed after creation.'
      pageDescription='Usage teams are the organization groups a key owner belongs to. A Usage Team identifies the team a credential user is part of; it is not an Application Owner Team.'
      emptyTitle='No usage teams found'
      emptyDescription='No usage teams match the current search and filters.'
      searchPlaceholder='Search by code or name...'
      addButton='Add Usage Team'
      createTitle='Create Usage Team'
      updateTitle='Update Usage Team'
      createDescription='Add a usage team for grouping credential owners.'
      updateDescription='Update the usage team name or status.'
      entityCreated='Usage team created'
      entityUpdated='Usage team updated'
      entityEnabled='Usage team enabled'
      entityDisabled='Usage team disabled'
      enableDesc='This usage team will be available for assignment to principals.'
      disableDesc='This usage team will no longer be selectable. Existing references are preserved.'
    />
  )
}
