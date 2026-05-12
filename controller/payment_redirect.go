package controller

import (
	"log"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func getPaymentTrustedDomains(c *gin.Context) []string {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	if providerId <= 0 {
		return nil
	}
	domains, err := model.GetProviderVerifiedDomains(providerId)
	if err != nil {
		log.Printf("failed to load provider trusted domains: %v", err)
		return []string{}
	}
	return domains
}

func getStripeTrustedDomains(c *gin.Context) []string {
	return getPaymentTrustedDomains(c)
}
