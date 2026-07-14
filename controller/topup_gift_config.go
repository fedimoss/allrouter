package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type TopUpGiftPublicConfig struct {
	Enabled bool                       `json:"enabled"`
	Rules   []model.TopUpGiftRule      `json:"rules"`
	Timed   model.TopUpGiftTimedConfig `json:"timed"`
}

// GetTopUpGiftConfig returns the configuration for the site resolved from the request domain.
func GetTopUpGiftConfig(c *gin.Context) {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	config, err := model.LoadTopUpGiftConfig(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	timed, err := model.LoadTopUpGiftTimedConfig(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, TopUpGiftPublicConfig{
		Enabled: config.Enabled,
		Rules:   config.Rules,
		Timed:   timed,
	})
}
