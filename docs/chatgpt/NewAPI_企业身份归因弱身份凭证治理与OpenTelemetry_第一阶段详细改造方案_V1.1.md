# NewAPI 企业身份归因、弱身份凭证治理、业务统计与 OpenTelemetry 第一阶段详细改造方案 V1.1

**文档性质：实施交接方案 / AI 编码执行约束**  
**实施范围：NewAPI 网关侧，不包含 FastGPT、Dify、智能体平台等应用层改造**  
**源码基线：QuantumNous/new-api `v1.0.0-rc.25`**  
**基线提交：`f116414284162ad15d8925f7bca494c109b83e93`**  
**编制日期：2026-08-20**  
**本版性质：替代 V1.0 的完整实施基线，不得再按 V1.0 开始编码**

---

# 0. 本文档的执行方式

本文档不是供实现者自由设计的架构建议，而是第一阶段的实施约束。V1.1 在 V1.0 基础上正式加入“弱身份凭证客户端”模型，用于 WorkBuddy、IDE、桌面客户端、第三方工具等只能配置 Base URL + API Key、无法提供可信动态签名上下文的调用方。

接手实现的 AI 或开发人员必须遵守以下原则：

1. 必须以 `v1.0.0-rc.25` / `f116414284162ad15d8925f7bca494c109b83e93` 为第一阶段代码基线。
2. 开始修改前必须确认本地 HEAD 与基线一致；如果文件、Symbol、调用链与本文档描述明显不同，必须先报告差异，不得自行推断后继续改造。
3. 第一阶段不得修改 NewAPI 原有 Token、User、Group 的业务语义，不得把企业业务领域、使用团队、使用人、负责团队、根应用等字段塞入 NewAPI 原生 User/Group 模型。
4. 第一阶段不得重新实现 Token 统计、模型价格、Quota、渠道计费、Provider Usage 解析。
5. 第一阶段不得逐个修改 Provider Adapter 来接企业身份或 OpenTelemetry；必须优先使用统一入口、统一上下文、统一日志出口和统一 HTTP 出口。
6. 第一阶段不得修改 FastGPT、Dify、WorkBuddy 或其他调用方源码。
7. 第一阶段不得采集完整 Prompt、完整模型 Response 作为企业治理数据。
8. 第一阶段不得信任客户端声明的业务领域、负责团队、使用团队、使用人、Caller、身份验证结果。
9. 第一阶段不得复用 NewAPI API Key 作为 HMAC 签名密钥。
10. 第一阶段不得把企业内部 `X-AI-*` 身份头转发给外部模型提供方。
11. 第一阶段不得创建企业私有 `gen_ai.*` 属性；企业扩展必须使用 `company.ai.*`。
12. `STATIC / DYNAMIC / HYBRID` 表示“身份取得方式”，`CREDENTIAL_ONLY / SIGNED_CONTEXT / HYBRID_VERIFIED_CONTEXT` 表示“身份可信等级”，两者不得混为一个字段。
13. 对 `CREDENTIAL_ONLY` 请求，只能宣称“凭证已验证”；不得宣称“客户端已验证”。例如某 Key 登记用途为 WorkBuddy，不等于 NewAPI 能证明本次请求确实来自 WorkBuddy。
14. 一个个人弱身份凭证必须绑定一个明确责任人和一个明确批准用途；同一 Key 不得同时批准用于 WorkBuddy、IDE、脚本等多个用途。
15. 同一个人可以拥有多个 Key，但不同用途必须使用不同 Key。
16. 业务应用的“建设负责团队”和凭证使用人的“使用团队”必须分离；不得复用同一个字段。
17. 业务应用的业务领域与使用人的业务领域可以复用同一套 Business Domain 主数据，但日志与统计字段必须区分来源。
18. 每一批必须先完成该批测试和验收，再进入下一批；不得为了“先跑起来”跳过安全或迁移门禁。
19. 若某个需求无法在本文档约束下完成，应停止该点实现并记录冲突，不得自行改变身份模型、数据库关系或签名协议。

---

# 1. 第一阶段最终目标

企业内部只将 NewAPI 作为统一人工智能模型网关使用。

NewAPI 原有用户体系在企业内部原则上只有一个内部管理用户。用户余额、充值、订阅、用户分组、用户额度等能力继续保留以维持 NewAPI 自身运行，但不作为企业业务治理主线。

企业治理以 **NewAPI Token / API Key** 为技术身份锚点，但必须区分两条归因链。

## 1.1 强身份平台链

适用于可持有独立 Signing Secret、可发送签名执行上下文的工作流平台、智能体平台和自研服务：

```text
API Key
→ Verified Caller
→ Root App
→ Application Business Domain
→ Application Owner Team
→ Root Run
→ Current Execution
→ Model Call
→ Token / Quota / Latency / Status / Trace
```

此链可以形成：

- `credential_verified=true`
- `client_verified=true`
- `identity_assurance=SIGNED_CONTEXT` 或 `HYBRID_VERIFIED_CONTEXT`

## 1.2 弱身份个人凭证链

适用于 WorkBuddy、IDE、桌面客户端、第三方工具等只能配置 Base URL + API Key 的客户端：

```text
API Key
→ Credential Owner
→ Usage Business Domain
→ Usage Team
→ Credential Purpose
→ Model Call
→ Token / Quota / Latency / Status / Trace
```

此链只能形成：

- `credential_verified=true`
- `client_verified=false`
- `identity_assurance=CREDENTIAL_ONLY`

例如：

```text
张三
→ 财务
→ 财务数字化组
→ 批准用途：WorkBuddy
→ Token 101
```

NewAPI 只能证明“请求使用了 Token 101”，不能证明“请求进程一定是 WorkBuddy”。

## 1.3 第一阶段必须回答的问题

第一阶段完成后必须能够回答：

- 这次请求使用了哪把 API Key；
- 这把 Key 的责任人是谁；
- 这把 Key 批准用于什么场景；
- 如果属于弱身份客户端，该 Key 的使用人属于哪个业务领域、哪个使用团队；
- 如果属于强身份平台，哪个技术系统被密码学方式验证为 Caller；
- 如果调用属于业务应用，本次调用最终归属于哪个企业 AI 应用；
- 该应用属于哪个业务领域；
- 该应用由哪个建设团队负责；
- 本次调用属于哪个 Root Run；
- 如调用方具备能力，本次调用发生在哪个工作流、智能体、任务、节点或其他执行位置；
- 调用了哪个模型、实际经过哪个渠道；
- 输入 Token、输出 Token、Quota、耗时、错误分别是多少；
- 当前可信度到底是“只验证凭证”还是“验证了客户端/平台与上下文”；
- 请求在 OpenTelemetry 中属于哪条 Trace。

---

# 2. 明确不改变的 NewAPI 核心能力

以下能力全部继续使用 NewAPI 原有实现：

- Token/API Key 创建、状态、过期、模型限制、IP 限制；
- TokenAuth；
- 用户对象和用户状态；
- NewAPI Group；
- ModelRequestRateLimit；
- Distribute；
- 渠道选择和重试；
- Provider Adapter；
- 模型协议转换；
- Prompt Token 估算；
- Provider Usage 解析；
- 预扣费；
- 最终结算；
- 退款；
- Subscription / Wallet 等原有计费路径；
- `RecordConsumeLog`；
- `RecordTaskBillingLog`；
- `QuotaData`；
- `PerfMetrics`。

企业改造只负责回答“这笔消费应归到谁、其身份可信到什么程度”，不得另外建立“这笔消费是多少”的权威计算体系。

对于弱身份客户端：

- IP 白名单：复用 NewAPI Token `AllowIps`；
- 模型白名单：复用 NewAPI Token `ModelLimits`；
- 总额度上限：复用 NewAPI Token `RemainQuota/UnlimitedQuota`；
- 有效期：复用 NewAPI Token `ExpiredTime`；
- 原 `ModelRequestRateLimit` 保留，但由于其按 NewAPI `user_id`/Group 计数，而企业内部所有 Key 基本属于同一个 NewAPI User，因此不得把它误认为“每 Key 限流”；
- 第一阶段额外新增一个企业 Profile 级请求频率限制，只解决现有产品在单用户场景下缺失的 per-credential guardrail，不替换原限流器。

---

# 3. 源码事实与固定接入边界

第一阶段必须利用 NewAPI 当前已有的集中边界。

## 3.1 身份认证边界

当前模型请求在主要 `/v1` 链路中已经形成：

```text
TokenAuth
→ ModelRequestRateLimit
→ Distribute
→ Relay
```

企业身份验证必须插在 TokenAuth 之后、模型限流/分发之前。由于原 `ModelRequestRateLimit` 按 user_id/Group 计数，V1.1 在身份解析之后增加凭证级限流：

```text
TokenAuth
→ AIIdentityAuth
→ AICredentialRateLimit
→ ModelRequestRateLimit
→ Distribute
→ Relay
```

`AICredentialRateLimit` 只按企业 `profile_id` 做每凭证请求频率保护；原 `ModelRequestRateLimit` 继续保持原行为。

不得把企业身份验证放到 Provider Adapter 中。

## 3.2 请求上下文边界

`TokenAuth` 已经把 `token_id`、`token_key`、`token_name`、Token Group、模型限制等信息写入 Gin Context。

企业身份中间件必须复用已经认证后的 `token_id`，不得重新解析或重新验证 API Key。TokenAuth 成功只能证明凭证有效；对 CREDENTIAL_ONLY 模式不得进一步推导真实客户端进程。

## 3.3 Relay 事实边界

`relay/common/relay_info.go` 中的 `RelayInfo` 已经集中携带 Token、User、Group、RequestId、模型、渠道、价格、重试、流式状态等模型调用事实。

企业改造只能给 `RelayInfo` 增加一个统一的归因快照/遥测状态引用，不得向其散落新增十几个互不关联的企业字段。

## 3.4 消费日志边界

`model.RecordConsumeLog` 已经是同步模型消费日志的重要统一出口；`RecordTaskBillingLog` 是异步任务退款、补扣等日志的重要统一出口。

第一阶段必须在这些集中出口合并企业归因快照，不得在每个 Provider 单独写一份业务日志。

## 3.5 出站 HTTP 边界

普通 Provider HTTP 请求主要经过 `relay/channel/api_request.go` 的统一请求创建和 `doRequest`，HTTP Client 由 `service/http_client.go` 集中构造和缓存。

OpenTelemetry HTTP Client Instrumentation 必须优先利用这一集中出口。

## 3.6 数据迁移边界

企业治理表属于主数据库 DB，不属于 LOG_DB。

所有新增主库模型必须同时加入：

- `migrateDB()` 的 AutoMigrate 列表；
- `migrateDBFast()` 的 migrations 列表。

第一阶段不得修改 ClickHouse `logs` DDL，因为消费日志企业扩展先写入已有 `Other` JSON。

---

# 4. 第一阶段整体模块划分

为降低与上游冲突，企业代码必须集中，不允许随意散落。

建议固定如下职责边界；实现者可以根据现有 package 循环依赖做极小调整，但不得改变职责：

| 位置 | 职责 |
|---|---|
| `constant/ai_attribution.go` | Header 名称、模式、可信等级、错误码、Context Key、长度限制等常量 |
| `types/ai_attribution.go` | 无数据库依赖的 Trusted Context、快照、协议 DTO、枚举 |
| `common/ai_attribution_context.go` | Gin Context 中 Trusted Attribution 的 Set/Get |
| `model/ai_governance.go` 或同前缀拆分文件 | Domain、Owner Team、Usage Team、Principal、Purpose、Application、Profile、Binding、Signing Key、Audit Event、Usage Projection |
| `service/ai_identity.go` | Profile/Principal/Purpose/Binding/App 主数据解析、运行时验证编排、缓存 |
| `service/ai_identity_crypto.go` | Signing Secret 生成、加密、解密、HMAC、轮换 |
| `service/ai_credential_risk.go` | 弱身份凭证安全姿态计算，只读取 NewAPI Token 现有 IP/Model/Quota/Expiry 等安全配置 |
| `middleware/ai_identity.go` | 请求是否需要归因、Header 提取/删除、模式执行、错误返回 |
| `middleware/ai_credential_rate_limit.go` | 按 `profile_id` 执行企业凭证级请求频率限制，不修改原 NewAPI `ModelRequestRateLimit` |
| `controller/ai_governance.go` 或同前缀拆分 | Root 管理 API |
| `pkg/telemetry/*` | OTel 初始化、GenAI Span/Metric、企业属性映射、Provider 映射 |
| `web/src/features/ai-governance/*` | 企业治理前端 |
| `web/src/routes/_authenticated/ai-governance/*` | 企业治理前端路由 |

不得新建 `v1/`、`v2/` 之类版本目录承载领域代码。

---

# 5. 批次总览与强制依赖关系

第一阶段严格拆为六批：

