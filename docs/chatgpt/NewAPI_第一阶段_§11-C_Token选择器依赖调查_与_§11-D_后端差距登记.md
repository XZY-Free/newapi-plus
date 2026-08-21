# 第一阶段 · §11-A 收口：§11-C Token 选择器依赖调查 与 §11-D 后端差距登记

> 状态：`§11-A.1 收口轮`产物之一（未开始 §11-B）。
> 目的：为后续 §11-B～E 前端批次明确“可直接复用”与“后端尚缺、需登记而非掩盖”的边界。
> 原则：本文件只做**调查**与**登记**，不产生任何后端代码改动；前端不得用 Mock 掩盖登记在案的差距。

---

## 一、§11-C：Identity Profile 的 Token 选择器依赖调查

### 1. 结论（决策）

**不需要第二套 Token 管理系统，也不需要第二张 Token 表。**
Identity Profile 的 `token_id` 字段指向的是 NewAPI **既有** Token（`api_tokens`），
Token 选择器直接复用现有 `features/keys/api.ts` 提供的“列表 + 搜索”能力，选择后仅提交
`token_id` 整数给 `/api/ai-governance/identity-profiles` 创建/更新请求。

### 2. 可复用的现有入口（已核对源码）

`web/src/features/keys/api.ts`：

| 函数 | 请求 | 说明 |
|---|---|---|
| `getApiKeys({ p, size })` | `GET /api/token/?p=&size=` | 分页列出全部 Token |
| `searchApiKeys({ keyword, token, p, size })` | `GET /api/token/search?...` | 按关键字/Token 搜索 |

- 二者均返回既有 `GetApiKeysResponse`，选择器只需把其中 `id` 字段作为 `token_id` 提交。
- Token 选择器属于**只读引用**，不新增/修改/删除任何 Token，因此不与现有 Token 管理产生写竞争。

### 3. 落库约束（写侧契约，与前端选择器无关）

- 创建 Profile：`CreateIdentityProfilePayload.token_id`（必填，数字）。
- 更新 Profile：`UpdateIdentityProfilePayload` **不含 `token_id`**（后端将其视为不可变字段），
  已在本轮 `types.ts` 强类型中固化为“结构上无法提交”，并由 `api.test.ts` 断言（
  `identity profile update never submits token_id`）。

### 4. 待 §11-B 实现时遵循

- 选择器 UI 用 `getApiKeys`/`searchApiKeys` 拉取候选，展示 Token 名称/ID，选择后仅把
  `token_id` 传入 Profile 表单；**不得**把整个 Token 对象序列化进 Profile 请求体。
- 若后端返回的 Token 已绑定到其他 Profile，需在 §11-B 前端做去重/提示；这是 UI 行为，非契约缺口。

---

## 二、§11-D：后端差距登记（Gap Register）

> 登记标准：仅登记**本轮已通过阅读后端真实代码核实**的、会阻碍后续 §11-D/E UI 的能力缺口。
> 状态列：`已核实（存在）` / `已核实（缺）`。凡 `缺` 项，**一律不改后端、不造 Mock**，留待专门批次/评审后再动。

### 2.1 身份审计查询能力（`GET /api/ai-governance/identity-audit-events`）

**后端现状（已核实，`controller/ai_governance.go:898` `GetIdentityAuditEvents`）：**

支持的唯一筛选键：

- `page` / `page_size`（`aiGovernancePagination`）
- `request_id`
- `token_id`
- `profile_id`
- `result`
- `reason_code`

| # | 期望能力（§11-D 审计看板） | 后端是否提供 | 差距说明 |
|---|---|---|---|
| D1 | 按时间范围筛选（start/end） | **缺** | 仅 `created_at desc` 排序，无时间区间过滤 |
| D2 | 按 `principal_id` 筛选 | **缺** | 无法只看某责任人相关事件 |
| D3 | 按 `credential_purpose_id` 筛选 | **缺** | 无法按批准用途收敛审计流 |
| D4 | 按 `caller_id` 筛选 | **缺** | 无法按强身份调用方收敛 |
| D5 | 按 `identity_assurance` 筛选 | **缺** | 无法只看降级/未验证态 |
| D6 | 按 App（root_app_id / app_id）筛选 | **缺** | 无法按应用收敛 |
| D7 | 按 `trace_id` 关联审计与 Trace | **缺** | 审计事件无 trace_id 查询维度（§9/§10 已为运行时埋 trace，审计查询未接） |
| D8 | 审计事件量级聚合（按结果/原因计数） | **缺** | 看板需 `GROUP BY result/reason_code` 计数，当前仅分页明细 |

**登记结论：** §11-D/E 的“审计看板”（趋势、结果分布、按责任人/用途/应用收敛）在当前后端接口下
无法用一次查询实现，需要后续新增审计查询能力（时间区间 + 更多维度筛选 + 聚合计数）。
本轮**不做任何后端改动**，前端 §11-A 亦**不**用 Mock 伪造这些聚合。

### 2.2 其他已核实能力（供 §11-B/E 前端参考，非缺口）

- **Identity Profile 聚合详情**（`GET/PUT /identity-profiles/:id`、`GET /identity-profiles`）：
  统一返回 `buildIdentityProfileDetail` 聚合 DTO（profile/principal/purpose/token/bindings/risk/rate_limits），
  列表与详情结构一致 —— 前端可直接复用，**无缺口**。
- **签名密钥**（`list/generate/rotate/revoke`）：元数据数组不含 secret；`generate/rotate` 一次性返回
  `{key, secret}`；`revoke` 返回 `{revoked: true}`。契约完整，**无缺口**。
- **App Binding**（`PUT /identity-profiles/:id/app-bindings`）：整体替换 `{app_ids:[...]}`，**无缺口**。
- **企业用量投影**（`GET /usage/stats`、`GET /usage/anomalies`、`POST /usage/rebuild`）：
  stats 返回裸数组（非分页），anomalies 用 `bucket_start/bucket_end`，rebuild 用 `start/end`。**无缺口**。

### 2.3 登记纪律

- 前端只实现后端已提供的契约；任何登记为 `缺` 的能力，**不在本轮**以 Mock/占位数据填充。
- 后续若开启补齐，须走独立评审，且遵守 AGENTS.md 的数据库三库兼容与安全不变量约束。

---

## 三、后续批次进入门禁

- 本轮（§11-A.1）仅交付：契约冻结 + 首页 Overview + i18n + 契约测试 + 本登记文件。
- **§11-B 尚未允许开始**，须待本收口轮评审通过。
