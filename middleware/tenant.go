package middleware

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TenantResolver() gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		providerCtx, err := model.GetProviderContextByDomain(host)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog("failed to resolve provider domain: " + err.Error())
		}
		if providerCtx != nil {
			common.SetContextKey(c, constant.ContextKeyProviderId, providerCtx.ProviderId)
			common.SetContextKey(c, constant.ContextKeyProviderOwnerUserId, providerCtx.OwnerUserId)
			common.SetContextKey(c, constant.ContextKeyProviderDomain, providerCtx.Domain)
			c.Set("provider_name", providerCtx.Name)
			if providerCtx.Config != nil {
				c.Set("provider_config", *providerCtx.Config)
			}
		} else {
			common.SetContextKey(c, constant.ContextKeyProviderId, 0)
			common.SetContextKey(c, constant.ContextKeyProviderOwnerUserId, 0)
		}
		c.Next()
	}
}