| 批次 | 名称 | 主要产物 | 是否允许进入下一批的前提 |
|---|---|---|---|
| 1 | 企业主数据、凭证责任、身份配置与密钥基础 | 10 张治理/审计表、Root 管理 API、Signing Secret 安全存储 | 数据约束、迁移、弱身份/强身份配置规则、加密、管理 API 全部通过 |
| 2 | 运行时身份认证 | STATIC 弱身份、DYNAMIC/HYBRID 强身份、HMAC、Timestamp、Nonce、凭证级独立限流、AUDIT/ENFORCE、路由接入 | 全部正反安全用例通过 |
| 3 | 消费事实归因 | Trusted Context → RelayInfo → Consume/Error Log、弱/强身份快照 | Token/Quota 不受影响，日志归因完整且无密钥泄漏 |
| 4 | OpenTelemetry GenAI | W3C Trace、Server Span、GenAI Span、HTTP 子 Span、企业身份可信等级与业务维度 | 跨服务 Trace、Token 语义、流式、重试全部验证 |
| 5 | 异步任务归因 | Task Attribution Snapshot、Trace Link、弱/强身份任务访问隔离 | 重启/轮询/退款/补扣/跨 Token 场景通过 |
| 6 | 管理页面、统计投影与 ENFORCE 上线门禁 | AI Governance UI、人员/使用团队/用途/风险姿态、Usage Projection、审计看板、强制模式 | 审计期达到强制上线门禁 |

不得把第 6 批前端管理页面提前作为第 1 批实现阻塞项；第 1～5 批允许通过 Root API 和测试 Fixture 管理配置。

---

# 6. 第一批：企业主数据、凭证责任、身份配置与签名密钥基础

## 6.1 第一批目标

第一批只建设“以后身份验证需要依赖的可信配置”，不接模型请求链。

第一批完成后：

- NewAPI 原模型调用行为必须与改造前完全一致；
- 管理员可以通过 Root API 建业务领域、应用建设团队、凭证使用团队、使用主体、凭证用途和 AI 应用；
- 可以将某个已有 NewAPI Token 登记为“个人弱身份凭证”或“平台强身份凭证”；
- 可以明确一个弱身份 Key 的责任人和批准用途；
- 可以配置 STATIC / DYNAMIC / HYBRID；
- 可以配置 DYNAMIC/HYBRID Token 允许代表哪些 AI 应用；
- 可以生成和轮换独立 Signing Secret；
- 可以计算弱身份凭证的安全姿态；
- 但模型请求此时尚不执行 AIIdentityAuth。

---

## 6.2 数据表一：`ai_business_domains`

字段必须包含：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `domain_code` | `varchar(64)`，唯一，创建后不可修改 |
| `domain_name` | `varchar(128)`，必填，去首尾空格 |
| `enabled` | 布尔，默认 true |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

`domain_code` 规则：

- 2～64 字符；
- 首字符必须为小写英文字母；
- 后续只允许小写字母、数字、`.`、`_`、`-`；
- 不允许空格；
- 不允许中文作为 code；
- 示例：`human_resources`、`finance`、`manufacturing`。

这张主数据同时被两类对象引用：

1. `ai_applications.business_domain_id`：表示应用服务的业务领域；
2. `ai_principals.business_domain_id`：表示个人弱身份凭证责任人的业务归属。

虽然复用同一 Domain 表，但日志字段必须区分：

- `application_business_domain_*`
- `usage_business_domain_*`

不得只使用一个含义模糊的 `business_domain` 字段。

业务规则：

1. code 创建后不可修改。
2. 第一阶段不提供物理删除 API，只提供启用/停用。
3. 已停用 Domain 不允许被新建/修改 Application 或 Principal 选中。
4. Domain 停用不自动禁用已存在 Application/Principal。
5. Domain 停用用于停止继续分配分类，不是模型调用总开关。

---

## 6.3 数据表二：`ai_owner_teams`

表示 **AI 应用的建设、维护、运营负责团队**，不是 Key 使用人的所属团队。

字段：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `team_code` | `varchar(64)`，唯一，创建后不可修改 |
| `team_name` | `varchar(128)`，必填 |
| `enabled` | 默认 true |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

示例：

- `ai_application` / AI应用组；
- `manufacturing_dev` / 生产开发组；
- `hr_product` / 人力产品组。

不得用该表表示“张三属于财务数字化组”。

---

## 6.4 数据表三：`ai_usage_teams`

表示 **凭证使用人的组织/使用团队**。

字段：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `team_code` | `varchar(64)`，唯一，创建后不可修改 |
| `team_name` | `varchar(128)`，必填 |
| `enabled` | 默认 true |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

示例：

- `finance_digital` / 财务数字化组；
- `hr_operations` / 人力运营组；
- `manufacturing_it` / 生产信息化组。

`ai_owner_teams` 和 `ai_usage_teams` 必须是两个不同表，禁止复用。

---

## 6.5 数据表四：`ai_principals`

表示个人弱身份凭证的责任主体/使用主体。第一阶段只支持 `PERSON`，不建设企业人员目录同步。

字段：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `principal_code` | `varchar(128)`，唯一，创建后不可修改 |
| `principal_name` | `varchar(128)`，必填 |
| `principal_type` | 第一阶段固定 `PERSON` |
| `business_domain_id` | 必填，索引 |
| `usage_team_id` | 必填，索引 |
| `enabled` | 默认 true |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

规则：

1. `principal_code` 使用稳定编号，不建议使用中文姓名作为 code。
2. 允许姓名修改，但 code 不变。
3. Domain 与 Usage Team 创建/修改时必须 enabled。
4. Principal 停用后，ENFORCE 下所有绑定该 Principal 的 STATIC/PRINCIPAL Profile 必须拒绝。
5. Principal 的 Domain/Usage Team 变化只影响后续调用，历史日志保存调用时快照。
6. 第一阶段不把 NewAPI User 与 Principal 建立一对一映射；NewAPI 仍只有统一内部用户。

---

## 6.6 数据表五：`ai_credential_purposes`

表示公司批准某把个人/固定 Key 用于什么场景。它是“登记用途”，不是“已验证客户端”。

字段：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `purpose_code` | `varchar(64)`，唯一，创建后不可修改 |
| `purpose_name` | `varchar(128)`，必填 |
| `purpose_type` | `DESKTOP_CLIENT / IDE / SCRIPT / SERVICE / OTHER` |
| `enabled` | 默认 true |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

示例：

- `workbuddy` / WorkBuddy；
- `ide_assistant` / IDE AI 助手；
- `personal_script` / 个人开发脚本。

规则：

1. Purpose 只是批准用途，日志必须使用 `credential_purpose_*`。
2. 不得产生 `verified_client=workbuddy` 之类字段。
3. Purpose 停用后禁止新建 Profile 选中；现有 Profile 是否继续调用由 Profile 自身 enabled 决定，直到第二批 ENFORCE 规则生效。
4. 一个个人弱身份 Profile 必须恰好绑定一个 Purpose。
5. 同一 Key 不允许对应多个 Purpose。

---

## 6.7 数据表六：`ai_applications`

字段：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `app_code` | `varchar(64)`，唯一，创建后不可修改 |
| `app_name` | `varchar(128)`，必填 |
| `business_domain_id` | 必填，索引 |
| `owner_team_id` | 必填，索引 |
| `enabled` | 默认 true |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

`app_code` 与 Domain Code 使用同一字符集规则。

**协议口径冻结：** 对外协议字段 `root_app_id` 使用稳定 `app_code`，例如 `hr_assistant`；数据库内部自增 `id` 仅用于关联和索引。

业务规则：

1. 创建 Application 时 Domain 和 Owner Team 必须 enabled。
2. `app_code` 一旦创建不得修改。
3. `app_name`、Domain、Owner Team 可以修改。
4. Application 停用后，APPLICATION/DYNAMIC/HYBRID 请求在 ENFORCE 下都必须拒绝。
5. 历史消费日志保留调用时的 Application Domain/Owner Team 快照。
6. 第一阶段不物理删除 Application。

---

## 6.8 数据表七：`ai_identity_profiles`

该表把 NewAPI Token 转换为企业治理凭证配置。

字段必须包含：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `token_id` | 必填，唯一索引，创建后不可修改 |
| `identity_mode` | STATIC / DYNAMIC / HYBRID |
| `attribution_target_type` | PRINCIPAL / APPLICATION / PLATFORM |
| `identity_assurance` | CREDENTIAL_ONLY / SIGNED_CONTEXT / HYBRID_VERIFIED_CONTEXT |
| `caller_id` | `varchar(128)`，可空；仅强身份 PLATFORM/HYBRID 使用 |
| `caller_name` | `varchar(128)`，可空 |
| `principal_id` | 可空，索引；STATIC/PRINCIPAL 必填 |
| `credential_purpose_id` | 可空，索引；STATIC/PRINCIPAL 必填 |
| `environment` | `varchar(32)`，默认 `prod` |
| `rate_limit_enabled` | bool，默认 false |
| `rate_limit_window_seconds` | int，默认 60；允许 10～3600 |
| `rate_limit_max_requests` | int，默认 0；启用时必须 >0，允许 1～100000 |
| `enabled` | 默认 false，配置完整后显式启用 |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

组合约束必须由 service 层强制：

### STATIC + PRINCIPAL

必须满足：

- `identity_mode=STATIC`
- `attribution_target_type=PRINCIPAL`
- `identity_assurance=CREDENTIAL_ONLY`
- `principal_id` 必填
- `credential_purpose_id` 必填
- `caller_id` 必须为空
- App Binding 数量必须为 0
- `client_verified` 运行时永远为 false

### STATIC + APPLICATION

必须满足：

- `identity_mode=STATIC`
- `attribution_target_type=APPLICATION`
- `identity_assurance=CREDENTIAL_ONLY`
- App Binding 恰好 1 个
- `caller_id` 必须为空
- `principal_id` 可空
- `credential_purpose_id` 可选
- `client_verified` 运行时永远为 false

该模式适合只能证明 API Key、但 Key 固定归属某业务应用的后端服务。

### DYNAMIC + PLATFORM

必须满足：

- `identity_mode=DYNAMIC`
- `attribution_target_type=PLATFORM`
- `identity_assurance=SIGNED_CONTEXT`
- `caller_id` 必填
- App Binding 至少 1 个
- 必须存在 ACTIVE Signing Key
- `principal_id` 第一阶段必须为空

### HYBRID + APPLICATION

必须满足：

- `identity_mode=HYBRID`
- `attribution_target_type=APPLICATION`
- `identity_assurance=HYBRID_VERIFIED_CONTEXT`
- `caller_id` 必填
- App Binding 恰好 1 个
- 必须存在 ACTIVE Signing Key

其他组合全部视为配置非法，Profile 不得启用。

通用规则：

1. 一个 NewAPI `token_id` 最多一个 Identity Profile。
2. `token_id` 必须引用真实存在的 NewAPI Token。
3. Identity Profile 不改变 Token 本身是否有效；运行时首先仍由 TokenAuth 判定 Token。
4. Profile 已启用时禁止修改 `identity_mode`、`attribution_target_type`、`identity_assurance`。
5. 弱身份 Profile 中 `credential_purpose` 只能表示登记用途，不能提升为可信 Caller。
6. `environment` 仅用于区分 prod/test/dev 等治理范围，第一阶段不参与签名。
7. 对 STATIC/PRINCIPAL，管理服务必须检查同一个 `(principal_id, credential_purpose_id, environment)` 是否已存在另一个 enabled Profile；存在则拒绝新启用，确保“一个人一个用途一个当前有效 Key”。如需轮换，先完成新 Token 配置并在切换窗口显式停用旧 Profile；第一阶段不设计长期双活个人 Key。
8. 停用 Profile 不自动停用 NewAPI Token，但 ENFORCE 下使用该 Token 的消费请求必须被企业身份层拒绝。
9. `rate_limit_*` 是企业 Profile 级补充能力，不能修改或覆盖 NewAPI 原 `ModelRequestRateLimit` 的全局/Group 配置。
10. 对 CREDENTIAL_ONLY 个人凭证，生产配置建议默认启用 Profile 级限流；是否强制由第 13 章上线门禁控制。

---

## 6.9 数据表八：`ai_identity_app_bindings`

字段：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `profile_id` | 必填，索引 |
| `app_id` | 必填，索引 |
| `enabled` | 默认 true |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

唯一约束：`profile_id + app_id`。

启用 Profile 时校验：

- STATIC/PRINCIPAL：必须 0 个 Binding；
- STATIC/APPLICATION：必须恰好 1 个；
- DYNAMIC/PLATFORM：至少 1 个；
- HYBRID/APPLICATION：必须恰好 1 个。

---

## 6.10 数据表九：`ai_identity_signing_keys`

只服务 DYNAMIC / HYBRID。

字段：

