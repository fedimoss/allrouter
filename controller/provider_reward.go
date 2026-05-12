package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetProviderRewardConfig(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	cfg, err := model.GetProviderRewardConfig(provider.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cfg)
}

func UpsertProviderRewardConfig(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	var cfg model.ProviderRewardConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		common.ApiError(c, err)
		return
	}
	cfg.ProviderId = provider.Id
	if err := model.UpsertProviderRewardConfig(&cfg); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cfg)
}

func AdminGetProviderRewardConfig(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	cfg, err := model.GetProviderRewardConfig(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cfg)
}

func AdminUpsertProviderRewardConfig(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	var cfg model.ProviderRewardConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		common.ApiError(c, err)
		return
	}
	cfg.ProviderId = id
	if err := model.UpsertProviderRewardConfig(&cfg); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cfg)
}

func GetProviderRewardSummary(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	summary, err := model.GetProviderRewardSummary(provider.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": summary})
}

func AdminGetProviderRewardSummary(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	summary, err := model.GetProviderRewardSummary(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": summary})
}
