package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildWechatOAuth2AuthorizeURL(t *testing.T) {
	url, err := BuildWechatOAuth2AuthorizeURL(
		"wx_test_appid",
		"https://allrouter.ai/wechat/scan",
		"",
		"",
	)
	require.NoError(t, err)
	require.Contains(t, url, "open.weixin.qq.com/connect/oauth2/authorize")
	require.Contains(t, url, "appid=wx_test_appid")
	require.Contains(t, url, "response_type=code")
	require.Contains(t, url, "scope=snsapi_base") // 留空默认静默授权
	require.Contains(t, url, "redirect_uri=")
	require.True(t, len(url) >= len("#wechat_redirect"))
	require.Equal(t, "#wechat_redirect", url[len(url)-len("#wechat_redirect"):])
}

func TestBuildWechatOAuth2AuthorizeURLValidation(t *testing.T) {
	_, err := BuildWechatOAuth2AuthorizeURL("", "https://a.b/c", "", "")
	require.Error(t, err)
	_, err = BuildWechatOAuth2AuthorizeURL("wx", "", "", "")
	require.Error(t, err)
}

func TestExchangeCodeForToken(t *testing.T) {
	var gotQuery map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotQuery = map[string]string{
			"path":       r.URL.Path,
			"appid":      q.Get("appid"),
			"secret":     q.Get("secret"),
			"code":       q.Get("code"),
			"grant_type": q.Get("grant_type"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":7200,"refresh_token":"RT","openid":"OPENID_VALUE","scope":"snsapi_base"}`))
	}))
	defer server.Close()

	svc := NewWechatOAuth2Service()
	svc.SetHost(server.URL)

	token, err := svc.ExchangeCodeForToken(context.Background(), "wx_appid", "secret", "CODE")
	require.NoError(t, err)
	require.Equal(t, "OPENID_VALUE", token.OpenID)
	require.Equal(t, "snsapi_base", token.Scope)
	require.Equal(t, 7200, token.ExpiresIn)
	require.Equal(t, 0, token.IsSnapshotUser)

	require.Equal(t, wechatOAuth2AccessTokenPath, gotQuery["path"])
	require.Equal(t, "wx_appid", gotQuery["appid"])
	require.Equal(t, "secret", gotQuery["secret"])
	require.Equal(t, "CODE", gotQuery["code"])
	require.Equal(t, "authorization_code", gotQuery["grant_type"])
}

func TestExchangeCodeForTokenBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":40163,"errmsg":"code been used"}`))
	}))
	defer server.Close()

	svc := NewWechatOAuth2Service()
	svc.SetHost(server.URL)

	_, err := svc.ExchangeCodeForToken(context.Background(), "wx_appid", "secret", "CODE")
	require.Error(t, err)

	var oauthErr *WechatOAuth2Error
	require.True(t, errors.As(err, &oauthErr))
	require.Equal(t, 40163, oauthErr.ErrCode)
}

func TestExchangeCodeForTokenMissingParams(t *testing.T) {
	svc := NewWechatOAuth2Service()
	_, err := svc.ExchangeCodeForToken(context.Background(), "", "secret", "code")
	require.Error(t, err)
	_, err = svc.ExchangeCodeForToken(context.Background(), "appid", "", "code")
	require.Error(t, err)
	_, err = svc.ExchangeCodeForToken(context.Background(), "appid", "secret", "")
	require.Error(t, err)
}

func TestExchangeCodeForTokenEmptyOpenID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":7200,"scope":"snsapi_base"}`))
	}))
	defer server.Close()

	svc := NewWechatOAuth2Service()
	svc.SetHost(server.URL)

	_, err := svc.ExchangeCodeForToken(context.Background(), "wx_appid", "secret", "CODE")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty openid")
}