| 字段 | 约束 |
|---|---|
| `id` | 自增主键 |
| `profile_id` | 必填，索引 |
| `key_id` | `varchar(64)` |
| `secret_ciphertext` | text，不允许通过查询 API 返回 |
| `status` | ACTIVE / RETIRING / REVOKED |
| `not_before` | Unix 秒 |
| `expires_at` | Unix 秒；0 表示未显式过期 |
| `revoked_at` | Unix 秒；未撤销为 0 |
| `created_at` | Unix 秒 |
| `updated_at` | Unix 秒 |

唯一约束：`profile_id + key_id`。

业务规则保持：

1. Signing Secret 与 NewAPI Token Key 完全分离。
2. 一个 Profile 同一时刻只有一个用于新请求签名的 ACTIVE 当前密钥。
3. 轮换时新 Key ACTIVE，旧 ACTIVE 原子变为 RETIRING。
4. RETIRING 在宽限期内可验证旧请求。
5. 默认宽限期 24 小时。
6. REVOKED 不允许恢复。
7. DYNAMIC/HYBRID Profile 启用前至少一个当前可用 ACTIVE Key。
8. API 永不返回 `secret_ciphertext`。

---

## 6.11 数据表十：`ai_identity_audit_events`

用于身份验证失败/降级审计。

字段：

| 字段 | 约束 |
|---|---|
| `id` | bigint 自增主键 |
| `created_at` | Unix 秒，索引 |
| `request_id` | `varchar(64)`，索引 |
| `token_id` | int，索引，未知时 0 |
| `profile_id` | int，索引，未知时 0 |
| `caller_id` | `varchar(128)`，可空 |
| `principal_id` | int，可空，索引 |
| `credential_purpose_id` | int，可空，索引 |
| `identity_mode` | `varchar(16)`，可空 |
| `identity_assurance` | `varchar(32)`，可空 |
| `result` | UNVERIFIED / REJECTED |
| `reason_code` | `varchar(64)`，索引 |
| `claimed_root_app_id` | `varchar(128)`，仅记录合法解析后的声明值 |
| `http_method` | `varchar(16)` |
| `request_path` | `varchar(256)` |
| `client_ip` | `varchar(64)` |

严禁保存 API Key、Signing Secret、Signature、原始 Context、Nonce、Prompt、Response。

---

## 6.12 主数据库迁移要求

以上 10 张表仅进入主库 `DB`。

必须同时加入 `model/main.go`：

- `migrateDB()` AutoMigrate；
- `migrateDBFast()` migrations。

不得加入 `migrateLOGDB()`。

不得修改 ClickHouse `logs` DDL。

数据库兼容：

- SQLite；
- MySQL；
- PostgreSQL。

禁止数据库特定 ENUM。

---

## 6.13 Signing Secret 主密钥规范

新增环境变量：`AI_ATTRIBUTION_MASTER_KEY`。

规范与 V1.0 保持：

- Standard Base64；
- 解码后恰好 32 字节；
- AES-256-GCM；
- Signing Secret 每个 Key 随机 32 原始字节；
- 只在创建/轮换成功响应展示一次 Base64URL 无 padding 明文；
- 数据库只存带版本前缀密文；
- AES-GCM Nonce 12 字节随机值；
- AAD 绑定 `profile_id` 与 `key_id`；
- 不使用 NewAPI `CRYPTO_SECRET`、Session Secret、API Key 替代。

---

## 6.14 第一批 Root 管理 API

统一前缀：`/api/ai-governance`，整个路由组使用 `RootAuth()`。

必须至少提供：

### Business Domain
- GET `/business-domains`
- POST `/business-domains`
- PUT `/business-domains/:id`

### Application Owner Team
- GET `/owner-teams`
- POST `/owner-teams`
- PUT `/owner-teams/:id`

### Usage Team
- GET `/usage-teams`
- POST `/usage-teams`
- PUT `/usage-teams/:id`

### Principal
- GET `/principals`
- GET `/principals/:id`
- POST `/principals`
- PUT `/principals/:id`

至少支持按 Domain、Usage Team、enabled、名称/编号过滤。

### Credential Purpose
- GET `/credential-purposes`
- POST `/credential-purposes`
- PUT `/credential-purposes/:id`

### AI Application
- GET `/applications`
- GET `/applications/:id`
- POST `/applications`
- PUT `/applications/:id`

### Identity Profile
- GET `/identity-profiles`
- GET `/identity-profiles/:id`
- POST `/identity-profiles`
- PUT `/identity-profiles/:id`
- PUT `/identity-profiles/:id/app-bindings`

Profile 查询响应必须同时返回：

- NewAPI Token 元数据：token_id、token_name、status、expired_time、是否 unlimited、IP 限制是否配置、模型限制是否配置；
- Profile 身份模式；
- Attribution Target；
- Assurance；
- Principal/Purpose（如适用）；
- Caller（如适用）；
- App Bindings；
- 计算后的凭证安全姿态；
- Profile 级请求限流配置与最近限流事件摘要。

不得返回 Token Key 明文。

### Signing Key
- GET `/identity-profiles/:id/signing-keys`
- POST `/identity-profiles/:id/signing-keys/generate`
- POST `/identity-profiles/:id/signing-keys/rotate`
- POST `/identity-profiles/:id/signing-keys/:key_id/revoke`

### Identity Audit
- GET `/identity-audit-events`

---

## 6.15 弱身份凭证安全姿态

第一阶段不重复建设 NewAPI 已经具备的 Token 安全策略。

复用：

- IP：Token AllowIps；
- 模型：Token ModelLimits；
- 总额度：Token RemainQuota/UnlimitedQuota；
- 有效期：Token ExpiredTime。

额外补齐两类 NewAPI 单用户模式下缺少的企业治理能力：

1. Profile 级请求频率限制；
2. 凭证风险、轮换与异常提示。

### 6.15.1 Profile 级请求频率限制

当前 NewAPI 原模型请求限流按 `user_id`/Group 计数。在企业只有一个 NewAPI User 时，所有个人 Key 会共享计数，因此不能用于“一人一 Key”独立防滥用。

不得修改原限流器键语义。

新增独立 `AICredentialRateLimit`，按 `profile_id` 计数。

配置字段使用 `ai_identity_profiles.rate_limit_*`。

Redis 语义固定：

- key 前缀：`ai:credential:rate:<profile_id>`；
- 采用滑动时间窗；
- 时间窗 `rate_limit_window_seconds`；
- 最大请求数 `rate_limit_max_requests`；
- 并发判断与计数必须原子完成，使用 Redis Lua/事务保证，不允许“先查再写”竞态；
- 成员使用 NewAPI `request_id` 加毫秒时间形成唯一成员；
- Redis key 过期时间至少为窗口长度 + 60 秒。

顺序：

```text
TokenAuth
→ AIIdentityAuth
→ AICredentialRateLimit
→ 原 ModelRequestRateLimit
→ Distribute
```

限流发生在 Provider/Distribute 之前。

Redis 故障：

- AUDIT：允许请求继续，记录 `CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE`；
- ENFORCE 且 Profile 明确启用了 per-credential rate limit：503，fail closed；
- disabled：不执行企业凭证级限流。

### 6.15.2 Risk Posture

至少计算：

- `ip_restricted`；
- `model_restricted`；
- `quota_restricted`；
- `expiry_configured`；
- `rate_limit_enabled`；
- `credential_only`；
- `rotation_overdue`。

凭证轮换周期使用环境变量：

`AI_CREDENTIAL_ROTATION_DAYS`

- 默认 90；
- 允许 30～365；
- 根据 NewAPI Token `CreatedTime` 判断是否超过建议轮换周期；
- 不自动轮换 API Key，因为 WorkBuddy/IDE 客户端仍需人工更新；
- `ExpiredTime` 是更强的强制失效闸。

风险状态：

- `LOWER_RISK`
- `MEDIUM_RISK`
- `HIGH_RISK`

固定底线：

1. CREDENTIAL_ONLY 且无 IP、无模型限制、无限额度、无过期、无 Profile 级限流 → HIGH_RISK。
2. `client_verified=false` 本身不会被安全姿态“升级”为 true。
3. Risk 只用于治理提示；第一阶段不因 HIGH_RISK 自动拒绝，除非第 13 章上线门禁明确要求。
4. User-Agent、请求体特征、普通 Header 不参与身份可信等级。
5. Token 超过轮换周期必须在管理页面显示“轮换逾期”。

### 6.15.3 第一阶段异常检测边界

第一阶段不建设复杂机器学习异常检测。

必须记录确定性风险事件：

- `CREDENTIAL_RATE_LIMIT_EXCEEDED`
- `CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE`
- `CREDENTIAL_ROTATION_OVERDUE`
- `CREDENTIAL_HIGH_RISK`

TokenAuth 原生 IP/模型/过期/额度拒绝如现有日志能够关联 token_id，则在治理页面聚合展示；不得为了这一点大改 TokenAuth。

第六批 Usage Projection 完成后，再增加简单阈值型异常提示：

- 单 Credential 小时请求量超过阈值；
- 单 Credential 小时 Token/Quota 超过阈值；
- 相比最近 7 天同小时基线达到明确倍数阈值。

异常第一阶段只告警/审计，不自动封禁 Token。

---

## 6.16 管理操作审计

新增管理 API 必须使用 RootAuth，并补充语义化 action：

- business_domain.create/update；
- owner_team.create/update；
- usage_team.create/update；
- principal.create/update；
- credential_purpose.create/update；
- application.create/update；
- identity_profile.create/update；
- identity_profile.replace_bindings；
- signing_key.generate/rotate/revoke。

不得记录 Signing Secret 明文或 Token Key 明文。

---

## 6.17 第一批缓存边界

第一批 service 层预留按 `token_id` 获取完整 Identity Snapshot 的统一方法。

Snapshot 至少一次得到：

- Profile；
- Principal + Usage Domain + Usage Team（如适用）；
- Credential Purpose（如适用）；
- Enabled App Bindings；
- Application + Application Domain + Owner Team（如适用）；
- 可用 Signing Key 元数据/解密能力；
- NewAPI Token 安全姿态摘要。

第二批中间件不得自己拼多表查询。

---

## 6.18 第一批测试门禁

至少覆盖：

1. SQLite 全新库可迁移。
2. 公司实际使用的 MySQL/PostgreSQL 至少一种真实迁移，代码仍兼容另一种。
3. Domain、Owner Team、Usage Team、Principal、Purpose、App code 唯一约束。
4. code 创建后修改拒绝。
5. Disabled Domain 不能分配给新 App/Principal。
6. Disabled Owner Team 不能分配给新 App。
7. Disabled Usage Team 不能分配给新 Principal。
8. 同一 token_id 第二个 Profile 拒绝。
9. STATIC/PRINCIPAL 缺 Principal 或 Purpose 拒绝。
10. STATIC/PRINCIPAL 存在 App Binding 拒绝。
11. STATIC/APPLICATION 0/2 个 Binding 拒绝。
12. DYNAMIC/PLATFORM 0 个 Binding 拒绝。
13. DYNAMIC/PLATFORM 缺 Caller 或 Signing Key 拒绝。
14. HYBRID/APPLICATION 0/2 个 Binding 拒绝。
15. 非法 identity_mode/target/assurance 组合拒绝。
16. CREDENTIAL_ONLY Profile 不能配置成 `client_verified=true`，数据库中不应存在可写 client_verified 字段。
17. 同 `(principal,purpose,environment)` 第二个 enabled STATIC/PRINCIPAL Profile 拒绝。
18. Signing Secret 数据库不存在明文。
19. 错 Master Key 解密失败。
20. generate/rotate 只本次返回 Secret。
21. GET Key 永不返回明文/密文。
22. rotate 后旧 Key 进入 RETIRING。
23. 所有写管理 API 产生管理审计。
24. Risk Posture 对无 IP/Model/Quota/Expiry/RateLimit 的 CREDENTIAL_ONLY 返回 HIGH_RISK。
25. rate_limit_enabled=true 但 window/max 参数非法时 Profile 不能启用。
26. 当前 NewAPI 原 `ModelRequestRateLimit` 不被修改。
27. 未改造模型请求行为与基线一致。

第一批不满足上述门禁，不得进入第二批。

---

# 7. 第二批：运行时身份认证

## 7.1 第二批目标

把企业身份治理真正接到模型调用入口，完成：

- STATIC/PRINCIPAL 弱身份凭证归因；
- STATIC/APPLICATION 固定应用归因；
- DYNAMIC/PLATFORM 强身份；
- HYBRID/APPLICATION 强上下文；
- HMAC-SHA256；
- Timestamp；
- Nonce 防重放；
- App Binding；
- Profile 级凭证请求限流；
- Trusted Attribution Context；
- AUDIT / ENFORCE；
- 内部 Header 清理。

第二批不修改 Token/Quota/Provider 逻辑。

---

## 7.2 企业协议 Header 固定

只有 DYNAMIC 和 HYBRID 使用以下六个 Header：

- `X-AI-Context-Version`
- `X-AI-Context`
- `X-AI-Timestamp`
- `X-AI-Nonce`
- `X-AI-Key-Id`
- `X-AI-Signature`

