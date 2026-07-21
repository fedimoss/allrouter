package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestGetNoticeLanguageAndProviderVisibility 覆盖公告接口的两个维度：
//  1. 语言选择优先级：URL 的 language 参数 > 登录用户的语言设置 > 浏览器 Accept-Language；
//     中文（简体/繁体）返回中文公告，其余语言一律返回英文公告。
//  2. 服务商站点可见性：服务商访问时受 NoticeShowToProviders 开关控制。
func TestGetNoticeLanguageAndProviderVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldOptionMap := common.OptionMap
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	testCases := []struct {
		name             string // 用例名称
		providerId       int    // 服务商 ID，>0 表示以服务商身份访问
		showToProviders  string // NoticeShowToProviders 配置值（"true"/"false"）
		acceptLanguage   string // 模拟的浏览器 Accept-Language 请求头
		userLanguage     string // 登录用户的语言设置（模拟已鉴权用户，空表示未登录）
		selectedLanguage string // 前端显式传入的 language 查询参数（手动切换语言）
		expectedNotice   string // 期望接口返回的公告内容
	}{
		{
			name:            "simplified Chinese uses Chinese notice",
			showToProviders: "false",
			acceptLanguage:  "zh-CN",
			expectedNotice:  "\u7ef4\u62a4\u516c\u544a",
		},
		{
			name:            "traditional Chinese uses Chinese notice",
			showToProviders: "false",
			acceptLanguage:  "zh-TW",
			expectedNotice:  "\u7ef4\u62a4\u516c\u544a",
		},
		{
			name:            "English uses English notice",
			showToProviders: "false",
			acceptLanguage:  "en-US",
			expectedNotice:  "maintenance notice",
		},
		{
			name:            "other languages use English notice",
			showToProviders: "false",
			acceptLanguage:  "ja-JP",
			expectedNotice:  "maintenance notice",
		},
		{
			name:            "logged in user setting overrides browser language",
			showToProviders: "false",
			acceptLanguage:  "zh-CN",
			userLanguage:    "en",
			expectedNotice:  "maintenance notice",
		},
		{
			name:            "logged in traditional Chinese setting overrides browser language",
			showToProviders: "false",
			acceptLanguage:  "en-US",
			userLanguage:    "zh-TW",
			expectedNotice:  "\u7ef4\u62a4\u516c\u544a",
		},
		{
			name:             "selected traditional Chinese overrides logged in English setting",
			showToProviders:  "false",
			acceptLanguage:   "en-US",
			userLanguage:     "en",
			selectedLanguage: "zh-TW",
			expectedNotice:   "\u7ef4\u62a4\u516c\u544a",
		},
		{
			name:             "selected non-Chinese language overrides logged in Chinese setting",
			showToProviders:  "false",
			acceptLanguage:   "zh-CN",
			userLanguage:     "zh-CN",
			selectedLanguage: "ja",
			expectedNotice:   "maintenance notice",
		},
		{
			name:            "provider site sees selected notice when enabled",
			providerId:      42,
			showToProviders: "true",
			acceptLanguage:  "en",
			expectedNotice:  "maintenance notice",
		},
		{
			name:            "provider site does not see notice when disabled",
			providerId:      42,
			showToProviders: "false",
			acceptLanguage:  "en",
			expectedNotice:  "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			common.OptionMapRWMutex.Lock()
			common.OptionMap = map[string]string{
				model.NoticeOptionKey:                "\u7ef4\u62a4\u516c\u544a",
				model.NoticeEnglishOptionKey:         "maintenance notice",
				model.NoticeShowToProvidersOptionKey: testCase.showToProviders,
			}
			common.OptionMapRWMutex.Unlock()

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/notice", nil)
			if testCase.selectedLanguage != "" {
				query := ctx.Request.URL.Query()
				query.Set("language", testCase.selectedLanguage)
				ctx.Request.URL.RawQuery = query.Encode()
			}
			ctx.Request.Header.Set("Accept-Language", testCase.acceptLanguage)
			common.SetContextKey(ctx, constant.ContextKeyProviderId, testCase.providerId)
			if testCase.userLanguage != "" {
				common.SetContextKey(ctx, constant.ContextKeyUserSetting, dto.UserSetting{
					Language: testCase.userLanguage,
				})
			}

			GetNotice(ctx)

			var response struct {
				Success bool   `json:"success"`
				Data    string `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success)
			require.Equal(t, testCase.expectedNotice, response.Data)
		})
	}
}
