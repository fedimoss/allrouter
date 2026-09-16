package system_setting

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// DouyinCardEndpoint 单个接口的请求方法与路径，整体以 JSON 存储在同一个 option key 中
type DouyinCardEndpoint struct {
	URL    string `json:"url"`
	Method string `json:"method"`
}

type DouyinCardSettings struct {
	ApiKey  string
	BaseURL string
	Add     DouyinCardEndpoint
	Query   DouyinCardEndpoint
	Update  DouyinCardEndpoint
	Delete  DouyinCardEndpoint
}

// 默认配置：四个接口按 REST 习惯预置请求方法
var defaultDouyinCardSettings = DouyinCardSettings{
	Add:    DouyinCardEndpoint{Method: "POST"},
	Query:  DouyinCardEndpoint{Method: "GET"},
	Update: DouyinCardEndpoint{Method: "PUT"},
	Delete: DouyinCardEndpoint{Method: "DELETE"},
}

func GetDouyinCardSettings() *DouyinCardSettings {
	return &defaultDouyinCardSettings
}

var validDouyinCardMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

// UpdateDouyinCardFromOption 将大驼峰 option key 写入抖音私信卡片配置，
// 返回是否处理了该 key（供 model.updateOptionMap 调用）。
// 接口 key 的 value 为 JSON（{"url":"...","method":"..."}），解析失败时保留原值。
func UpdateDouyinCardFromOption(key, value string) bool {
	s := &defaultDouyinCardSettings
	switch key {
	case "DouyinCardApiKey":
		s.ApiKey = value
	case "DouyinCardBaseUrl":
		s.BaseURL = value
	case "DouyinCardAddApi":
		_ = common.Unmarshal([]byte(value), &s.Add)
	case "DouyinCardQueryApi":
		_ = common.Unmarshal([]byte(value), &s.Query)
	case "DouyinCardUpdateApi":
		_ = common.Unmarshal([]byte(value), &s.Update)
	case "DouyinCardDeleteApi":
		_ = common.Unmarshal([]byte(value), &s.Delete)
	default:
		return false
	}
	return true
}

// ValidateDouyinCardOption 校验抖音私信卡片 option 的 value 格式，
// 供 controller 在落库前调用；接口 key 必须是包含合法 method 的 JSON
func ValidateDouyinCardOption(key, value string) error {
	switch key {
	case "DouyinCardAddApi", "DouyinCardQueryApi", "DouyinCardUpdateApi", "DouyinCardDeleteApi":
		var ep DouyinCardEndpoint
		if err := common.Unmarshal([]byte(value), &ep); err != nil {
			return fmt.Errorf("接口配置必须是包含 url 和 method 的 JSON")
		}
		if !validDouyinCardMethods[ep.Method] {
			return fmt.Errorf("不支持的请求方法: %s", ep.Method)
		}
	}
	return nil
}