STATIC 请求不需要这些 Header。

不得扩展一组平铺的 Root App/Workflow Header。

---

## 7.3 X-AI-Context 编码规范

保持 UTF-8 JSON → Base64URL 无 padding。

解码后最大 6144 bytes；编码 Header 最大 8192 字符。

严格 Schema，未知字段拒绝。

允许字段：

| 字段 | DYNAMIC | HYBRID | 规则 |
|---|---|---|---|
| `root_app_id` | 必填 | 可省略 | HYBRID 如传入必须等于固定 App |
| `root_run_id` | 必填 | 必填 | 1～128 字符 |
| `current_execution_id` | 可选 | 可选 | 有任意当前执行字段时必须存在 |
| `parent_execution_id` | 可选 | 可选 | 需要 current_execution_id |
| `execution_type` | 可选 | 可选 | 有任意当前执行字段时必须存在 |
| `execution_depth` | 可选 | 可选 | 0～64 |
| `workflow_id` | 可选 | 可选 | 需要 current_execution_id |
| `agent_id` | 可选 | 可选 | 需要 current_execution_id |
| `task_id` | 可选 | 可选 | 需要 current_execution_id |
| `node_id` | 可选 | 可选 | 需要 current_execution_id |

客户端不得提供：

- caller_id；
- principal_id；
- credential_owner；
- credential_purpose；
- usage_team；
- business_domain；
- owner_team；
- identity_mode；
- identity_assurance；
- identity_verified；
- client_verified；
- token_id；
- signing_secret。

这些全部由 NewAPI 决定。

---

## 7.4 签名原文固定

DYNAMIC/HYBRID 必须签名。

Canonical String 七行保持：

1. `v1`
2. HTTP Method 大写
3. URL Path，不含 query
4. Timestamp 原始字符串
5. Nonce 原始字符串
6. KeyId 原始字符串
7. X-AI-Context 原始 Base64URL

LF 分隔，末行后无 LF。

HMAC-SHA256，64 位小写十六进制，常量时间比较。

---

## 7.5 Timestamp 固定规则

`X-AI-Timestamp` 为 Unix 秒。

默认允许 ±300 秒；环境变量 `AI_ATTRIBUTION_CLOCK_SKEW_SECONDS` 范围 60～900，默认 300。

---

## 7.6 Nonce 固定规则

Nonce 推荐 16～32 原始随机字节后 Base64URL。

接受长度 22～64，只允许 Base64URL 字符。

Redis Key：

`ai:identity:nonce:<profile_id>:<nonce>`

默认 TTL 600 秒。

校验顺序：

```text
格式
→ Profile/Key
→ Timestamp
→ HMAC
→ Caller/App 配置
→ Redis SET NX
→ Trusted Context
```

---

## 7.7 Redis 故障语义

STATIC 不依赖 Nonce Redis。

DYNAMIC/HYBRID：

### AUDIT

Redis 不可用：

- 请求允许；
- `credential_verified=true`，因为 API Key 已通过 TokenAuth；
- `client_verified=false`；
- `identity_verified=false`；
- `identity_assurance` 保留配置值；
- reason=`REPLAY_STORE_UNAVAILABLE`；
- 不使用客户端 root_app 形成正式应用归因；
- 写 Audit Event。

### ENFORCE

Redis 不可用返回 503，fail closed。

---

## 7.8 Identity Runtime Mode

`AI_ATTRIBUTION_MODE`：

- disabled
- audit
- enforce

默认 disabled。

### disabled

不执行企业归因，但始终删除入站 `X-AI-*`。

### audit

合法请求生成完整 Trusted Context。

非法/缺失请求继续模型调用，但只保留能由 Token/Profile 静态确定的可信事实。

规则：

- TokenAuth 成功时 `credential_verified=true`；
- 不得因为 Profile 写着 `purpose=workbuddy` 就把 `client_verified=true`；
- DYNAMIC/HYBRID 验证失败时，客户端声明的 root_app/run/execution 全部不可进入正式归因；
- STATIC/PRINCIPAL 若 Profile/Principal/Purpose 配置合法，可以保留 Principal/Purpose，因为这些是网关静态主数据；
- 写 Identity Audit Event。

### enforce

只有符合对应组合规则的消费请求才能继续。

---

## 7.9 STATIC/PRINCIPAL 行为固定

典型场景：张三的 WorkBuddy Key、IDE Key。

运行时：

```text
TokenAuth
→ token_id
→ Profile(STATIC/PRINCIPAL/CREDENTIAL_ONLY)
→ Principal
→ Usage Business Domain
→ Usage Team
→ Credential Purpose
→ Trusted Context
```

必须设置：

- `credential_verified=true`
- `client_verified=false`
- `identity_verified=true`：表示网关已完成本模式下可完成的归因验证；不得被解释为客户端验证
- `identity_assurance=CREDENTIAL_ONLY`
- caller 字段为空
- root_app 字段为空
- application domain/owner team 为空
- usage principal/domain/team/purpose 有值

STATIC/PRINCIPAL 不生成伪造 Root App，也不使用 RequestId 假装 Root Run。

如果后续统计需要请求级相关性，使用 NewAPI `request_id`，不要把它冒充 `root_run_id`。

请求主动携带 X-AI-* 时全部删除，不允许覆盖静态主数据。

---

## 7.10 STATIC/APPLICATION 行为固定

适用于一个 API Key 固定归属于一个业务应用，但客户端本身无法密码学证明的场景。

运行时：

```text
TokenAuth
→ Profile(STATIC/APPLICATION/CREDENTIAL_ONLY)
→ 唯一 App Binding
→ Application
→ Application Domain
→ Owner Team
→ Trusted Context
```

设置：

- credential_verified=true
- client_verified=false
- identity_assurance=CREDENTIAL_ONLY
- root_app 固定
- root_run 为空
- caller 为空

不得声称“客户端已验证”。

---

## 7.11 DYNAMIC/PLATFORM 行为固定

必须：

- Profile enabled；
- Caller 从 Profile 取得；
- Context 存在；
- root_app_id/root_run_id 必填；
- Signing Key 可用；
- HMAC/Timestamp/Nonce 全部有效；
- App 在 Binding 中；
- Application enabled；
- Domain/Owner Team 从 Registry 查询。

成功：

- credential_verified=true
- client_verified=true
- identity_verified=true
- identity_assurance=SIGNED_CONTEXT

---

## 7.12 HYBRID/APPLICATION 行为固定

必须：

- Profile 恰好绑定一个 App；
- Caller 从 Profile 取得；
- Root App 由 Binding 固定；
- Context 存在；
- root_run_id 必填；
- 若 Context 提供 root_app_id，必须与固定 App 一致；
- HMAC/Timestamp/Nonce 有效。

成功：

- credential_verified=true
- client_verified=true
- identity_verified=true
- identity_assurance=HYBRID_VERIFIED_CONTEXT

---

## 7.13 Trusted Attribution Context 固定结构

运行时只生成一个统一对象。

### 凭证事实

- token_id；
- profile_id；
- credential_verified；
- environment。

### 身份模式与可信等级

- identity_mode；
- attribution_target_type；
- identity_assurance；
- identity_source；
- identity_verified；
- client_verified；
- failure_reason。

### 弱身份个人归因

- principal_id；
- principal_code；
- principal_name；
- credential_purpose_id；
- credential_purpose_code；
- credential_purpose_name；
- usage_business_domain_id/code/name；
- usage_team_id/code/name。

### 强身份 Caller

- caller_id；
- caller_name。

### 应用归因

- root_app_id；
- root_app_name；
- application_business_domain_id/code/name；
- owner_team_id/code/name。

### 执行

- root_run_id；
- current_execution_id；
- parent_execution_id；
- execution_type；
- execution_depth；
- workflow_id；
- agent_id；
- task_id；
- node_id。

### 签名元数据

- signing_key_id。

绝对不得进入：

- Signing Secret；
- Signature；
- Nonce；
- 原始 Context；
- API Key 明文。

---

## 7.14 Header 删除要求

AIIdentityAuth 先复制需要的 Header，再在任何 `c.Next()` 之前删除六个企业身份 Header。

disabled/audit/enforce 都一样。

必须测试 Channel Header Override 的 `*` 与正则 passthrough。

---

## 7.15 运行时配置缓存

按 `token_id` 缓存不可变 Identity Snapshot。

Snapshot 包含：

- Profile；
- Principal/Usage Domain/Usage Team/Purpose；
- App Binding/Application/App Domain/Owner Team；
- Signing Key；
- NewAPI Token 安全姿态摘要。

TTL：

- AUDIT/disabled 30 秒；
- ENFORCE 10 秒。

管理 API 修改后当前节点立即失效。

多实例其他节点靠 TTL；紧急撤销同时禁用原 NewAPI Token。

---

## 7.16 需要归因的请求范围

保持 V1.0 的消费入口覆盖原则。

第一阶段必须归因所有真实产生模型/生成式任务消费的 POST/WS 入口，包括 `/v1` chat/messages/responses/images/audio/embeddings/rerank/moderation、Gemini `/v1beta`、Suno/MJ/Video/Kling/Jimeng 等实际消费入口。

模型列表 GET、纯任务状态查询、纯结果下载不要求“新消费身份验证”，但第 5 批需要任务访问隔离。

必须继续使用集中 `IsAttributionRequired(method, route/path)` 分类器，不允许散落 if 判断。

---

## 7.17 凭证级请求限流运行时

`AICredentialRateLimit` 只在以下条件同时满足时执行：

- Runtime Mode != disabled；
- Profile 已成功解析；
- Profile `rate_limit_enabled=true`。

限流 Key 必须使用 `profile_id`，不得使用 `user_id`，也不得使用 `principal_id`。原因是同一个人不同 Purpose 必须拥有独立桶。

```text
张三 / WorkBuddy / Profile 101
→ 独立限流桶

张三 / IDE / Profile 102
→ 另一个限流桶
```

超过阈值：

- HTTP 429；
- error code=`AI_CREDENTIAL_RATE_LIMIT_EXCEEDED`；
- Provider 不收到请求；
- 写风险/身份审计事件；
- 不修改 NewAPI 原 ModelRequestRateLimit 计数语义。

Redis 不可用：

- audit：放行并记录 `AI_CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE`；
- enforce：对启用了该 Guardrail 的 Profile 返回 503。

---

## 7.18 Router 接入要求

在每个消费路由 `TokenAuth` 后、`Distribute` 前插 `AIIdentityAuth()`。

不得重构整个 relay-router。

模型列表等只读 Router 不挂 AIIdentityAuth。

---

## 7.19 固定错误码

在 V1.0 基础上补充：

- `AI_IDENTITY_PRINCIPAL_REQUIRED`
- `AI_IDENTITY_PRINCIPAL_DISABLED`
- `AI_IDENTITY_PURPOSE_REQUIRED`
- `AI_IDENTITY_PURPOSE_DISABLED`
- `AI_IDENTITY_USAGE_TEAM_INVALID`
- `AI_IDENTITY_ASSURANCE_INVALID`
- `AI_IDENTITY_TARGET_INVALID`
- `AI_IDENTITY_DUPLICATE_ACTIVE_PERSONAL_CREDENTIAL`
- `AI_CREDENTIAL_RATE_LIMIT_EXCEEDED`
- `AI_CREDENTIAL_RATE_LIMIT_STORE_UNAVAILABLE`

并保留：

- PROFILE_REQUIRED/DISABLED；
- CONTEXT_REQUIRED/INVALID/TOO_LARGE；
- TIMESTAMP；
- NONCE；
- KEY；
- SIGNATURE；
- REPLAY；
- APP_NOT_BOUND；
- APP_DISABLED；
- HYBRID_APP_MISMATCH 等。

---

## 7.20 第二批测试门禁

### STATIC/PRINCIPAL

1. 张三 + WorkBuddy Profile 正常 API Key 通过。
2. Trusted Context 中 Principal/Usage Domain/Usage Team/Purpose 正确。
3. `credential_verified=true`。
4. `client_verified=false`。
5. caller/root_app/root_run 为空。
6. 多带 X-AI-* 不得覆盖任何静态属性。
7. Principal disabled：ENFORCE 拒绝。
8. Purpose disabled：ENFORCE 拒绝。
9. 同一个 Key 拿去 curl 仍会通过凭证验证——测试必须明确证明系统不会错误标成 `client_verified=true`。

### STATIC/APPLICATION

1. 固定 App/Domain/Owner Team 正确。
2. client_verified=false。
3. root_run 为空。
4. X-AI-* 无法覆盖 App。

### DYNAMIC

保持完整 HMAC、JSON、长度、未知字段、root_app、root_run、Method/Path、Key、App Binding、Timestamp、Replay 正反测试。

成功必须同时验证：

- credential_verified=true；
- client_verified=true；
- assurance=SIGNED_CONTEXT。

### HYBRID

固定 App + 动态 root_run/execution，成功 assurance=HYBRID_VERIFIED_CONTEXT。

