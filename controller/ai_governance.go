package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// 第一批企业身份归因 Root 管理 API（/api/ai-governance）。
// 整个路由组使用 RootAuth()。所有写操作的管理审计由 authHelper 的兜底逻辑
// 依据 middleware/audit.go 的语义化 action 自动记录。

// aiGovernanceMaxPageSize 是治理列表分页 page_size 的统一上限，防止一次性拉取超大结果集。
const aiGovernanceMaxPageSize = 200

func aiGovernancePagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > aiGovernanceMaxPageSize {
		pageSize = aiGovernanceMaxPageSize
	}
	return
}

func aiGovernanceEnabledParam(c *gin.Context) (*bool, error) {
	v := c.Query("enabled")
	if v == "" {
		return nil, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil, errors.New("enabled 参数非法")
	}
	return &b, nil
}

func aiGovernanceIntParam(c *gin.Context, name string) (int, error) {
	v := c.Query(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, errors.New(name + " 参数非法")
	}
	return n, nil
}

func aiGovernanceID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return 0, errors.New("id 非法")
	}
	return id, nil
}

func listResult(items interface{}, total int64, page, pageSize int) gin.H {
	return gin.H{"items": items, "total": total, "page": page, "page_size": pageSize}
}

// ---------------------------------------------------------------------------
// Business Domain
// ---------------------------------------------------------------------------

type businessDomainCreateRequest struct {
	DomainCode string `json:"domain_code"`
	DomainName string `json:"domain_name"`
}

type businessDomainUpdateRequest struct {
	DomainName string `json:"domain_name"`
	Enabled    *bool  `json:"enabled"`
}

func GetBusinessDomains(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	keyword := c.Query("keyword")
	enabled, err := aiGovernanceEnabledParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	items, total, err := service.ListBusinessDomains(page, pageSize, keyword, enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, listResult(items, total, page, pageSize))
}

func CreateBusinessDomain(c *gin.Context) {
	var req businessDomainCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	domain, err := service.CreateBusinessDomain(req.DomainCode, req.DomainName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, domain)
}

func UpdateBusinessDomain(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var req businessDomainUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	domain, err := service.UpdateBusinessDomain(id, req.DomainName, req.Enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, domain)
}

// ---------------------------------------------------------------------------
// Owner Team
// ---------------------------------------------------------------------------

type ownerTeamCreateRequest struct {
	TeamCode string `json:"team_code"`
	TeamName string `json:"team_name"`
}

type ownerTeamUpdateRequest struct {
	TeamName string `json:"team_name"`
	Enabled  *bool  `json:"enabled"`
}

func GetOwnerTeams(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	keyword := c.Query("keyword")
	enabled, err := aiGovernanceEnabledParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	items, total, err := service.ListOwnerTeams(page, pageSize, keyword, enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, listResult(items, total, page, pageSize))
}

func CreateOwnerTeam(c *gin.Context) {
	var req ownerTeamCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	team, err := service.CreateOwnerTeam(req.TeamCode, req.TeamName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, team)
}

func UpdateOwnerTeam(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var req ownerTeamUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	team, err := service.UpdateOwnerTeam(id, req.TeamName, req.Enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, team)
}

// ---------------------------------------------------------------------------
// Usage Team
// ---------------------------------------------------------------------------

type usageTeamCreateRequest struct {
	TeamCode string `json:"team_code"`
	TeamName string `json:"team_name"`
}

type usageTeamUpdateRequest struct {
	TeamName string `json:"team_name"`
	Enabled  *bool  `json:"enabled"`
}

func GetUsageTeams(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	keyword := c.Query("keyword")
	enabled, err := aiGovernanceEnabledParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	items, total, err := service.ListUsageTeams(page, pageSize, keyword, enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, listResult(items, total, page, pageSize))
}

func CreateUsageTeam(c *gin.Context) {
	var req usageTeamCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	team, err := service.CreateUsageTeam(req.TeamCode, req.TeamName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, team)
}

func UpdateUsageTeam(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var req usageTeamUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	team, err := service.UpdateUsageTeam(id, req.TeamName, req.Enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, team)
}

// ---------------------------------------------------------------------------
// Principal
// ---------------------------------------------------------------------------

