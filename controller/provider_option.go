package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
)

func canManageProviderOptions(c *gin.Context, providerId int) bool {
	provider, err := model.GetProviderById(providerId)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if c.GetInt("role") >= common.RoleAdminUser {
		return true
	}
	if provider.OwnerUserId == c.GetInt("id") {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": "无权访问该服务商配置",
	})
	return false
}

// GetProviderOptions 获取服务商配置
func GetProviderOptions(c *gin.Context) {
	// 服务商ID
	providerId, err := strconv.Atoi(c.Param("id"))

	// 验证服务商ID是否有效
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的服务商ID",
		})
		return
	}
	if !canManageProviderOptions(c, providerId) {
		return
	}

	// 获取服务商配置
	options, err := model.GetProviderOptions(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    options,
	})
}

// UpdateProviderOption 更新服务商配置
func UpdateProviderOption(c *gin.Context) {
	providerId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的服务商ID",
		})
		return
	}
	if !canManageProviderOptions(c, providerId) {
		return
	}

	var option OptionUpdateRequest
	err = common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	// 转换值为字符串
	switch option.Value.(type) {
	case bool:
		option.Value = common.Interface2String(option.Value.(bool))
	case float64:
		option.Value = common.Interface2String(option.Value.(float64))
	case int:
		option.Value = common.Interface2String(option.Value.(int))
	default:
		option.Value = fmt.Sprintf("%v", option.Value)
	}

	switch option.Key {
	// 系统公告
	case "console_setting.announcements":
		// 验证系统公告是否符合要求
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}

	// 更新服务商配置
	err = model.UpdateProviderOption(providerId, option.Key, option.Value.(string))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}