### AUDIT

- 强身份签名失败：保留可信 Token/Profile 元数据，但不得采用 root_app；
- STATIC/PRINCIPAL 配置合法：可保留 Principal/Purpose；
- Redis 故障：强身份降为 unverified，不采用客户端应用上下文；
- Audit Event reason 正确。

### 凭证级限流

- 张三 WorkBuddy 与张三 IDE 使用不同 Profile 时计数完全独立；
- 张三和李四计数独立；
- 同 Profile 达到阈值后下一个请求 429；
- 被限流请求不能进入 Distribute/Provider；
- 原 NewAPI `ModelRequestRateLimit` 仍可同时工作且保持 user_id/Group 语义；
- audit Redis 故障放行并审计；
- enforce Redis 故障在该 Profile 启用限流时 503。

### Header 泄漏

Provider 不得收到 X-AI-*。

第二批失败不得进入第三批。

---

# 8. 第三批：消费事实归因与可查询日志

## 8.1 第三批目标

把 Trusted Attribution 与 NewAPI 已有模型消费事实结合。

第一阶段不修改 `logs` 正式列，不修改 ClickHouse DDL，统一写 `Log.Other`。

---

## 8.2 RelayInfo 扩展

`RelayInfo` 只增加一个统一归因快照字段/指针，例如逻辑上：

- `Attribution *TrustedAttributionSnapshot`

不得新增几十个散落字段。

---

## 8.3 Consume Log 写入规则

在 `RecordConsumeLog` 集中路径中，将 Trusted Attribution 强制写入：

`params.Other["ai_attribution"]`

上游如已有同名值必须被 Trusted 数据覆盖。

快照按两类语义保存。

### 通用字段

- profile_id；
- token_id；
- environment；
- identity_mode；
- attribution_target_type；
- identity_assurance；
- identity_source；
- identity_verified；
- credential_verified；
- client_verified；
- failure_reason。

### 弱身份个人字段

仅有值时写：

- principal_id/code/name；
- credential_purpose_id/code/name；
- usage_business_domain_id/code/name；
- usage_team_id/code/name。

### 强身份/应用字段

仅有值时写：

- caller_id/name；
- root_app_id/name；
- application_business_domain_id/code/name；
- owner_team_id/code/name；
- root_run_id；
- current execution fields；
- signing_key_id。

严禁保存 API Key、Signing Secret、Signature、Nonce、原始 Context。

---

## 8.4 WorkBuddy 弱身份日志语义

例如张三 WorkBuddy Key：

```text
principal = 张三
usage_business_domain = 财务
usage_team = 财务数字化组
credential_purpose = WorkBuddy
identity_mode = STATIC
identity_assurance = CREDENTIAL_ONLY
credential_verified = true
client_verified = false
root_app = empty
caller = empty
```

查询 UI 和导出必须使用“登记用途”措辞，不得展示“Verified WorkBuddy Client”。

---

## 8.5 强身份平台日志语义

例如工作流平台：

```text
caller = workflow-platform-prod
root_app = hr_assistant
application_business_domain = 人力
owner_team = AI应用组
identity_assurance = SIGNED_CONTEXT
credential_verified = true
client_verified = true
root_run = run_xxx
```

---

## 8.6 Error Log 写入规则

`RecordErrorLog` 能拿到 Trusted Context 时写相同快照。

AIIdentityAuth 阶段拒绝的请求走 `ai_identity_audit_events`。

---

## 8.7 Legacy Log 语义

上线前 Log 视为 `legacy_unattributed`，不得解释成验证失败。

---

## 8.8 历史主数据快照

Principal 的 Domain/Usage Team 变化、Application 的 Domain/Owner Team 变化，都只影响未来调用。

历史日志保存调用时快照。

---

## 8.9 不修改 QuotaData

不把 Principal/Purpose/App/Domain/Team 塞进现有 QuotaData。

企业统计留第 6 批。

---

## 8.10 第三批测试门禁

1. WorkBuddy 弱身份 Consume Log 有 Principal/Usage Domain/Usage Team/Purpose。
2. WorkBuddy 日志 `credential_verified=true`、`client_verified=false`。
3. WorkBuddy 日志 caller/root_app 不被伪造填充。
4. STATIC/APPLICATION 有 App Domain/Owner Team，client_verified=false。
5. DYNAMIC/HYBRID 有 Caller/App/Run 且 client_verified=true。
6. Error Log 同样有 Attribution。
7. 原 Other 中 model_ratio/cache/billing 保持。
8. 伪造 ai_attribution 被 Trusted 数据覆盖。
9. 日志无 Secret/Signature/Nonce/API Key。
10. Principal 或 App 主数据迁移后，新旧日志快照正确。
11. Token/Quota/渠道统计与基线一致。
12. SQLite/MySQL/Postgres/ClickHouse 无 schema 变更即可写 Other。

---

# 9. 第四批：OpenTelemetry、W3C Trace Context 与 GenAI

## 9.1 第四批目标

NewAPI 从当前 RequestId + Consume Log + PerfMetrics 的内部观测，升级为可加入企业分布式调用链的 OpenTelemetry 节点。

第一阶段只实现：

- Trace；
- GenAI Span；
- GenAI Client Metrics；
- W3C Trace Context；
- HTTP Client 子 Span；
- 企业业务属性；
- 弱身份凭证责任人/用途/可信等级属性。

不实施完整 OTel Logs，也不采集 Prompt/Response。

---

## 9.2 依赖版本冻结

基线 `go.mod` 当前只有间接 OpenTelemetry 1.34.0。

第一阶段 OTel 改造将版本明确提升并直接固定：

- OpenTelemetry Go core / SDK / OTLP exporter：`v1.45.0` 同一版本线；
- `otelgin`：`v0.69.0`；
- `otelhttp`：`v0.69.0`。

版本冻结依据 2026-08-20 官方稳定发布：OpenTelemetry Go core `v1.45.0`；Go Contrib 当前稳定 release line 使用 `v0.69.0`。不得使用 main 分支伪版本替代稳定版。

实现者不得执行会升级全仓无关依赖的操作，不得 `go get -u ./...`。

如果上述精确版本在本地 Go Module 解算时发生直接不兼容，必须停止并报告依赖冲突，不能自行切换另一个 OTel 大版本或降级 NewAPI 其他依赖。

---

## 9.3 OTel 配置

至少支持：

- `OTEL_ENABLED=false`，默认 false；
- `OTEL_SERVICE_NAME=new-api`；
- `OTEL_EXPORTER_OTLP_ENDPOINT`；
- `OTEL_EXPORTER_OTLP_PROTOCOL`：grpc 或 http/protobuf；
- `OTEL_TRACES_SAMPLER`；
- `OTEL_TRACES_SAMPLER_ARG`。

第一阶段 POC 建议采样率 1.0；正式生产由部署配置决定。

`OTEL_ENABLED=false` 时不得改变模型请求成功/失败语义。

Exporter 短暂不可用不得阻塞正常模型调用；配置本身非法导致初始化不能完成时允许启动失败并给明确错误。

---

## 9.4 OTel 初始化位置

在 `InitResources()` 中完成环境变量和 Logger 初始化后初始化 Telemetry，且必须在 HTTP Server 接收请求前完成。

主程序 graceful shutdown 时必须调用 Telemetry Shutdown/Flush，并受已有 shutdown timeout 控制。

不得创建无法回收的第二套后台 exporter 生命周期。

---

## 9.5 Propagator

第一阶段全局 Propagator 使用 W3C Trace Context：

- `traceparent`
- `tracestate`

第一阶段不要默认启用 Baggage 传播企业业务身份，避免任意业务 metadata 跨边界泄漏。

企业业务归因通过 Trusted Attribution 设置为 Span Attributes，而不是通过 OTel Baggage 充当安全身份机制。

---

## 9.6 HTTP Server Span

使用 `otelgin` 在 Gin HTTP 入口建立 Server Span。

放置顺序要求：

- NewAPI RequestId 先建立；
- OTel Server Middleware 随后进入；
- 路由和 TokenAuth/AIIdentityAuth 在其下。

AIIdentityAuth 验证成功后必须把企业属性追加到当前 Server Span，而不是新建第二个“身份 Span”。

没有入站 `traceparent`：NewAPI 创建新 Trace。

有合法入站 `traceparent`：NewAPI Server Span 加入调用方现有 Trace。

---

## 9.7 GenAI 逻辑 Span

每一次真正模型操作必须建立一个 GenAI CLIENT Span。

该 Span 表达“客户端请求 NewAPI 完成的一次逻辑模型操作”，必须覆盖 NewAPI 自己的 Channel Retry 生命周期，而不是每重试一次创建一条彼此独立的 GenAI 业务操作。

预期层级：

```text
上游应用 Span
  └─ NewAPI HTTP SERVER
       └─ GenAI logical CLIENT
            ├─ Provider HTTP attempt 1
            ├─ Provider HTTP attempt 2（如重试）
            └─ ...
```

Provider HTTP attempt 由 `otelhttp` 形成底层 HTTP 子 Span。

GenAI logical Span 要等模型响应/Usage/最终结算事实可用后结束，才能记录最终 Token 和错误结果。

流式调用要覆盖到最后一个有效模型输出/流结束，而不是收到 Provider HTTP Header 就结束。

---

## 9.8 GenAI Operation Name 映射

第一阶段固定：

| NewAPI 操作 | `gen_ai.operation.name` |
|---|---|
| chat/completions、Claude messages、Responses | `chat` |
| legacy completions | `text_completion` |
| embeddings | `embeddings` |
| 图像/音频/视频等内容生成 | `generate_content` |
| rerank | `rerank` |
| moderation | `moderation` |
| alpha search / 检索类 Search | `retrieval` |

`rerank`、`moderation` 当前作为没有对应标准预定义值时的受控自定义 operation；检索类 Search 使用标准预定义 `retrieval`。不得为了统一而全部伪装成 `chat`。

---

## 9.9 GenAI 标准属性

第一阶段至少实现：

- `gen_ai.operation.name`
- `gen_ai.provider.name`
- `gen_ai.request.model`
- `gen_ai.response.model`（可确定时）
- `gen_ai.usage.input_tokens`
- `gen_ai.usage.output_tokens`
- cache read / cache creation token 属性（Provider 能提供时）
- `server.address`
- `error.type`（错误时）

Span Name 使用当前语义约定推荐形式：

`{gen_ai.operation.name} {gen_ai.request.model}`

不得把 Root App 拼入 Span Name，避免高基数 Span 名称。

---

## 9.10 Provider 映射

Provider 必须通过集中函数由 NewAPI Channel Type 映射，不允许各 Adapter 自己填。

至少明确：

- OpenAI → `openai`；
- Azure OpenAI → `azure.ai.openai`；
- Anthropic → `anthropic`；
- AWS Bedrock → `aws.bedrock`；
- Gemini API（`generativelanguage.googleapis.com`）→ `gcp.gemini`；
- Vertex AI → `gcp.vertex_ai`。

其他 Channel Type 必须在一个集中映射表里给稳定小写名称。

不能因为某渠道提供 OpenAI-compatible API，就一律记成 `openai`；Provider 表示真实提供方/系统，协议格式是另一个概念。

---

## 9.11 Token 语义必须使用 NewAPI 已归一化 Usage

不能从原 Provider Response 任意字段直接生成 OTel Token，也不能使用已经乘倍率后的 Quota 反推 Token。

必须复用 NewAPI 已有 BillingUsage / Usage 归一化结果。

### OpenAI 语义

- input = PromptTokens；如当前响应模式只提供 InputTokens，则使用已归一化后的对应输入总量；
- output = CompletionTokens；对应 Responses 模式使用已归一化 OutputTokens；
- cached tokens 是输入总量中的子集，不能再次加到 input 上形成双计数。

### Anthropic 语义

Anthropic 的普通 `input_tokens` 不等于完整有效输入。

完整 input 必须按已有 Claude Usage 语义组合：

- InputTokens；
- CacheCreationInputTokens；
- CacheReadInputTokens。

即：input total = 三者之和。

输出使用 OutputTokens。

### Gemini 语义

`promptTokenCount` 已代表有效 Prompt 总量，并包含 cached content；`cachedContentTokenCount` 是其中子集。

因此：

- input = promptTokenCount；
- output = candidatesTokenCount；
- cachedContentTokenCount 单独记录缓存维度，不能再次加到 input。

### 思考/推理 Token

如果 Provider 的 output total 已经包含 reasoning/thought token，不得再额外叠加；可以额外记录细分属性，但总数不能重复。

---

## 9.12 GenAI Metrics

第一阶段至少输出：

- `gen_ai.client.token.usage`；
- `gen_ai.client.operation.duration`。

流式且 `FirstResponseTime` 可靠时输出：

- `gen_ai.client.operation.time_to_first_chunk`。

Metric Labels 只能使用受控低基数维度，例如：