type principalCreateRequest struct {
	PrincipalCode    string `json:"principal_code"`
	PrincipalName    string `json:"principal_name"`
	BusinessDomainID int    `json:"business_domain_id"`
	UsageTeamID      int    `json:"usage_team_id"`
}

type principalUpdateRequest struct {
	PrincipalName    string `json:"principal_name"`
	BusinessDomainID int    `json:"business_domain_id"`
	UsageTeamID      int    `json:"usage_team_id"`
	Enabled          *bool  `json:"enabled"`
}

func GetPrincipals(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	keyword := c.Query("keyword")
	enabled, err := aiGovernanceEnabledParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	businessDomainID, err := aiGovernanceIntParam(c, "business_domain_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	usageTeamID, err := aiGovernanceIntParam(c, "usage_team_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	items, total, err := service.ListPrincipals(page, pageSize, keyword, enabled, businessDomainID, usageTeamID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, listResult(items, total, page, pageSize))
}

func GetPrincipal(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	p, err := service.GetPrincipal(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, p)
}

func CreatePrincipal(c *gin.Context) {
	var req principalCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	p, err := service.CreatePrincipal(req.PrincipalCode, req.PrincipalName, req.BusinessDomainID, req.UsageTeamID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, p)
}

func UpdatePrincipal(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var req principalUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	p, err := service.UpdatePrincipal(id, req.PrincipalName, req.BusinessDomainID, req.UsageTeamID, req.Enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, p)
}

// ---------------------------------------------------------------------------
// Credential Purpose
// ---------------------------------------------------------------------------

type credentialPurposeCreateRequest struct {
	PurposeCode string `json:"purpose_code"`
	PurposeName string `json:"purpose_name"`
	PurposeType string `json:"purpose_type"`
}

type credentialPurposeUpdateRequest struct {
	PurposeName string `json:"purpose_name"`
	PurposeType string `json:"purpose_type"`
	Enabled     *bool  `json:"enabled"`
}

func GetCredentialPurposes(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	keyword := c.Query("keyword")
	enabled, err := aiGovernanceEnabledParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	purposeType := c.Query("purpose_type")
	items, total, err := service.ListCredentialPurposes(page, pageSize, keyword, enabled, purposeType)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, listResult(items, total, page, pageSize))
}

func CreateCredentialPurpose(c *gin.Context) {
	var req credentialPurposeCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	p, err := service.CreateCredentialPurpose(req.PurposeCode, req.PurposeName, req.PurposeType)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, p)
}

func UpdateCredentialPurpose(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var req credentialPurposeUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	p, err := service.UpdateCredentialPurpose(id, req.PurposeName, req.PurposeType, req.Enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, p)
}

// ---------------------------------------------------------------------------
// AI Application
// ---------------------------------------------------------------------------

type applicationCreateRequest struct {
	AppCode          string `json:"app_code"`
	AppName          string `json:"app_name"`
	BusinessDomainID int    `json:"business_domain_id"`
	OwnerTeamID      int    `json:"owner_team_id"`
}

type applicationUpdateRequest struct {
	AppName          string `json:"app_name"`
	BusinessDomainID int    `json:"business_domain_id"`
	OwnerTeamID      int    `json:"owner_team_id"`
	Enabled          *bool  `json:"enabled"`
}

