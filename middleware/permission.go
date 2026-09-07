package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// AdminOrAnyModuleAuth 放行管理员或持有任意主站模块权限的普通用户，
// 用于分组列表等跨页面共享的只读查询接口。
func AdminOrAnyModuleAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		if !authCheck(c, common.RoleCommonUser) {
			return
		}
		if c.GetInt("role") >= common.RoleAdminUser {
			c.Next()
			return
		}
		userCache, err := model.GetUserCache(c.GetInt("id"))
		if err != nil {
			common.SysLog("AdminOrAnyModuleAuth GetUserCache error: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": common.TranslateMessage(c, i18n.MsgDatabaseError),
			})
			c.Abort()
			return
		}
		// 与 AdminOrModuleAuth 一致：服务商站点成员不进入主站管理接口
		if userCache.ProviderId > 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": common.TranslateMessage(c, i18n.MsgAuthInsufficientPrivilege),
			})
			c.Abort()
			return
		}
		if len(userCache.GetPermissionList()) > 0 {
			c.Next()
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgAuthInsufficientPrivilege),
		})
		c.Abort()
	}
}

// AdminOrModuleAuth 在 AdminAuth 的基础上放行"被授予对应模块权限的普通用户"。
// 规则：
//   - 管理员(role>=10)/超管直接放行，行为与 AdminAuth 一致；
//   - 普通用户仅当其 permissions 中包含任一给定模块 key 时放行（仅 role=1 的用户会被授予模块权限）。
//
// 用于主站管理员路由组。路由自身叠加的 RootAuth 等更严中间件不受影响。
func AdminOrModuleAuth(modules ...string) func(c *gin.Context) {
	return func(c *gin.Context) {
		if !authCheck(c, common.RoleCommonUser) {
			return
		}
		if c.GetInt("role") >= common.RoleAdminUser {
			c.Next()
			return
		}
		userCache, err := model.GetUserCache(c.GetInt("id"))
		if err != nil {
			common.SysLog("AdminOrModuleAuth GetUserCache error: " + err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": common.TranslateMessage(c, i18n.MsgDatabaseError),
			})
			c.Abort()
			return
		}
		// 主站管理接口不对服务商站点成员开放：
		// 其模块权限仅作用于本站 /api/provider/* 资源（由 getPermittedProvider 按租户校验），
		// 防止主站/服务商同名模块 key（provider/providerWithdraw/providerProfits）跨站泄漏
		if userCache.ProviderId > 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": common.TranslateMessage(c, i18n.MsgAuthInsufficientPrivilege),
			})
			c.Abort()
			return
		}
		if userCache.GetPermissionList().HasAny(modules...) {
			c.Next()
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgAuthInsufficientPrivilege),
		})
		c.Abort()
	}
}