- gen_ai.operation.name；
- provider；
- model；
- caller（仅强身份且低基数平台 Caller）；
- root_app；
- application_business_domain；
- owner_team；
- usage_business_domain；
- usage_team；
- credential_purpose；
- identity_assurance；
- status/result。

不得把 `principal_id/principal_name` 作为普通 Metric Label，个人数量属于高基数维度。

不得作为普通 Metric Label：

- root_run_id；
- current_execution_id；
- node_id；
- request_id；
- trace_id；
- span_id。

这些只进入 Span/Log。

---

## 9.13 企业 Span Attributes

统一使用以下企业属性；字段不存在时不写空字符串。

### 通用凭证与可信等级

- `company.ai.gateway.credential_id`
- `company.ai.gateway.request_id`
- `company.ai.identity.mode`
- `company.ai.identity.target_type`
- `company.ai.identity.assurance`
- `company.ai.identity.verified`
- `company.ai.credential.verified`
- `company.ai.client.verified`
- `company.ai.environment`

### 弱身份个人凭证

- `company.ai.principal.id`
- `company.ai.principal.code`
- `company.ai.principal.name`
- `company.ai.credential.purpose.id`
- `company.ai.credential.purpose.code`
- `company.ai.credential.purpose.name`
- `company.ai.usage_business_domain.id`
- `company.ai.usage_business_domain.code`
- `company.ai.usage_business_domain.name`
- `company.ai.usage_team.id`
- `company.ai.usage_team.code`
- `company.ai.usage_team.name`

注意：Principal 这些字段允许进入 Span/Log，但不得默认作为常规 Metrics Label。

### 强身份 Caller 与应用

- `company.ai.caller.id`
- `company.ai.root_app.id`
- `company.ai.root_app.name`
- `company.ai.application_business_domain.id`
- `company.ai.application_business_domain.code`
- `company.ai.application_business_domain.name`
- `company.ai.owner_team.id`
- `company.ai.owner_team.code`
- `company.ai.owner_team.name`
- `company.ai.root_run.id`
- `company.ai.current_execution.id`
- `company.ai.parent_execution.id`
- `company.ai.execution.type`
- `company.ai.execution.depth`
- `company.ai.workflow.id`
- `company.ai.agent.id`
- `company.ai.task.id`
- `company.ai.node.id`

费用第一阶段只记录 NewAPI 权威 Quota：

- `company.ai.cost.quota`

不得创建 `gen_ai.cost.*`。

---

## 9.14 出站 HTTP Context 修正

当前共享请求构造使用 `http.NewRequest`，没有天然继承 `c.Request.Context()`。

第四批必须在统一请求创建边界修正：所有 Provider HTTP 请求使用当前 Gin Request Context，使 GenAI Span 成为 Provider HTTP Span 的父级，并允许 OTel Propagator 注入正确的 `traceparent`。

修改集中在：

- `DoApiRequest`；
- `DoFormRequest`；
- `DoTaskApiRequest`；
- 其他同一共享工厂中确实创建 Provider HTTP Request 的位置。

不得逐个 Provider 修改。

---

## 9.15 HTTP Client Instrumentation

`service/http_client.go` 的集中 Transport 构建处，在 `OTEL_ENABLED=true` 时使用 `otelhttp` 包装基础 RoundTripper。

必须继续保留：

- Proxy；
- TLS；
- HTTP/2；
- Shards；
- Pool；
- Redirect；
- Timeout；

现有 NewAPI 行为。

OTel 只包装 RoundTripper，不得重写现有连接池架构。

---

## 9.16 Trace Header 与 Channel Passthrough

为了避免 Channel Header `*` passthrough 把调用方原始 `traceparent` 原封不动转发，同时 OTel Transport 又注入新的上下文：

第四批必须将 `traceparent` 和 `tracestate` 加入通用 Header Passthrough 的跳过名单。

Provider 出站 Trace Context 只能由 OTel Propagator 根据当前 Child Span 注入。

---

## 9.17 WebSocket Realtime

WebSocket 出站当前使用 Dialer。

第四批必须：

- 使用带 Context 的 Dial；
- 在目标 Header 中通过 OTel Propagator 注入当前 Context；
- 保持现有 WebSocket Protocol Header 行为；
- 不允许企业 X-AI-* Header 进入 Provider WS 握手。

无需为每个 Realtime Provider 单独改适配器。

---

## 9.18 不采集 Prompt/Response

第一阶段 OTel 默认不记录：

- Prompt 文本；
- system prompt；
- tool arguments 全文；
- model output 全文。

即使 OTel GenAI 语义存在可选事件，也不启用内容采集。

第一阶段只记录身份、模型、Token、Quota、Latency、Status、Error、Trace。

---

## 9.19 Identity 自身 Metric

允许增加：

`company.ai.identity.verification`

Counter 标签：

- result：verified/unverified/rejected；
- identity_mode；
- identity_assurance；
- reason_code；
- caller_id：仅强身份且已可信识别时；
- credential_purpose：弱身份可用；
- usage_business_domain；
- usage_team。

不得加：

- principal_id/name；
- root_run；
- nonce；
- request_id；
- trace_id。

另外允许增加弱身份安全姿态低基数指标，例如 `company.ai.credential.risk`，标签只允许 purpose/risk_level，不允许 person。

---

## 9.20 第四批测试门禁

必须覆盖：

1. 无 traceparent：NewAPI 产生新 Trace。
2. 合法 traceparent：NewAPI 与上游保持同一 TraceId，Server Span 是新 SpanId。
3. Server Span 上能看到可信 App/Domain/Team 属性。
4. 一个逻辑模型请求只产生一个 GenAI logical Span。
5. NewAPI 内部发生两次 Channel Retry：一个 GenAI logical Span，下面有多个 HTTP attempt Span。
6. Provider 出站请求有正确 child traceparent。
7. Channel wildcard passthrough 不会原样复制入站 traceparent/tracestate。
8. OTel disabled 时不改变模型 HTTP 行为。
9. Exporter 暂时不可达时模型调用仍然可完成。
10. OpenAI Token 总量没有 cache 双计数。
11. Anthropic input total 包含 cache creation/read。
12. Gemini promptTokenCount 不再叠加 cachedContentTokenCount。
13. Streaming TTFC 与 NewAPI FirstResponseTime 一致。
14. Span 结束时有最终 Token/Quota。
15. Error Span 包含 error.type。
16. 不存在 Prompt/Response 全文属性。
17. 不存在 `gen_ai.business_*` 私有字段。
18. 高基数字段没有进入常规 Metric 标签。
19. CREDENTIAL_ONLY Span 明确 `credential_verified=true`、`client_verified=false`。
20. WorkBuddy 的 Principal/Purpose 可出现在 Span，但 Principal 不出现在常规 Metric Label。
21. 强身份平台 Span `client_verified=true`，弱身份永远不因 User-Agent 等指纹被提升可信度。

---

# 10. 第五批：异步任务归因、Trace Link 与单用户场景隔离

## 10.1 为什么第五批是必需项

视频、Suno、Midjourney 等任务提交后可能数分钟才完成。

请求结束后 Gin Context 消失，但后续仍可能查询、完成、失败、退款、差额结算。

企业部署所有 API Key 基本属于同一个 NewAPI User，因此只按 `user_id` 查询任务不能隔离不同凭证责任人、不同登记用途、不同 Caller/App。

第五批必须同时解决归因持久化与访问边界。

---

## 10.2 TaskPrivateData 扩展

新增两个可选快照。

### AttributionSnapshot

保存提交时 Trusted Attribution：

通用：

- profile_id；
- token_id；
- identity_mode；
- target_type；
- assurance；
- credential_verified；
- client_verified。

弱身份：

- principal_id/code/name；
- credential_purpose_id/code/name；
- usage_business_domain_*；
- usage_team_*。

强身份/应用：

- caller_id；
- root_app_id/name；
- application_business_domain_*；
- owner_team_*；
- root_run/current execution；
- identity_source/verified。

不保存签名秘密。

### TraceContextSnapshot

保存：

- traceparent；
- tracestate。

---

## 10.3 快照写入时机

任务创建并准备持久化时，从当前 RelayInfo/Trusted Context 深拷贝。

不得后台重新根据当前 Token 推导原任务归属。

---

## 10.4 异步计费日志

`taskBillingOther(task)` 合并持久化 AttributionSnapshot，使 Refund/Recalculate/Token 重算/最终结算继续归原凭证主体或原业务应用。

后续轮询者不能覆盖。

---

## 10.5 异步 OpenTelemetry

提交阶段正常结束 Trace/Span。

后台阶段新建后台 Span，并用 Span Link 指向提交阶段。

重新附加持久化 `company.ai.*`。

---

## 10.6 单 NewAPI User 下的任务访问边界

必须按身份类型分别处理。

### 弱身份 STATIC/PRINCIPAL

新任务 Snapshot 包含 Principal + Purpose。

当前查询凭证必须满足：

- Profile enabled；
- principal_id 等于任务 principal_id；
- credential_purpose_id 等于任务 purpose_id。

这样允许同一个人未来因 Token 更换而使用新的“同人 + 同用途”凭证继续访问原任务；但不允许：

- 张三 WorkBuddy Key 访问张三 IDE Key 创建的任务；
- 李四 WorkBuddy Key 访问张三 WorkBuddy 任务。

如果公司希望最严格“只能原 Key”，可以部署策略收紧为 token_id 一致，但 V1.1 默认治理语义按 Principal + Purpose。

### STATIC/APPLICATION

当前 Profile 必须仍固定同一个 App。

### DYNAMIC/PLATFORM

当前 caller_id 必须等于任务 caller_id，且当前 Profile 对 Root App 仍有 Binding。

### HYBRID/APPLICATION

Caller 与固定 App 均必须一致。

### Billable remix/continuation

新消费本身必须重新经过 AIIdentityAuth。

第一阶段禁止跨 Root App 的强身份 remix。

---

## 10.7 Legacy Task 策略

无 AttributionSnapshot 的历史任务：

### AUDIT
沿用 NewAPI 原有 user_id 行为，但打 `legacy_task_unattributed` 审计标记。

### ENFORCE
不得简单全局封死历史任务。必须提供可配置 Legacy 窗口：

- `AI_ATTRIBUTION_LEGACY_TASK_ACCESS=allow|deny`
- 默认 `allow` 仅用于迁移期
- 最终生产目标 `deny`

---

## 10.8 第五批测试门禁

1. WorkBuddy 弱身份任务保存 Principal/Purpose/Usage Team。
2. NewAPI 重启后归因仍存在。
3. 张三新 WorkBuddy Key（同 Principal + Purpose）可按策略访问旧 WorkBuddy 任务。
4. 张三 IDE Key 不可访问张三 WorkBuddy 任务。
5. 李四 WorkBuddy Key 不可访问张三任务。
6. DYNAMIC caller 相同 + App Binding 合法可访问。
7. caller 不同即使同一 NewAPI user_id 也拒绝。
8. HYBRID App 不同拒绝。
9. Refund/Recalculate 日志仍使用提交时归因。
10. 后台 Span Link 正确。
11. Legacy allow/deny 行为正确。

---

# 11. 第六批：管理页面、业务统计投影与 ENFORCE 上线

## 11.1 第六批目标

将第一至第五批能力做成可运营的 NewAPI 企业治理界面，不要求管理员直接改数据库。

必须支持：

- Business Domain；
- Application Owner Team；
- Usage Team；
- Principal；
- Credential Purpose；
- AI Application；
- API Key Identity Profile；
- Signing Key；
- Identity Audit；
- 弱身份凭证风险姿态；
- 按领域/组/人/用途/App/Caller 的模型消费统计。

---

## 11.2 前端位置固定

新增：

- `web/src/features/ai-governance/*`
- `web/src/routes/_authenticated/ai-governance/*`

侧边栏在 Admin 下增加“企业 AI 治理”。

不要把企业治理塞进 NewAPI 原 Users 页面，也不要把 Business Domain 塞进 Group 页面。

---

## 11.3 企业治理页面分组

建议一级“企业 AI 治理”，内部至少：

1. 业务领域；
2. 使用团队；
3. 使用主体；
4. 凭证用途；
5. 应用建设团队；
6. AI 应用；
7. API Key 身份；
8. 身份审计；
9. 企业用量。

可以在一个页面内使用 Tabs，但领域对象和语义不能合并。

---

## 11.4 使用主体页面

列表必须支持：

```text
领域
→ 使用团队
→ 人
```

例如：

```text
财务
└─ 财务数字化组
   ├─ 张三
   └─ 李四
```

人员详情必须展示其所有企业治理 Key：

```text
张三
├─ WorkBuddy → Token 101
├─ IDE Assistant → Token 102
└─ Personal Script → Token 103
```

每条 Key 显示：

- Token Name；
- Purpose；
- Identity Mode；
- Assurance；
- enabled；
- NewAPI Token Status；
- IP 限制；
- 模型限制；
- 额度限制；
- 过期时间；
- Risk Level；
- 最近调用时间；
- 最近 Token/Quota。