func GetApplications(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	keyword := c.Query("keyword")
	enabled, err := aiGovernanceEnabledParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	businessDomainID, err := aiGovernanceIntParam(c, "business_domain_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	ownerTeamID, err := aiGovernanceIntParam(c, "owner_team_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	items, total, err := service.ListApplications(page, pageSize, keyword, enabled, businessDomainID, ownerTeamID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, listResult(items, total, page, pageSize))
}

func GetApplication(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	app, err := service.GetApplication(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, app)
}

func CreateApplication(c *gin.Context) {
	var req applicationCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	app, err := service.CreateApplication(req.AppCode, req.AppName, req.BusinessDomainID, req.OwnerTeamID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, app)
}

func UpdateApplication(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var req applicationUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	app, err := service.UpdateApplication(id, req.AppName, req.BusinessDomainID, req.OwnerTeamID, req.Enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, app)
}

// ---------------------------------------------------------------------------
// Identity Profile
// ---------------------------------------------------------------------------

type identityProfileRequest struct {
	TokenId                int    `json:"token_id"`
	IdentityMode           string `json:"identity_mode"`
	AttributionTargetType  string `json:"attribution_target_type"`
	IdentityAssurance      string `json:"identity_assurance"`
	CallerId               string `json:"caller_id"`
	CallerName             string `json:"caller_name"`
	PrincipalId            int    `json:"principal_id"`
	CredentialPurposeId    int    `json:"credential_purpose_id"`
	Environment            string `json:"environment"`
	RateLimitEnabled       bool   `json:"rate_limit_enabled"`
	RateLimitWindowSeconds int    `json:"rate_limit_window_seconds"`
	RateLimitMaxRequests   int    `json:"rate_limit_max_requests"`
	AppIds                 []int  `json:"app_ids"`
}

// identityProfileUpdateRequest 使用指针承载所有可选标量，以区分“省略（保持原值）”
// 与“显式 false / 0 / 空字符串”。转发给 service.IdentityProfilePatch。
type identityProfileUpdateRequest struct {
	IdentityMode           *string `json:"identity_mode"`
	AttributionTargetType  *string `json:"attribution_target_type"`
	IdentityAssurance      *string `json:"identity_assurance"`
	CallerId               *string `json:"caller_id"`
	CallerName             *string `json:"caller_name"`
	PrincipalId            *int    `json:"principal_id"`
	CredentialPurposeId    *int    `json:"credential_purpose_id"`
	Environment            *string `json:"environment"`
	RateLimitEnabled       *bool   `json:"rate_limit_enabled"`
	RateLimitWindowSeconds *int    `json:"rate_limit_window_seconds"`
	RateLimitMaxRequests   *int    `json:"rate_limit_max_requests"`
	Enabled                *bool   `json:"enabled"`
}

func GetIdentityProfiles(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	keyword := c.Query("keyword")
	enabled, err := aiGovernanceEnabledParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	identityMode := c.Query("identity_mode")
	tokenID, err := aiGovernanceIntParam(c, "token_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	items, total, err := service.ListIdentityProfiles(page, pageSize, keyword, enabled, identityMode, tokenID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 列表与详情复用同一 DTO 构建路径（6.14），避免漂移。
	details := make([]gin.H, 0, len(items))
	for i := range items {
		d, err := buildIdentityProfileDetail(&items[i])
		if err != nil {
			common.ApiError(c, err)
			return
		}
		details = append(details, d)
	}
	common.ApiSuccess(c, listResult(details, total, page, pageSize))
}

func GetIdentityProfile(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	p, err := service.GetIdentityProfile(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	detail, err := buildIdentityProfileDetail(p)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

func CreateIdentityProfile(c *gin.Context) {
	var req identityProfileRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	p := &model.AIIdentityProfile{
		TokenId:                req.TokenId,
		IdentityMode:           req.IdentityMode,
		AttributionTargetType:  req.AttributionTargetType,
		IdentityAssurance:      req.IdentityAssurance,
		CallerId:               req.CallerId,
		CallerName:             req.CallerName,
		PrincipalId:            req.PrincipalId,
		CredentialPurposeId:    req.CredentialPurposeId,
		Environment:            req.Environment,
		RateLimitEnabled:       req.RateLimitEnabled,
		RateLimitWindowSeconds: req.RateLimitWindowSeconds,
		RateLimitMaxRequests:   req.RateLimitMaxRequests,
		Enabled:                false,
	}
	created, err := service.CreateIdentityProfile(p, req.AppIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, created)
}

func UpdateIdentityProfile(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var req identityProfileUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	// 停用与字段更新合并为一次原子调用，避免两步操作后半步失败留下不一致状态。
	patch := &service.IdentityProfilePatch{
		Id:                     id,
		IdentityMode:           req.IdentityMode,
		AttributionTargetType:  req.AttributionTargetType,
		IdentityAssurance:      req.IdentityAssurance,
		CallerId:               req.CallerId,
		CallerName:             req.CallerName,
		PrincipalId:            req.PrincipalId,
		CredentialPurposeId:    req.CredentialPurposeId,
		Environment:            req.Environment,
		RateLimitEnabled:       req.RateLimitEnabled,
		RateLimitWindowSeconds: req.RateLimitWindowSeconds,
		RateLimitMaxRequests:   req.RateLimitMaxRequests,
		Enabled:                req.Enabled,
	}
	updated, err := service.UpdateIdentityProfile(patch)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, updated)
}

type replaceAppBindingsRequest struct {
	AppIds []int `json:"app_ids"`
}

func ReplaceIdentityProfileAppBindings(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var req replaceAppBindingsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "请求体解析失败")
		return
	}
	bindings, err := service.ReplaceAppBindings(id, req.AppIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, bindings)
}

// buildIdentityProfileDetail 构建 Profile 详情：Token 安全元数据 + 模式/target/assurance +
// Principal/Purpose（含稳定 code/name）+ Caller + bindings（含 domain/owner team）+ risk +
// rate-limit 配置 + 空事件摘要。列表与详情复用本函数。绝不返回 Token Key / 密钥密文。
func buildIdentityProfileDetail(p *model.AIIdentityProfile) (gin.H, error) {
	// Token 引用缺失或 DB 错误必须报错，不得静默返回空/半详情。
	token, err := model.GetTokenById(p.TokenId)
	if err != nil {
		return nil, fmt.Errorf("Profile 引用的 NewAPI Token(id=%d)读取失败: %w", p.TokenId, err)
	}
	tokenMeta := gin.H{
		"token_id":         token.Id,
		"token_name":       token.Name,
		"status":           token.Status,
		"expired_time":     token.ExpiredTime,
		"unlimited":        token.UnlimitedQuota,
		"ip_restricted":    len(token.GetIpLimits()) > 0,
		"model_restricted": token.ModelLimitsEnabled && strings.TrimSpace(token.ModelLimits) != "",
		"remain_quota":     token.RemainQuota,
		"created_time":     token.CreatedTime,
	}
	risk := service.ComputeCredentialRisk(token, types.ProfileRateLimit{
		Enabled:       p.RateLimitEnabled,
		WindowSeconds: p.RateLimitWindowSeconds,
		MaxRequests:   p.RateLimitMaxRequests,
	}, p.IdentityAssurance == constant.IdentityAssuranceCredentialOnly)

	// Principal / Purpose 稳定 code/name：引用缺失或 DB 错误必须报错，不得静默半详情。
	principal := gin.H{}
	if p.PrincipalId > 0 {
		pr, err := service.GetPrincipal(p.PrincipalId)
		if err != nil {
			return nil, fmt.Errorf("Profile 引用的使用主体(id=%d)读取失败: %w", p.PrincipalId, err)
		}
		principal = gin.H{
			"principal_id":   pr.Id,
			"principal_code": pr.PrincipalCode,
			"principal_name": pr.PrincipalName,
		}
	}
	purpose := gin.H{}
	if p.CredentialPurposeId > 0 {
		pu, err := service.GetCredentialPurpose(p.CredentialPurposeId)
		if err != nil {
			return nil, fmt.Errorf("Profile 引用的凭证用途(id=%d)读取失败: %w", p.CredentialPurposeId, err)
		}
		purpose = gin.H{
			"credential_purpose_id":   pu.Id,
			"credential_purpose_code": pu.PurposeCode,
			"credential_purpose_name": pu.PurposeName,
		}
	}

	bindings, err := service.ListAppBindings(p.Id)
	if err != nil {
		return nil, err
	}
	bindingViews := make([]gin.H, 0, len(bindings))
	for _, b := range bindings {
		view := gin.H{"id": b.Id, "app_id": b.AppId, "enabled": b.Enabled}
		app, err := model.GetAIBusinessApplication(b.AppId)
		if err != nil {
			return nil, fmt.Errorf("绑定应用(id=%d)读取失败: %w", b.AppId, err)
		}
		view["app_code"] = app.AppCode
		view["app_name"] = app.AppName
		if app.BusinessDomainID > 0 {
			d, err := service.GetBusinessDomain(app.BusinessDomainID)
			if err != nil {
				return nil, fmt.Errorf("应用(id=%d)所属业务领域(id=%d)读取失败: %w", app.Id, app.BusinessDomainID, err)
			}
			view["business_domain_id"] = d.Id
			view["business_domain_code"] = d.DomainCode
			view["business_domain_name"] = d.DomainName
		}
		if app.OwnerTeamID > 0 {
			ot, err := service.GetOwnerTeam(app.OwnerTeamID)
			if err != nil {
				return nil, fmt.Errorf("应用(id=%d)负责团队(id=%d)读取失败: %w", app.Id, app.OwnerTeamID, err)
			}
			view["owner_team_id"] = ot.Id
			view["owner_team_code"] = ot.TeamCode
			view["owner_team_name"] = ot.TeamName
		}
		bindingViews = append(bindingViews, view)
	}

	return gin.H{
		"profile": gin.H{
			"id":                        p.Id,
			"token_id":                  p.TokenId,
			"identity_mode":             p.IdentityMode,
			"attribution_target_type":   p.AttributionTargetType,
			"identity_assurance":        p.IdentityAssurance,
			"caller_id":                 p.CallerId,
			"caller_name":               p.CallerName,
			"principal_id":              p.PrincipalId,
			"credential_purpose_id":     p.CredentialPurposeId,
			"environment":               p.Environment,
			"rate_limit_enabled":        p.RateLimitEnabled,
			"rate_limit_window_seconds": p.RateLimitWindowSeconds,
			"rate_limit_max_requests":   p.RateLimitMaxRequests,
			"enabled":                   p.Enabled,
			"created_at":                p.CreatedAt,
			"updated_at":                p.UpdatedAt,
		},
		"principal": principal,
		"purpose":   purpose,
		"token":     tokenMeta,
		"bindings":  bindingViews,
		"risk":      risk,
		"rate_limits": gin.H{
			"items": []gin.H{},
			"config": gin.H{
				"enabled":        p.RateLimitEnabled,
				"window_seconds": p.RateLimitWindowSeconds,
				"max_requests":   p.RateLimitMaxRequests,
			},
		}, // 第一批无运行时事件，返回空摘要
	}, nil
}

// ---------------------------------------------------------------------------
// Signing Key
// ---------------------------------------------------------------------------

func GetSigningKeys(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	keys, err := service.ListSigningKeysMeta(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 仅返回元数据，绝不含密文/明文。
	views := make([]gin.H, 0, len(keys))
	for _, k := range keys {
		views = append(views, signingKeyMetaView(k))
	}
	common.ApiSuccess(c, views)
}

func GenerateSigningKey(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	key, secret, err := service.GenerateSigningKey(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 明文只在此次响应返回一次。
	common.ApiSuccess(c, gin.H{"key": signingKeyMetaView(*key), "secret": secret})
}

func RotateSigningKey(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	key, secret, err := service.RotateSigningKey(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 明文只在此次响应返回一次。
	common.ApiSuccess(c, gin.H{"key": signingKeyMetaView(*key), "secret": secret})
}

func RevokeSigningKey(c *gin.Context) {
	id, err := aiGovernanceID(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	keyID := c.Param("key_id")
	if keyID == "" {
		common.ApiErrorMsg(c, "key_id 不能为空")
		return
	}
	if err := service.RevokeSigningKey(id, keyID); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"revoked": true})
}

// signingKeyMetaView 返回签名密钥元数据视图，显式排除 SecretCiphertext 与明文。
func signingKeyMetaView(k model.AIIdentitySigningKey) gin.H {
	return gin.H{
		"id":         k.Id,
		"profile_id": k.ProfileId,
		"key_id":     k.KeyId,
		"status":     k.Status,
		"not_before": k.NotBefore,
		"expires_at": k.ExpiresAt,
		"revoked_at": k.RevokedAt,
		"created_at": k.CreatedAt,
		"updated_at": k.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Identity Audit
// ---------------------------------------------------------------------------

func GetIdentityAuditEvents(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	requestID := c.Query("request_id")
	tokenID, err := aiGovernanceIntParam(c, "token_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	profileID, err := aiGovernanceIntParam(c, "profile_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	result := c.Query("result")
	reasonCode := c.Query("reason_code")

	var events []model.AIIdentityAuditEvent
	q := model.DB.Model(&model.AIIdentityAuditEvent{})
	if requestID != "" {
		q = q.Where("request_id = ?", requestID)
	}
	if tokenID > 0 {
		q = q.Where("token_id = ?", tokenID)
	}
	if profileID > 0 {
		q = q.Where("profile_id = ?", profileID)
	}
	if result != "" {
		q = q.Where("result = ?", result)
	}
	if reasonCode != "" {
		q = q.Where("reason_code = ?", reasonCode)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := q.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&events).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, listResult(events, total, page, pageSize))
}
