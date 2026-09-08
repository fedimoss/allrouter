package model

import (
	"database/sql/driver"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// PermissionList 存储授予普通用户的页面级模块权限 key（与前端侧边栏 itemKey 一致），
// 持久化在 users.permissions 列（TEXT 中的 JSON 数组）。
// 角色为管理员/超管的用户天然拥有全部权限，该字段仅对普通用户(role=1)生效。
type PermissionList []string

// Value 实现 driver.Valuer：写入数据库时序列化为 JSON 数组文本。
func (p PermissionList) Value() (driver.Value, error) {
	if len(p) == 0 {
		return "[]", nil
	}
	data, err := common.Marshal(p)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// Scan 实现 sql.Scanner：从数据库读取 JSON 数组文本，兼容 NULL/空串/[]byte。
func (p *PermissionList) Scan(value any) error {
	if value == nil {
		*p = nil
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into PermissionList", value)
	}
	if len(raw) == 0 {
		*p = nil
		return nil
	}
	// 统一走 ParsePermissionList，过滤历史遗留的已下线模块 key
	*p = ParsePermissionList(string(raw))
	return nil
}

// HasAny 判断权限列表是否包含给定模块中的任意一个。
func (p PermissionList) HasAny(modules ...string) bool {
	if len(p) == 0 || len(modules) == 0 {
		return false
	}
	granted := make(map[string]struct{}, len(p))
	for _, m := range p {
		granted[m] = struct{}{}
	}
	for _, m := range modules {
		if _, ok := granted[m]; ok {
			return true
		}
	}
	return false
}

// JSONString 返回权限列表的 JSON 文本表示（空列表返回 "[]"），用于写入用户缓存。
func (p PermissionList) JSONString() string {
	if len(p) == 0 {
		return "[]"
	}
	data, err := common.Marshal(p)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// ParsePermissionList 从 JSON 文本解析权限列表，空/非法输入返回空列表。
// 仅保留两份目录内的合法 key，历史遗留的已下线模块（如 user）自动失效。
func ParsePermissionList(s string) PermissionList {
	list := PermissionList{}
	if s == "" {
		return list
	}
	if err := common.Unmarshal([]byte(s), &list); err != nil {
		common.SysLog("failed to parse user permission list: " + err.Error())
		return PermissionList{}
	}
	result := make(PermissionList, 0, len(list))
	for _, key := range list {
		if IsValidPermissionKey(key) {
			result = append(result, key)
		}
	}
	return result
}

// 主站"管理员"导航分组的可授权模块（不含 root 专属的系统设置，不含用户管理）。
var MainSitePermissionModules = []string{
	"channel",
	"subscription",
	"models",
	"deployment",
	"callLog",
	"provider",
	"providerProfits",
	"providerWithdraw",
	"billing",
	"operational",
	"reconciliation",
	"redemption",
	"questionSurvey",
}

// 服务商站点"服务商"导航分组的可授权模块（不含系统设置 providerSetting、
// 不含用户管理 providerUsers——用户管理仅服务商属主可用）。
var ProviderSitePermissionModules = []string{
	"provider",
	"providerOperational",
	"providerWithdraw",
	"providerReward",
	"providerRewardReport",
	"providerRedemption",
	"providerSubscription",
	"providerProfits",
	"providerLogs",
	"providerQuestionSurvey",
}

// permissionCatalog 按目标用户所属站点返回可授权模块目录：
// 主站用户(provider_id=0)只能被授予主站模块，服务商用户只能被授予服务商模块。
func permissionCatalog(providerId int) []string {
	if providerId > 0 {
		return ProviderSitePermissionModules
	}
	return MainSitePermissionModules
}

// IsValidPermissionModule 校验模块 key 是否属于目标用户所属站点的可授权目录。
func IsValidPermissionModule(providerId int, module string) bool {
	for _, m := range permissionCatalog(providerId) {
		if m == module {
			return true
		}
	}
	return false
}

// IsValidPermissionKey 校验模块 key 是否属于任一站点的可授权目录。
func IsValidPermissionKey(module string) bool {
	for _, m := range MainSitePermissionModules {
		if m == module {
			return true
		}
	}
	for _, m := range ProviderSitePermissionModules {
		if m == module {
			return true
		}
	}
	return false
}

// SanitizePermissionModules 去重并过滤掉不属于目标站点目录的模块 key。
func SanitizePermissionModules(providerId int, modules []string) PermissionList {
	if len(modules) == 0 {
		return PermissionList{}
	}
	catalog := permissionCatalog(providerId)
	allowed := make(map[string]struct{}, len(catalog))
	for _, m := range catalog {
		allowed[m] = struct{}{}
	}
	seen := make(map[string]struct{}, len(modules))
	result := make(PermissionList, 0, len(modules))
	for _, m := range modules {
		if _, ok := allowed[m]; !ok {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		result = append(result, m)
	}
	return result
}

// UpdateUserPermissions 单独更新用户的模块权限并清理用户缓存。
// permissions 为空切片表示清空全部授权。
func UpdateUserPermissions(userId int, permissions PermissionList) error {
	if err := DB.Model(&User{}).Where("id = ?", userId).Update("permissions", permissions).Error; err != nil {
		return err
	}
	return invalidateUserCache(userId)
}