必须明确提示：

> “CREDENTIAL_ONLY 仅验证凭证，不验证实际客户端进程。”

---

## 11.5 AI 应用页面

展示：

- app_code/name；
- Application Business Domain；
- Owner Team；
- enabled；
- 绑定的强身份 Caller/Profile。

不得把 Usage Team 当成 Owner Team。

---

## 11.6 API Key 身份页面

选择一个 NewAPI Token 后配置：

- identity_mode；
- attribution_target_type；
- assurance；
- environment。

### STATIC/PRINCIPAL

配置：

- Principal；
- Credential Purpose。

UI 不出现 Caller 输入。

### STATIC/APPLICATION

配置固定 App，显示“客户端不可验证”。

### DYNAMIC/PLATFORM

配置：

- Caller；
- Allowed Apps；
- Signing Key。

### HYBRID/APPLICATION

配置：

- Caller；
- 固定 App；
- Signing Key。

UI 必须根据 mode/target/assurance 固定组合自动锁定不可编辑字段，不能让管理员自由组合出非法状态。

---

## 11.7 Weak Credential Risk UI

对 CREDENTIAL_ONLY 显示安全检查卡：

- IP 白名单：已配置/未配置；
- 模型白名单：已配置/未配置；
- 有限额度：是/否；
- 有效期：已配置/未配置；
- Profile 级请求限流：窗口/阈值/状态；
- 建议轮换日期/是否逾期；
- 最近限流/异常事件；
- Risk Level；
- client_verified=false。

第一阶段不新建安全策略，只读取 NewAPI Token。

---

## 11.8 Signing Secret UI 安全

生成/轮换后：

- Secret 只显示一次；
- 提供复制按钮；
- 用户关闭对话框后前端状态立即清空；
- 不写 LocalStorage/SessionStorage；
- 不进 URL；
- 不进全局持久 Store；
- 不打印 Console。

Key 列表只显示元数据。

---

# 12. Usage Projection 设计

## 12.1 为什么需要独立投影

`Log.Other` 是企业扩展事实来源，但长期看板不能依赖每次实时 JSON 解析。

第六批新增可重建投影：`ai_usage_hourly`。

Consume/Error/Task Billing Log 仍为事实来源，Projection 只是查询优化。

---

## 12.2 `ai_usage_hourly` 字段

使用主数据库。

建议字段：

| 字段 | 说明 |
|---|---|
| `id` | 自增主键 |
| `bucket_time` | 整点 Unix 秒 |
| `profile_id` | Identity Profile，未归因为 0 |
| `principal_id` | 弱身份个人主体，无则 0 |
| `credential_purpose_id` | 弱身份用途，无则 0 |
| `usage_business_domain_id` | 人员所属领域，无则 0 |
| `usage_team_id` | 使用团队，无则 0 |
| `caller_key` | 强身份 Caller 的稳定短字符串；弱身份为空 |
| `app_id` | 强身份/固定应用归因，无则 0 |
| `application_business_domain_id` | 应用领域，无则 0 |
| `owner_team_id` | 应用建设团队，无则 0 |
| `identity_assurance` | CREDENTIAL_ONLY / SIGNED_CONTEXT / HYBRID_VERIFIED_CONTEXT / UNVERIFIED |
| `model_name` | 模型 |
| `request_count` | 请求数 |
| `success_count` | 成功数 |
| `error_count` | 错误数 |
| `input_tokens` | 输入 Token |
| `output_tokens` | 输出 Token |
| `total_tokens` | 总 Token |
| `quota_net` | 净 Quota，bigint |
| `duration_ms_total` | 总耗时 |
| `created_at` | 创建 |
| `updated_at` | 更新 |

唯一聚合维度必须覆盖：

`bucket_time + profile_id + principal_id + purpose_id + usage_domain_id + usage_team_id + caller_key + app_id + app_domain_id + owner_team_id + assurance + model_name`

若数据库组合唯一索引长度不适合，可使用稳定 `dimension_hash` 作为唯一键，但 Hash 原文与版本必须由方案固定，不能让实现 AI自行选择字段遗漏。

---

## 12.3 Projection 写入语义

### 弱身份 CREDENTIAL_ONLY

只要 STATIC/PRINCIPAL Profile 配置合法并完成 TokenAuth + 静态主数据验证，可以计入正式“凭证责任统计”，因为 Principal/Purpose 是 NewAPI 自己的可信登记。

但是统计名称必须是：

- Credential Owner；
- Usage Domain；
- Usage Team；
- Declared Purpose。

不得叫“Verified Client Usage”。

### 强身份

只有 `client_verified=true` 的 DYNAMIC/HYBRID 才进入正式 Caller/Root App 统计。

### 未验证流量

强身份 AUDIT 失败请求进入“未验证/未归因”桶，不得塞入客户端声明 App。

---

## 12.4 统计视角

必须支持：

### 弱身份

- Usage Domain → Usage Team → Principal → Credential Purpose；
- Principal → Purpose；
- Purpose → Usage Domain/Team；
- Risk Level 辅助筛选。

### 强身份

- Caller；
- Application Business Domain → Application；
- Owner Team → Application；
- Caller → App。

### 共用

- Model；
- 时间；
- Token；
- Quota；
- Requests；
- Errors；
- Avg Latency；
- Identity Assurance。

---

## 12.5 弱身份确定性异常检测

第六批基于 `ai_usage_hourly` 提供简单、可解释的异常提示，不构建机器学习模型。

系统设置至少支持：

- `AI_CREDENTIAL_HOURLY_REQUEST_ALERT`：0 表示关闭；
- `AI_CREDENTIAL_HOURLY_TOKEN_ALERT`：0 表示关闭；
- `AI_CREDENTIAL_HOURLY_QUOTA_ALERT`：0 表示关闭；
- `AI_CREDENTIAL_BASELINE_MULTIPLIER`：默认 5。

若启用 baseline：

- 使用最近 7 个有效业务日相同小时的中位数；
- 当前小时指标 >= baseline × multiplier 且同时超过绝对最小阈值时生成风险提示；
- 新凭证历史不足时只使用绝对阈值；
- 异常只记录和展示，不自动禁用 Token；
- 必须定位到 `profile_id → principal → purpose`。

不得把统计异常提升为“客户端身份验证”。

---

## 12.6 Projection 重建

Root-only，按时间范围重建。

核心逻辑在 Go 中解析 Log `Other.ai_attribution`，不得依赖数据库特定 JSON SQL。

幂等、失败不留半清空数据、成功后原子替换目标范围。

---

## 12.7 企业统计 API

至少支持：

- 按 Usage Business Domain；
- Usage Team；
- Principal；
- Credential Purpose；
- Caller；
- Application；
- Application Business Domain；
- Owner Team；
- Model；
- Identity Assurance；
- 时间。

第一阶段不承诺人民币/美元金额，除非公司另行冻结 Quota 到货币口径。

---

# 13. ENFORCE 上线门禁

不得代码完成当天直接开启 ENFORCE。

必须先运行 AUDIT。

满足以下条件才允许切 ENFORCE：

1. 所有生产模型 API Key 都已配置 Profile，或处置计划已完成；最终例外清零。
2. 所有个人 WorkBuddy/IDE 等弱身份 Key 都有明确 Principal + Purpose。
3. 同一个人不同用途使用不同 Token，不存在“一个 Key 同时批准 WorkBuddy/IDE/脚本”。
4. CREDENTIAL_ONLY 管理页面明确显示 `client_verified=false`，不存在将登记用途当作客户端验证的逻辑。
5. 所有 DYNAMIC/HYBRID Profile 有 ACTIVE Signing Key。
6. Redis 稳定可用并完成故障演练。
7. 所有 AI Application 有 Application Domain + Owner Team。
8. 所有 Principal 有 Usage Domain + Usage Team。
9. 最近连续 3 个业务日不存在未知 UNVERIFIED 强身份调用。
10. 最近连续 3 个业务日不存在未知 APP_NOT_BOUND。
11. STATIC/PRINCIPAL、STATIC/APPLICATION、DYNAMIC、HYBRID 各至少一个真实 POC（未实际使用的模式可以书面豁免但测试必须存在）。
12. 至少覆盖公司真实使用的主要协议渠道烟测。
13. SSE/Streaming 通过。
14. 异步任务如果在用，第 5 批必须完成。
15. OTel Exporter 故障不导致模型整体不可用。
16. 上游动态平台拿到正式签名协议。
17. 回滚开关完成演练。
18. 对弱身份 Key 已形成公司安全基线：原则上启用公司 VPN/IP 白名单、模型白名单、有限额度、有效期、Profile 级独立限流；无法满足某项必须有责任人和书面豁免。
19. 原 NewAPI user_id 级 ModelRequestRateLimit 已确认不承担个人 Key 独立限流；所有需要频率保护的 CREDENTIAL_ONLY Profile 已启用企业 per-profile limiter。
20. 超过建议轮换周期的个人 Key 已处理或有明确轮换计划。
21. 弱身份确定性异常提示已能定位到 Profile/Principal/Purpose。

切换方式：配置改为 ENFORCE 后滚动部署。

---

# 14. 回滚策略

出现应用层大面积身份接入问题：

`enforce → audit`

不得通过删除 Identity Table、绕过 TokenAuth、关闭 NewAPI 原认证来回滚。

出现企业身份代码严重故障且需要完全回到官方行为：

`audit → disabled`

数据表和历史日志保留。

OTel 故障：

`OTEL_ENABLED=false`

不影响 AI Attribution。

Signing Secret 泄漏：

1. 立即 revoke Signing Key；
2. 若怀疑 API Key 也泄漏，立即禁用原 NewAPI Token；
3. 创建新 Token/新 Signing Key；
4. 更新调用方；
5. 验证后恢复 Profile。

个人 CREDENTIAL_ONLY API Key 泄漏：

1. 立即禁用原 NewAPI Token；
2. Profile 同步 disabled；
3. 新建/轮换个人专用 Token；
4. 保持 Principal + Purpose 不变；
5. 重新检查 IP/模型/额度/有效期安全姿态；
6. 审计旧 Token 在泄漏时间窗内的调用。

---

# 15. 各批次允许修改与禁止修改文件范围

## 15.1 允许新增

- `constant/ai_attribution.go`
- `types/ai_attribution.go`
- `common/ai_attribution_context.go`
- `model/ai_governance*.go`
- `service/ai_identity*.go`
- `service/ai_credential_risk.go`
- `middleware/ai_identity.go`
- `middleware/ai_credential_rate_limit.go`
- `controller/ai_governance*.go`
- `pkg/telemetry/*`
- 对应 `_test.go`
- `web/src/features/ai-governance/*`
- `web/src/routes/_authenticated/ai-governance/*`

## 15.2 允许极薄修改

- `model/main.go`：注册主库迁移；
- `router/api-router.go`：Root 管理 API；
- `router/relay-router.go`：TokenAuth 后插 AIIdentityAuth；
- `router/video-router.go`：身份中间件/任务访问保护；
- 其他 Task Router：仅插身份/访问保护；
- `constant/context_key.go`：一个 Trusted Attribution Context Key；
- `relay/common/relay_info.go`：Attribution/Telemetry 单对象；
- `model/log.go`：集中合并归因/Trace；
- `model/task.go`：Task PrivateData 快照；
- `service/task_billing.go`：异步日志读快照；
- `relay/relay_task.go` / 对应 Controller：任务创建与访问保护；
- `relay/channel/api_request.go`：Request Context/Trace Header；
- `service/http_client.go`：OTel RoundTripper；
- `main.go`：OTel init/shutdown；
- `middleware/audit.go`：治理 action；
- `web/src/hooks/use-sidebar-data.ts`：企业治理菜单。

`middleware/model-rate-limit.go` 原则上不修改；企业凭证级限流通过独立中间件实现。

## 15.3 第一阶段原则上禁止修改

- NewAPI Token 核心表字段；
- NewAPI User 核心表字段；
- NewAPI Group 语义；
- 模型价格算法；
- Quota 公式；
- Subscription/Wallet 核心结算；
- Provider Adapter 内部业务转换；
- Provider Usage 算法；
- ClickHouse `logs` DDL；
- FastGPT/Dify/WorkBuddy 源码。

额外禁止：

- 通过 User-Agent 识别 WorkBuddy 并标记 client_verified；
- 新建一套重复的 IP/模型/Quota 安全策略表；
- 把 Principal/Usage Team 塞到 NewAPI User/Group；
- 把登记用途字段命名成 verified_client/client_id。

若实现 AI 认为必须修改禁区，必须停止并给出源码阻塞证据。

---

# 16. 第一阶段最终端到端验收场景

必须同时覆盖弱身份和强身份。

## 16.1 基础主数据

Business Domain：

- `human_resources` / 人力；
- `finance` / 财务。

Usage Team：

- `finance_digital` / 财务数字化组。

Principal：

