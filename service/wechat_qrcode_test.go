package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildWechatPermanentQRCodeBodySceneStr(t *testing.T) {
	body, err := buildWechatPermanentQRCodeBody(&WechatPermanentQRCodeRequest{SceneStr: "placeholder_scene_str"})
	require.NoError(t, err)
	require.Equal(t, wechatQRCodeActionLimitStrScene, body.ActionName)
	require.Equal(t, "placeholder_scene_str", body.ActionInfo.Scene.SceneStr)
	require.Nil(t, body.ActionInfo.Scene.SceneID)
	// 永久二维码绝不能带 expire_seconds。
	require.Nil(t, body.ExpireSeconds)
}

func TestBuildWechatPermanentQRCodeBodySceneID(t *testing.T) {
	sceneID := 123
	body, err := buildWechatPermanentQRCodeBody(&WechatPermanentQRCodeRequest{SceneID: &sceneID})
	require.NoError(t, err)
	require.Equal(t, wechatQRCodeActionLimitScene, body.ActionName)
	require.NotNil(t, body.ActionInfo.Scene.SceneID)
	require.Equal(t, 123, *body.ActionInfo.Scene.SceneID)
	require.Empty(t, body.ActionInfo.Scene.SceneStr)
	require.Nil(t, body.ExpireSeconds)
}

func TestBuildWechatPermanentQRCodeBodyRejectsEmptyScene(t *testing.T) {
	_, err := buildWechatPermanentQRCodeBody(&WechatPermanentQRCodeRequest{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "scene_str or scene_id")
}

func TestBuildWechatPermanentQRCodeBodySceneStrTooLong(t *testing.T) {
	longScene := make([]byte, wechatQRCodeSceneStrMaxLen+1)
	for i := range longScene {
		longScene[i] = 'a'
	}
	_, err := buildWechatPermanentQRCodeBody(&WechatPermanentQRCodeRequest{SceneStr: string(longScene)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "scene_str length")
}

func TestBuildWechatPermanentQRCodeBodySceneIDOutOfRange(t *testing.T) {
	sceneID := wechatQRCodeSceneIDMaxValue + 1
	_, err := buildWechatPermanentQRCodeBody(&WechatPermanentQRCodeRequest{SceneID: &sceneID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "scene_id must be between")
}

func TestBuildWechatTemporaryQRCodeBodyDefaultsToTenMinutes(t *testing.T) {
	body, expireSeconds, err := buildWechatTemporaryQRCodeBody(&WechatTemporaryQRCodeRequest{SceneStr: "tmp_scene"})
	require.NoError(t, err)
	require.Equal(t, int64(600), expireSeconds)
	require.NotNil(t, body.ExpireSeconds)
	require.Equal(t, int64(600), *body.ExpireSeconds)
	require.Equal(t, wechatQRCodeActionStrScene, body.ActionName)
	require.Equal(t, "tmp_scene", body.ActionInfo.Scene.SceneStr)
}

func TestBuildWechatTemporaryQRCodeBodySceneID(t *testing.T) {
	sceneID := 456
	body, expireSeconds, err := buildWechatTemporaryQRCodeBody(&WechatTemporaryQRCodeRequest{SceneID: &sceneID})
	require.NoError(t, err)
	require.Equal(t, int64(600), expireSeconds)
	require.Equal(t, wechatQRCodeActionScene, body.ActionName)
	require.NotNil(t, body.ActionInfo.Scene.SceneID)
	require.Equal(t, 456, *body.ActionInfo.Scene.SceneID)
}

func TestBuildWechatTemporaryQRCodeBodyCustomExpire(t *testing.T) {
	custom := int64(1200)
	body, expireSeconds, err := buildWechatTemporaryQRCodeBody(&WechatTemporaryQRCodeRequest{SceneStr: "tmp_scene", ExpireSeconds: &custom})
	require.NoError(t, err)
	require.Equal(t, int64(1200), expireSeconds)
	require.NotNil(t, body.ExpireSeconds)
	require.Equal(t, int64(1200), *body.ExpireSeconds)
}

func TestBuildWechatTemporaryQRCodeBodyExpireOutOfRange(t *testing.T) {
	tooLong := int64(wechatQRCodeExpireSecondsMax + 1)
	_, _, err := buildWechatTemporaryQRCodeBody(&WechatTemporaryQRCodeRequest{SceneStr: "tmp_scene", ExpireSeconds: &tooLong})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expire_seconds must be between")
}

func TestBuildWechatTemporaryQRCodeBodyAllowsLargeSceneID(t *testing.T) {
	// 临时二维码 scene_id 是 32 位非 0 整型，可超过永久二维码的 100000 上限。
	sceneID := wechatQRCodeSceneIDMaxValue + 1
	_, _, err := buildWechatTemporaryQRCodeBody(&WechatTemporaryQRCodeRequest{SceneID: &sceneID})
	require.NoError(t, err)
}
