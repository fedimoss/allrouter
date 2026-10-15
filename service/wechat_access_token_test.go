package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWechatAccessTokenServiceGetAccessToken(t *testing.T) {
	var gotQuery map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = map[string]string{
			"grant_type": r.URL.Query().Get("grant_type"),
			"appid":      r.URL.Query().Get("appid"),
			"secret":     r.URL.Query().Get("secret"),
			"path":       r.URL.Path,
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ACCESS_TOKEN_VALUE","expires_in":7200}`))
	}))
	defer server.Close()

	svc := NewWechatAccessTokenService()
	svc.SetHost(server.URL)
	svc.SetCredentials("wx_test_appid", "test_secret")

	token, err := svc.GetAccessToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ACCESS_TOKEN_VALUE", token.AccessToken)
	require.Equal(t, 7200, token.ExpiresIn)

	require.Equal(t, wechatAccessTokenPath, gotQuery["path"])
	require.Equal(t, wechatAccessTokenGrant, gotQuery["grant_type"])
	require.Equal(t, "wx_test_appid", gotQuery["appid"])
	require.Equal(t, "test_secret", gotQuery["secret"])
}

func TestWechatAccessTokenServiceBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":40125,"errmsg":"invalid appsecret"}`))
	}))
	defer server.Close()

	svc := NewWechatAccessTokenService()
	svc.SetHost(server.URL)

	_, err := svc.GetAccessToken(context.Background())
	require.Error(t, err)

	var apiErr *WechatAccessTokenError
	require.True(t, errors.As(err, &apiErr))
	require.Equal(t, 40125, apiErr.ErrCode)
	require.Equal(t, "invalid appsecret", apiErr.ErrMsg)
}

func TestWechatAccessTokenServiceMissingCredentials(t *testing.T) {
	svc := NewWechatAccessTokenService()
	svc.SetCredentials("", "")

	_, err := svc.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "appid is empty")
}

func TestWechatAccessTokenServiceEmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"expires_in":7200}`))
	}))
	defer server.Close()

	svc := NewWechatAccessTokenService()
	svc.SetHost(server.URL)

	_, err := svc.GetAccessToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty access_token")
}