- `zhangsan` / 张三 / 财务 / 财务数字化组。

Credential Purpose：

- `workbuddy` / WorkBuddy；
- `ide_assistant` / IDE AI 助手。

Owner Team：

- `ai_application` / AI应用组。

Application：

- `hr_assistant` / 人力助手 / 人力 / AI应用组；
- `finance_assistant` / 财务助手 / 财务 / AI应用组。

## 16.2 弱身份场景 A：张三 WorkBuddy

Profile：

- Token 101；
- STATIC；
- PRINCIPAL；
- CREDENTIAL_ONLY；
- Principal=张三；
- Purpose=WorkBuddy。

调用只带 API Key。

验收：

- credential_verified=true；
- client_verified=false；
- principal=张三；
- usage domain=财务；
- usage team=财务数字化组；
- purpose=WorkBuddy；
- caller/root_app/root_run 为空；
- Consume Log/OTel 属性正确；
- 统计进入 财务 → 财务数字化组 → 张三 → WorkBuddy；
- Profile 级频率桶与其他 Key 独立；
- 管理页面显示 IP/模型/Quota/Expiry/RateLimit/Rotation 风险姿态。

## 16.3 弱身份场景 B：同一个 Key 被 curl 使用

从允许网络使用同一 Token 101 发送 curl。

预期：

- 请求在 Token/网络/模型等安全策略允许时仍可能成功；
- 系统必须仍记录 Purpose=WorkBuddy；
- 但 `client_verified=false`；
- 不得产生“已验证 WorkBuddy”结论。

这个测试用于证明系统没有夸大身份可信程度。

## 16.4 弱身份场景 C：张三第二用途

创建 Token 102：

- Principal=张三；
- Purpose=IDE Assistant。

必须能与 Token 101 并存。

不得允许 Token 101 同时登记 WorkBuddy + IDE 两用途。

## 16.5 弱身份场景 D：独立限流与轮换

连续调用 Token 101 达到 Profile 阈值：

- 下一个请求 429；
- Token 102 不受影响；
- 李四 Token 不受影响；
- Provider 不收到被限流请求；
- 管理页面能看到限流事件。

模拟 Token 101 超过轮换周期：

- 不自动假装轮换；
- Risk 显示 rotation overdue；
- 责任人明确为张三；
- 管理员换发新 Key 后旧 Token 禁用。

---

## 16.6 强身份场景 D：合法人力请求

DYNAMIC/PLATFORM：

- Caller=`workflow-platform-prod`；
- 绑定 hr/finance app；
- HMAC 合法。

结果：

- client_verified=true；
- Root App=hr_assistant；
- Application Domain=人力；
- Owner Team=AI应用组；
- Root Run/Execution 正确。

## 16.7 强身份场景 E：合法财务请求

同理。

## 16.8 强身份场景 F：伪造未绑定 App

ENFORCE 403，Provider 不收到请求。

## 16.9 强身份场景 G：只有 API Key 无签名

AUDIT 放行但不采用 root_app；
ENFORCE 拒绝。

## 16.10 强身份场景 H：API Key 泄漏

攻击者没有 Signing Secret，无法生成 VERIFIED Dynamic 请求。

## 16.11 重放、嵌套、渠道重试、流式

保持 V1.0 验收：

- Nonce Replay 拒绝；
- nested execution 保持 Root App/Root Run；
- Channel Retry 一个逻辑 GenAI Span；
- Streaming 结束后结束 Span，Usage 正确；
- 企业 Header 不泄漏。

## 16.12 异步任务

弱身份：

- 张三 WorkBuddy 创建任务；
- 张三 IDE Key 不能访问；
- 李四 WorkBuddy 不能访问；
- 同 Principal + Purpose 的替换 Token 可按第五批策略访问。

强身份：

- 不同 Caller 即使同 user_id 不能访问；
- 原 Root App 不丢失。

---

# 17. 第一阶段 Definition of Done

只有全部满足以下条件才算完成：

1. NewAPI 原 Token/User/Group/Quota 核心模型没有被企业归因污染。
2. Business Domain、Owner Team、Usage Team、Principal、Credential Purpose、Application 有独立可信主数据。
3. Application Owner Team 与 Principal Usage Team 完全分离。
4. 一个 Principal 可以拥有多个不同用途 Key。
5. 一个个人弱身份 Key 只能有一个 Principal 和一个 Purpose。
6. WorkBuddy/IDE 等弱身份统一 `CREDENTIAL_ONLY`。
7. CREDENTIAL_ONLY 永远 `client_verified=false`。
8. 登记 Purpose 不被当作客户端认证结果。
9. 弱身份统计可以按 Usage Domain → Usage Team → Principal → Purpose 下钻。
10. DYNAMIC/HYBRID 使用独立 HMAC Signing Secret。
11. 强身份平台可以形成 verified Caller/Root App/Root Run。
12. Signing Secret 数据库不明文。
13. Timestamp/Nonce/Replay 完成。
14. AUDIT/ENFORCE 有完整测试。
15. AUDIT 失败强身份请求不污染 App 统计。
16. 企业 Header 不到 Provider。
17. 所有真实模型消费入口覆盖，不只 chat/completions。
18. Consume/Error Log 保存弱/强身份可信快照。
19. 原 Token/Quota/计费结果无回归。
20. NewAPI 原 user_id/Group `ModelRequestRateLimit` 未被企业逻辑改写。
21. Profile 级凭证限流按 `profile_id` 隔离，张三 WorkBuddy、张三 IDE、李四 WorkBuddy 互不共享计数。
22. CREDENTIAL_ONLY Risk 页面覆盖 IP/模型/额度/有效期/独立限流/轮换状态。
23. 弱身份确定性异常提示可以定位到 Principal/Purpose。
24. W3C Trace 对接。
25. Server Span、GenAI logical Span、Provider HTTP Span 正确。
26. OpenAI/Anthropic/Gemini Token 无 cache 双计数。
27. 企业属性使用 `company.ai.*`。
28. Principal 不作为默认常规 Metric Label。
29. 不采集 Prompt/Response 全文。
30. 异步任务跨请求保存 Attribution。
31. 同 NewAPI User 场景下任务不存在跨 Principal/Purpose/Caller/App 横向访问。
32. 管理 UI 可维护人、领域、使用组、用途、应用、Caller、Key。
33. Signing Secret 只展示一次。
34. Usage Projection 可按弱身份和强身份两套维度重建。
35. 审计期达到 ENFORCE 门禁。
36. 关闭 Attribution/OTel 可回到接近官方行为。
37. 企业扩展集中，未逐 Provider Patch。
38. 每批测试、迁移、集成结果留存。

---

# 18. 交给实现 AI 的最终硬约束

必须按批次 1 → 2 → 3 → 4 → 5 → 6 推进，每次只实施当前批次。

每批输出：

1. 实际修改文件；
2. 文件目的；
3. 与本文档逐项映射；
4. 数据库迁移；
5. 单元测试；
6. 集成测试；
7. 未完成项；
8. 是否触碰禁止区域；
9. 是否发现源码冲突；
10. Git commit SHA。

禁止：

- 顺手重构无关 NewAPI；
- 删除任一正式身份模式；
- 用 API Key 做 HMAC Secret；
- 把 Domain/Team/Principal/Purpose 塞进 NewAPI User/Group；
- 把一个 WorkBuddy Key 同时批准给 IDE/Script；
- 因为 Header/User-Agent 看起来像 WorkBuddy 就设 client_verified=true；
- 为弱身份再造一套 IP/模型/Quota 表；
- 修改原 `ModelRequestRateLimit` 为 token_id 语义；必须使用独立 Profile 级限流；
- 大面积修改 logs/ClickHouse DDL；
- 自造私有 Trace ID；
- 逐 Provider Adapter 接 Attribution/OTel。

若真实源码与本文档冲突，停止该点并提交：

`文件路径 + Symbol + 调用链 + 冲突原因 + 可选最小调整`

不得自行改架构。

---

# 19. 实施前 Git 基线要求

企业仓库应保持：

```text
origin   = 企业自己的 NewAPI 仓库
upstream = QuantumNous/new-api 官方仓库
```

第一阶段固定从官方 `v1.0.0-rc.25` 建企业基线，不直接在不断变化的官方 `main` 上开发。

建议形成：

- 不可变基线标识：`enterprise-base-v1.0.0-rc.25`；
- 企业长期分支：`enterprise/main`；
- 第一阶段各批次独立 feature 分支。

每一批合入前必须记录与官方基线的差异，并持续保证“企业代码集中、官方核心接入点极薄”。

后续升级官方版本时使用独立 Upgrade Branch 合并上游并跑完整企业身份、计费、Trace、异步任务回归，不以“Git 无冲突”等同于“升级成功”。

---

# 20. 第一阶段最终冻结架构

```text
                           企业治理主数据
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                        │
  Business Domain          Application Side         Credential Side
        │                        │                        │
        │                 ┌──────┴──────┐          ┌──────┴────────┐
        │             Owner Team   AI Application  Usage Team   Principal
        │                        │                    │           │
        │                        │                    └─────┬─────┘
        │                        │                          │
        │                        │                Credential Purpose
        │                        │                          │
        └────────────────────────┴──────────────┬───────────┘
                                               │
                                         NewAPI Token
                                               │
                                        Identity Profile
                                               │
                      ┌────────────────────────┼────────────────────────┐
                      │                        │                        │
              STATIC/PRINCIPAL        STATIC/APPLICATION       DYNAMIC/HYBRID
              CREDENTIAL_ONLY         CREDENTIAL_ONLY          SIGNED CONTEXT
                      │                        │                        │
                Principal/Purpose          Fixed App             Caller + App
                      │                        │                        │
                      └──────────────┬─────────┴───────────┬────────────┘
                                     │                     │
                               TokenAuth              AIIdentityAuth
                                     │                     │
                                     └──────────────┬──────┘
                                                    ▼
                                         Trusted Attribution
                                                    │
                                       AICredentialRateLimit
                                                    │
                                                    │
                           ┌────────────────────────┼───────────────────────┐
                           │                        │                       │
                       RelayInfo               Consume Log          OpenTelemetry
                           │                        │                       │
                           └────────────────────────┼───────────────────────┘
                                                    │
                                             NewAPI 原 Relay
                                                    │
                                             NewAPI 原 Billing
                                                    │
                                                 Provider
                                                    │
                                                    ▼
                                   Token / Quota / Latency / Status
                                                    │
                                                    ▼
                                             Usage Projection
                                                    │
                         ┌──────────────────────────┴──────────────────────┐
                         │                                                 │
       Usage Domain → Usage Team → Person → Purpose       Caller → App → App Domain → Owner Team
```

第一阶段核心原则冻结为：

> **NewAPI 原 Token 负责证明“这把 API Key 是否可以进入网关”。对于只能配置 API Key 的 WorkBuddy、IDE、桌面客户端等，企业系统只把 Key 归属到责任人和批准用途，并明确 `CREDENTIAL_ONLY / client_verified=false`，通过一人一用途一 Key、公司网络/IP、模型限制、额度、有效期、审计等降低滥用风险；不得假装 NewAPI 能证明真实客户端。对于工作流平台、智能体平台等可签名调用方，使用独立 Signing Secret 验证 Caller、Root App、Root Run 和执行上下文，形成强身份。两类流量最后都与 NewAPI 原有 Token、Quota、模型、渠道、日志和 OpenTelemetry 事实关联。**

---

# 21. V1.1 相对 V1.0 的强制修订摘要

本节只用于帮助实施者确认没有误拿旧方案，不是增量实施顺序。

V1.1 强制新增：

1. `ai_usage_teams`
2. `ai_principals`
3. `ai_credential_purposes`
4. `attribution_target_type`
5. `identity_assurance`
6. `credential_verified`
7. `client_verified`
8. `usage_business_domain_*`
9. `usage_team_*`
10. `credential_purpose_*`
11. 弱身份 Risk Posture
12. Principal + Purpose 异步任务访问隔离
13. 人员 → 多用途 → 多 Key 管理视图
14. Profile 级每凭证独立请求限流
15. 弱身份轮换逾期与确定性异常提示

V1.1 强制修正：

- `caller_id=workbuddy` 不再表示已验证客户端；
- STATIC 不再统一强制绑定 Root App；
- STATIC/PRINCIPAL 不生成伪造 root_run；
- Business Domain 分成 usage 与 application 两种日志/统计语义；
- Owner Team 与 Usage Team 分离；
- Principal 不作为普通 Metrics 高基数标签；
- 一个人可以多 Key，但一个 Key 只能一个批准用途；
- WorkBuddy Key 被 curl/其他客户端复用时，若其他安全条件满足仍可能调用成功，这属于弱身份边界，系统必须准确记录而不是虚假“验证成功客户端”；
- NewAPI 原请求限流按 user_id/Group，不适合作为企业单用户模式下的个人 Key 独立限流，因此新增独立 profile_id 级 Guardrail，但不得改写原限流逻辑。
