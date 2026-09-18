package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestDouyinCardTransformBody 添加/修改转发前对请求体的统一加工：
// 注入系统设置的跳转链接、补齐审计字段（当前用户 ID + 秒级时间戳）；
// 未配置且无需补齐、请求体缺失或不是 JSON 对象时原样透传。
func TestDouyinCardTransformBody(t *testing.T) {
	s := system_setting.GetDouyinCardSettings()
	oldLink := s.WxAppPageURL
	defer func() { s.WxAppPageURL = oldLink }()

	readAll := func(r io.Reader) string {
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		return string(data)
	}
	parse := func(raw string) map[string]any {
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &out))
		return out
	}

	t.Run("注入跳转链接", func(t *testing.T) {
		// 地址两侧带空白：应去掉空白后提交
		s.WxAppPageURL = " weixin://dl/business/?appid=xxxxx&path=xxxxx "
		out := parse(readAll(douyinCardTransformBody(
			strings.NewReader(`{"title":"添加客服","link":"weixin://old"}`), 0, true, false)))
		// Marshal 会把 URL 里的 & 转义为 JSON 等价的 Unicode 转义，按解析结果断言
		require.Equal(t, "weixin://dl/business/?appid=xxxxx&path=xxxxx", out["link"])
		require.Equal(t, "添加客服", out["title"])
	})

	t.Run("图片相对路径按服务器地址拼接", func(t *testing.T) {
		s.WxAppPageURL = ""
		oldAddr := system_setting.ServerAddress
		defer func() { system_setting.ServerAddress = oldAddr }()
		// 服务器地址两侧带空白、末尾带斜杠：拼接时应归一化
		system_setting.ServerAddress = " https://allrouter.ai/ "
		out := parse(readAll(douyinCardTransformBody(strings.NewReader(
			`{"cover":"/static/card/logo/1/a.jpg","wxAvatar":"/static/card/weixin/1/b.jpg","wxQr":"https://old.example/c.jpg"}`), 0, false, false)))
		require.Equal(t, "https://allrouter.ai/static/card/logo/1/a.jpg", out["cover"])
		require.Equal(t, "https://allrouter.ai/static/card/weixin/1/b.jpg", out["wxAvatar"])
		// 已是绝对地址的旧数据原样保留
		require.Equal(t, "https://old.example/c.jpg", out["wxQr"])
	})

	t.Run("服务器地址未配置时保留相对路径", func(t *testing.T) {
		s.WxAppPageURL = ""
		oldAddr := system_setting.ServerAddress
		defer func() { system_setting.ServerAddress = oldAddr }()
		system_setting.ServerAddress = ""
		body := `{"cover":"/static/card/logo/1/a.jpg"}`
		require.Equal(t, body, readAll(douyinCardTransformBody(
			strings.NewReader(body), 0, false, false)))
	})

	t.Run("补齐审计字段", func(t *testing.T) {
		s.WxAppPageURL = ""
		before := time.Now().Unix()
		out := parse(readAll(douyinCardTransformBody(
			strings.NewReader(`{"title":"添加客服"}`), 1001, false, true)))
		require.Equal(t, "1001", out["createdBy"])
		require.Equal(t, "1001", out["updatedBy"])
		require.GreaterOrEqual(t, out["createdAt"], float64(before))
		require.GreaterOrEqual(t, out["updatedAt"], float64(before))
	})

	t.Run("添加场景同时注入链接与审计字段", func(t *testing.T) {
		s.WxAppPageURL = "weixin://dl/business/?appid=xxxxx&path=xxxxx"
		out := parse(readAll(douyinCardTransformBody(
			strings.NewReader(`{"title":"添加客服"}`), 1001, true, true)))
		require.Equal(t, s.WxAppPageURL, out["link"])
		require.Equal(t, "1001", out["createdBy"])
		require.NotContains(t, out, "link_placeholder")
	})

	t.Run("未配置地址且无需审计时原样透传", func(t *testing.T) {
		s.WxAppPageURL = ""
		body := `{"title":"添加客服","link":"weixin://old"}`
		require.Equal(t, body, readAll(douyinCardTransformBody(
			strings.NewReader(body), 1001, true, false)))
	})

	t.Run("未知用户不补审计字段", func(t *testing.T) {
		s.WxAppPageURL = ""
		body := `{"title":"添加客服"}`
		require.Equal(t, body, readAll(douyinCardTransformBody(
			strings.NewReader(body), 0, false, true)))
	})

	t.Run("非JSON对象原样透传", func(t *testing.T) {
		s.WxAppPageURL = "weixin://dl/business/?appid=xxxxx"
		require.Equal(t, "not json", readAll(douyinCardTransformBody(
			strings.NewReader("not json"), 1001, true, true)))
	})

	t.Run("空请求体原样透传", func(t *testing.T) {
		s.WxAppPageURL = "weixin://dl/business/?appid=xxxxx"
		require.Equal(t, "", readAll(douyinCardTransformBody(
			strings.NewReader(""), 1001, true, true)))
	})
}

