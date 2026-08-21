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
import { useTranslation } from 'react-i18next'

import {
  createOwnerTeam,
  listOwnerTeams,
  updateOwnerTeam,
} from '../../api'
import {
  getSimpleCodeSchema,
  SIMPLE_CODE_MAX_LENGTH,
} from '../../lib/code-schema'
import type {
  CreateOwnerTeamPayload,
  GovernanceOwnerTeam,
  UpdateOwnerTeamPayload,
} from '../../types'
import { MasterDataCodeCrudPage } from '../master-data-code-crud-page'

/**
 * 应用负责团队分区页（§11-B §7）。
 *
 * Application Owner Team = AI 应用建设/维护/运营负责团队；不合并 Usage Team。
 * team_code 创建后只读；无删除。
 */
export function OwnerTeamsPage() {
  const { t } = useTranslation()
  return (
    <MasterDataCodeCrudPage<
      GovernanceOwnerTeam,
      CreateOwnerTeamPayload,
      UpdateOwnerTeamPayload
    >
      queryKey={['ai-governance', 'owner-teams']}
      codeSchema={getSimpleCodeSchema(t, SIMPLE_CODE_MAX_LENGTH)}
      list={listOwnerTeams}
      create={createOwnerTeam}
      update={updateOwnerTeam}
      getCode={(row) => row.team_code}
      getName={(row) => row.team_name}
      getUpdatedAt={(row) => row.updated_at}
      toCreate={(form) => ({ team_code: form.code, team_name: form.name })}
      toUpdate={(form) => ({ team_name: form.name })}
      toToggle={(_row, enabled) => ({ enabled })}
      toDefaults={(row) => ({ code: row.team_code, name: row.team_name })}
      codeLabel='Owner Team Code'
      nameLabel='Owner Team Name'
      codePlaceholder='e.g. ai-platform, ml-infra'
      namePlaceholder='Enter an owner team name'
      codeHelp='Required, no whitespace, up to 64 characters. Cannot be changed after creation.'
      pageDescription='Application owner teams are the teams responsible for building, maintaining and operating AI applications. They are not Usage Teams.'
      emptyTitle='No owner teams found'
      emptyDescription='No owner teams match the current search and filters.'
      searchPlaceholder='Search by code or name...'
      addButton='Add Owner Team'
      createTitle='Create Application Owner Team'
      updateTitle='Update Application Owner Team'
      createDescription='Add an application owner team responsible for building and operating AI apps.'
      updateDescription='Update the owner team name or status.'
      entityCreated='Owner team created'
      entityUpdated='Owner team updated'
      entityEnabled='Owner team enabled'
      entityDisabled='Owner team disabled'
      enableDesc='This owner team will be available for assignment to AI applications.'
      disableDesc='This owner team will no longer be selectable. Existing references are preserved.'
    />
  )
}
