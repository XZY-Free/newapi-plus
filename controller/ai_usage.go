package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// parseUsageFilter 从查询参数构建企业用量筛选条件（§12.7）。
func parseUsageFilter(c *gin.Context) model.UsageProjectionFilter {
	f := model.UsageProjectionFilter{}
	f.BucketStart, _ = strconv.ParseInt(c.Query("bucket_start"), 10, 64)
	f.BucketEnd, _ = strconv.ParseInt(c.Query("bucket_end"), 10, 64)
	f.ProfileID, _ = strconv.Atoi(c.Query("profile_id"))
	f.PrincipalID, _ = strconv.Atoi(c.Query("principal_id"))
	f.CredentialPurposeID, _ = strconv.Atoi(c.Query("credential_purpose_id"))
	f.UsageBusinessDomainID, _ = strconv.Atoi(c.Query("usage_business_domain_id"))
	f.UsageTeamID, _ = strconv.Atoi(c.Query("usage_team_id"))
	f.CallerKey = c.Query("caller_key")
	f.RootAppCode = c.Query("root_app_code")
	f.AppBusinessDomainID, _ = strconv.Atoi(c.Query("app_business_domain_id"))
	f.OwnerTeamID, _ = strconv.Atoi(c.Query("owner_team_id"))
	f.IdentityAssurance = c.Query("identity_assurance")
	f.ModelName = c.Query("model_name")
	return f
}

// GetEnterpriseUsageStats 企业用量统计（§12.7 / E.2 P1-E 服务端分页）。
// 返回 items/total/page/page_size，排序 bucket_time DESC, id DESC。
func GetEnterpriseUsageStats(c *gin.Context) {
	page, pageSize := aiGovernancePagination(c)
	rows, total, err := service.QueryUsageStats(parseUsageFilter(c), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query usage stats failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"items": rows, "total": total, "page": page, "page_size": pageSize,
	}})
}

// RebuildEnterpriseUsageProjection 重建指定时间范围的用量投影（§12.6，Root-only）。
// query: start, end（Unix 秒）。
func RebuildEnterpriseUsageProjection(c *gin.Context) {
	start, _ := strconv.ParseInt(c.Query("start"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end"), 10, 64)
	if end <= start || start <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time range"})
		return
	}
	total, err := service.ProjectUsageRange(c.Request.Context(), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rebuild failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"processed_logs": total}})
}

// GetEnterpriseUsageAnomalies 弱身份确定性异常检测（§12.5 / E.2 P1-D）。
// query: bucket_start, bucket_end（Unix 秒，后端自身规范化到整点小时）。
func GetEnterpriseUsageAnomalies(c *gin.Context) {
	start, _ := strconv.ParseInt(c.Query("bucket_start"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("bucket_end"), 10, 64)
	if start == 0 {
		start = end - 3600
	}
	if end == 0 {
		end = start + 3600
	}
	anomalies, err := service.DetectUsageAnomalies(c.Request.Context(), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query usage anomalies failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": anomalies})
}