// TestDeleteDouyinCardPathPlaceholder 删除接口路径配置了 {id} 占位符时
// （如 /openapi/card/{id}），转发前应替换为真实主键，且不再追加查询参数 id。
func TestDeleteDouyinCardPathPlaceholder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath, gotMethod, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod, gotQuery = r.URL.Path, r.Method, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	// SSRF 防护会拦截 127.0.0.1 测试服务，测试内临时关闭
	fetch := system_setting.GetFetchSetting()
	oldSSRF := fetch.EnableSSRFProtection
	defer func() { fetch.EnableSSRFProtection = oldSSRF }()
	fetch.EnableSSRFProtection = false

	s := system_setting.GetDouyinCardSettings()
	oldDelete := s.Delete
	defer func() { s.Delete = oldDelete }()
	s.Delete = system_setting.DouyinCardEndpoint{
		URL:    srv.URL + "/openapi/card/{id}",
		Method: http.MethodDelete,
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "abc123"}}
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/douyin_card/abc123", nil)

	DeleteDouyinCard(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "/openapi/card/abc123", gotPath)
	require.Equal(t, http.MethodDelete, gotMethod)
	require.Equal(t, "", gotQuery, "占位符方式不应再追加查询参数 id")
}

// TestUpdateDouyinCardPathPlaceholder 修改接口路径配置了 {id} 占位符时
// （如 /openapi/card/{id}），转发前应取请求体中的卡片主键替换，请求体原样保留。
func TestUpdateDouyinCardPathPlaceholder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotPath, gotMethod, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotPath, gotMethod, gotBody = r.URL.Path, r.Method, string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	fetch := system_setting.GetFetchSetting()
	oldSSRF := fetch.EnableSSRFProtection
	defer func() { fetch.EnableSSRFProtection = oldSSRF }()
	fetch.EnableSSRFProtection = false

	s := system_setting.GetDouyinCardSettings()
	oldUpdate := s.Update
	oldLink := s.WxAppPageURL
	defer func() {
		s.Update = oldUpdate
		s.WxAppPageURL = oldLink
	}()
	s.Update = system_setting.DouyinCardEndpoint{
		URL:    srv.URL + "/openapi/card/{id}",
		Method: http.MethodPut,
	}
	s.WxAppPageURL = ""

	body := `{"id":"abc123","title":"点击咨询"}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/douyin_card/", strings.NewReader(body))

	UpdateDouyinCard(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "/openapi/card/abc123", gotPath, "{id} 应替换为请求体中的主键")
	require.Equal(t, http.MethodPut, gotMethod)
	require.JSONEq(t, body, gotBody, "请求体应原样透传")
}

// TestGetDouyinCardsBuildsQueryBody 查询接口以 JSON 请求体调用外部服务：
// 分页/关键词由本服务查询参数映射为 { keyword, page: { pageNo, pageSize } }，
// userId 为当前登录用户 ID。
func TestGetDouyinCardsBuildsQueryBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotMethod, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotMethod, gotBody = r.Method, string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"items":[],"total":0}}`))
	}))
	defer srv.Close()

	fetch := system_setting.GetFetchSetting()
	oldSSRF := fetch.EnableSSRFProtection
	defer func() { fetch.EnableSSRFProtection = oldSSRF }()
	fetch.EnableSSRFProtection = false

	s := system_setting.GetDouyinCardSettings()
	oldQuery := s.Query
	defer func() { s.Query = oldQuery }()
	s.Query = system_setting.DouyinCardEndpoint{
		URL:    srv.URL + "/openapi/card/query",
		Method: http.MethodPost,
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 1847292357012580)
	// keyword=领取 的 URL 编码
	ctx.Request = httptest.NewRequest(
		http.MethodGet, "/api/douyin_card/?page=2&page_size=20&keyword=%E9%A2%86%E5%8F%96", nil)

	GetDouyinCards(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.MethodPost, gotMethod, "查询接口应按配置的 POST 方法调用")
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(gotBody), &body))
	require.Equal(t, "领取", body["keyword"])
	require.Equal(t, "1847292357012580", body["userId"])
	page, ok := body["page"].(map[string]any)
	require.True(t, ok, "page 应为嵌套对象")
	require.Equal(t, float64(2), page["pageNo"])
	require.Equal(t, float64(20), page["pageSize"])
}

// TestGetDouyinCardBaseUrl 基础URL只读接口：返回去掉末尾斜杠与空白的基础URL，
// 供控制台「复制」按钮拼接卡片链接。
func TestGetDouyinCardBaseUrl(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := system_setting.GetDouyinCardSettings()
	oldBase := s.BaseURL
	defer func() { s.BaseURL = oldBase }()
	s.BaseURL = " https://api.example.com/ "

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/douyin_card/base_url", nil)

	GetDouyinCardBaseUrl(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"baseUrl":"https://api.example.com"`)
}
