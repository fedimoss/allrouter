package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type authInfo struct {
	State  string `json:"state"`
	Status string `json:"status"`
	Url    string `json:"url"`
}

type authState struct {
	Error  string `json:"error,omitempty"`
	Status string `json:"status"`
}

type UserAuth struct {
	CliUserId string `json:"cli_user_id"`
	ModelAuth int    `json:"model_auth"`
	State     string `json:"state"`
	UserId    string `json:"user_id"`
}
type UpOFStateReq struct {
	Id       string `json:"id"`
	Disabled bool   `json:"disabled"` //true是停用
	AuthType int    `json:"auth_type"`
}

type OAuthCallReq struct {
	Provider     string `json:"provider"`
	Redirect_url string `json:"redirect_url"`
}

var (
	mu           sync.Mutex
	userAuthInfo = make(map[string]UserAuth, 0) //key为state
)

// /v0/management/codex-auth-url get
func GetCodexAuthUrl(c *gin.Context) {
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	auInfo, err := getAuthUrlByType(c, cliUrl, cliPassword, constant.AuthCodex)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"success": true,
		"data":    auInfo,
	})

}

// /v0/management/qwen-auth-url get

func GetQwenAuthUrl(c *gin.Context) {
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	auInfo, err := getAuthUrlByType(c, cliUrl, cliPassword, constant.AuthQwen)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"success": true,
		"data":    auInfo,
	})
}

// /v0/management/anthropic-auth-url get
func GetAnthropicAuthUrl(c *gin.Context) {
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	auInfo, err := getAuthUrlByType(c, cliUrl, cliPassword, constant.AuthAnthropic)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"success": true,
		"data":    auInfo,
	})

}

// /v0/management/antigravity-auth-url get
func GetAntigravityAuthUrl(c *gin.Context) {
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	auInfo, err := getAuthUrlByType(c, cliUrl, cliPassword, constant.AuthAntigravity)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"success": true,
		"data":    auInfo,
	})

}

// /v0/management/gemini-cli-auth-url get
func GetGeminiCliAuthUrl(c *gin.Context) {
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	auInfo, err := getAuthUrlByType(c, cliUrl, cliPassword, constant.AuthGeminiCli)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"success": true,
		"data":    auInfo,
	})

}

// /v0/management/kimi-auth-url get
func GetKimiAuthUrl(c *gin.Context) {
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	auInfo, err := getAuthUrlByType(c, cliUrl, cliPassword, constant.AuthKimi)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"success": true,
		"data":    auInfo,
	})

}

// v0/management/iflow-auth-url post
func IflowAuth(c *gin.Context) {
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	userId := c.GetInt("id")
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}

	cliUrl = cliUrl + "/v0/management/iflow-auth-url"
	req := struct {
		Cookie string `json:"cookie"`
	}{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false})
		return
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	request, err := http.NewRequest("POST", cliUrl, bytes.NewBuffer(reqBytes))
	client := http.Client{}

	resp, err := client.Do(request)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	if resp.StatusCode != 200 {
		logger.LogError(c, fmt.Sprintf("get auth url failed, status code: %d,resp message: %s", resp.StatusCode, string(body)))
		c.JSON(http.StatusOK, gin.H{
			"message": err,
			"success": false,
		})
		return
	}
	aAuthUrl := &authInfo{}
	err = json.Unmarshal(body, aAuthUrl)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err,
			"success": false,
		})
		return
	}
	if aAuthUrl.Status != "ok" {
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Errorf("get auth url not success, status code: %d,resp message: %s", resp.StatusCode, string(body)),
			"success": false,
		})
		return
	}

	//写入到redis中，如果开启的话，没有就写到内存中，额还是直接写到内存中吧
	mu.Lock()
	defer mu.Unlock()

	apiById, err := model.GetAPIKeyByUserId(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}

	userAuthInfo[aAuthUrl.State] = UserAuth{
		CliUserId: apiById.Id,
		ModelAuth: constant.AuthIFlow,
		State:     aAuthUrl.State,
		UserId:    strconv.Itoa(userId),
	}

	common.ApiSuccess(c, aAuthUrl)
}

// v0/management/oauth-callback post
func OAuthCallBack(c *gin.Context) {
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	var oac OAuthCallReq
	err := c.ShouldBindJSON(&oac)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	call, err := oauthCall(c, cliUrl, cliPassword, &oac)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, call)
	return

}

// /v0/management/get-auth-status get
func GetAuthStatus(c *gin.Context) {
	state := c.Query("state")
	if state == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "Missing parameters",
			"success": false,
		})

		return
	}

	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	status, err := getAuthStatus(c, cliUrl, cliPassword, state)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	common.ApiSuccess(c, status)
	return
}

// v0/management/useroauths get
func GetUserOAuths(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	modelType := c.Query("modeType")
	keyWord := c.Query("keyWord")
	var (
		co    []*model.CliOAuth
		total int64
		err   error
	)

	var modelInt int64
	if modelType != "" {
		modelInt, err = strconv.ParseInt(modelType, 10, 64)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}

	co, total, err = model.GetCliOAuthPages(userId, pageInfo, modelInt, keyWord)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(co)
	common.ApiSuccess(c, pageInfo)
}

// 删除认证文件
// /v0/management/delete-oauth delete
func DeleteOAuth(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiError(c, errors.New("user is not found"))
		return
	}
	oauthId := c.Param("id")
	if oauthId == "" {
		common.ApiError(c, errors.New("oauthId is not found"))
		return
	}
	dinfo, err := model.DeleteCliOAuth(userId, oauthId)
	if dinfo == nil || err != nil {
		common.ApiError(c, err)
		return
	}
	var modeType int
	if dinfo.ReMainModelCount == 0 {
		switch dinfo.AuthType { //1: Codex 2: Anthropic 3: Qwen 对应数据库存储
		case 1:
			modeType = constant.AuthCodex
		case 2:
			modeType = constant.AuthAnthropic
		case 3:
			modeType = constant.AuthQwen
		}
		//删除渠道中的密钥
		err = updateOAuthChannelKey(modeType, dinfo.CliUserId, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}

	common.ApiSuccess(c, gin.H{"message": "success"})
	return
}

// 下载认证文件 /v0/management/downloadoauth get
func DownloadOauth(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiError(c, errors.New("user is not found"))
		return
	}
	oauthId := c.Query("oauthId")
	if oauthId == "" {
		common.ApiError(c, errors.New("oauthId is not found"))
		return
	}
	clioath, err := model.GetOAuthByIdAndUserId(userId, oauthId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 2. 序列化为 JSON
	jsonData, err := json.MarshalIndent(clioath, "", "  ")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 3. 设置下载响应头
	c.Header("Content-Disposition", "attachment; filename=oauth.json")
	c.Header("Content-Type", "application/json")
	c.Header("Content-Length", strconv.Itoa(len(jsonData)))

	// 4. 写出数据
	c.Data(http.StatusOK, "application/json", jsonData)
}

// 修改认证文件状态 /vo/management/auth-files/status post
func UpdateAuthFileStatus(c *gin.Context) {
	var req UpOFStateReq
	err := c.ShouldBindJSON(&req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiError(c, errors.New("user is not found"))
		return
	}
	//暂时不做这个认证文件是否是这个用户的校验了
	//获取管理员填写的cli proxy地址
	cliUrl := common.OptionMap["CLIServerAddress"]
	if cliUrl == "" && !strings.HasPrefix(cliUrl, "http://") && !strings.HasPrefix(cliUrl, "https://") {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli url not found",
			"success": false,
		})
		return
	}
	//获取管理员填写的cli proxy端口
	cliPassword := common.OptionMap["CLIProxyAPIPassword"]
	if strings.TrimSpace(cliPassword) == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": "cli password not found",
			"success": false,
		})
		return
	}
	err = updateOAuthFileStatus(c, cliUrl, cliPassword, &req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"message": "success"})
	return
}

// 获取用户成功认证的服务数量 /v0/management/get-oauth-success-count get
func GetUserAuthSuccessCount(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiError(c, errors.New("user is not found"))
		return
	}
	countInfo, errr := model.GetCliUserOAuthCountByUserId(userId)
	if errr != nil {
		common.ApiError(c, errr)
		return
	}
	common.ApiSuccess(c, gin.H{
		"count":       countInfo.Count,
		"model_types": countInfo.ModelTypes,
	})
	return
}

// **************************func
func getAuthUrlByType(c *gin.Context, cliUrl string, cliPassword string, authType int) (*authInfo, error) {
	if strings.HasSuffix(cliUrl, "/") {
		cliUrl = cliUrl[:len(cliUrl)-1]
	}

	switch authType {
	case constant.AuthQwen:
		cliUrl = cliUrl + "/v0/management/qwen-auth-url"
	case constant.AuthCodex:
		cliUrl = cliUrl + "/v0/management/codex-auth-url?is_webui=true"
	case constant.AuthAnthropic:
		cliUrl = cliUrl + "/v0/management/anthropic-auth-url?is_webui=true"
	case constant.AuthAntigravity: //谷歌的反重力
		cliUrl = cliUrl + "/v0/management/antigravity-auth-url?is_webui=true"
	case constant.AuthGeminiCli:
		pid := c.Query("project_id")
		if pid != "" {
			cliUrl = cliUrl + "/v0/management/gemini-cli-auth-url?is_webui=true&project_id=" + pid
		} else {
			cliUrl = cliUrl + "/v0/management/gemini-cli-auth-url?is_webui=true"
		}

	case constant.AuthKimi:
		cliUrl = cliUrl + "/v0/management/kimi-auth-url"

	default:
		return nil, fmt.Errorf("auth type not found")

	}
	request, err := http.NewRequest("GET", cliUrl, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+cliPassword)
	query := request.URL.Query()
	userId := c.GetInt("id")
	if userId == 0 {
		return nil, errors.New("user is not found")
	}
	cliUserId, err := model.GetAPIKeyByUserId(userId)
	if err != nil {
		return nil, err
	}
	if cliUserId == nil {
		return nil, errors.New("cli user is not found")
	}
	query.Add("cli_user_id", cliUserId.Id)
	request.URL.RawQuery = query.Encode() //写回到url中使其生效
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		logger.LogError(c, fmt.Sprintf("get auth url failed, status code: %d,resp message: %s", resp.StatusCode, string(body)))
		return nil, fmt.Errorf("get auth url failed, status code: %d,resp message: %s", resp.StatusCode, string(body))
	}
	aAuthUrl := &authInfo{}
	err = json.Unmarshal(body, aAuthUrl)

	if err != nil {
		return nil, err
	}
	if aAuthUrl.Status != "ok" {
		return nil, fmt.Errorf("get auth url not success, status code: %d,resp message: %s", resp.StatusCode, string(body))
	}

	//写入到redis中，如果开启的话，没有就写到内存中，额还是直接写到内存中吧
	mu.Lock()
	defer mu.Unlock()
	apiById, err := model.GetAPIKeyByUserId(userId)
	if err != nil {
		return nil, err
	}

	userAuthInfo[aAuthUrl.State] = UserAuth{
		CliUserId: apiById.Id,
		ModelAuth: authType,
		State:     aAuthUrl.State,
		UserId:    strconv.Itoa(userId),
	}

	return aAuthUrl, nil
}

func getAuthStatus(c *gin.Context, cliUrl string, cliPassword string, state string) (*authState, error) {
	if strings.HasSuffix(cliUrl, "/") {
		cliUrl = cliUrl[:len(cliUrl)-1]
	}
	cliUrl = cliUrl + "/v0/management/get-auth-status"
	request, err := http.NewRequest("GET", cliUrl, nil)
	if err != nil {
		return nil, err
	}
	query := request.URL.Query()

	query.Add("state", state)
	request.URL.RawQuery = query.Encode()
	request.Header.Set("Authorization", "Bearer "+cliPassword)
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		logger.LogError(c, fmt.Sprintf("get auth status failed, status code: %d,resp message: %s", resp.StatusCode, string(body)))
		return nil, fmt.Errorf("get auth status failed, status code: %d,resp message: %s", resp.StatusCode, string(body))
	}
	aAuthstate := &authState{}
	err = common.Unmarshal(body, aAuthstate)
	if err != nil {
		return nil, err

	}
	//判断是否成功
	if aAuthstate.Status == "ok" && aAuthstate.Error == "" {
		if auth, exits := userAuthInfo[state]; exits {
			if err = updateOAuthChannelKey(auth.ModelAuth, auth.CliUserId, true); err != nil {
				return nil, err
			}
		}

		return aAuthstate, nil
	}
	return aAuthstate, nil
}

// 请求cli proxy接口去修改文件状态
func updateOAuthFileStatus(c *gin.Context, cliUrl string, cliPassword string, req *UpOFStateReq) error {
	if strings.HasSuffix(cliUrl, "/") {
		cliUrl = cliUrl[:len(cliUrl)-1]
	}
	cliUrl = cliUrl + "/v0/management/auth-files/status"
	request, err := http.NewRequest("PATCH", cliUrl, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+cliPassword)
	body, err := common.Marshal(struct {
		Name     string `json:"name"`
		Disabled bool   `json:"disabled"`
	}{
		Name:     req.Id,
		Disabled: req.Disabled,
	})
	if err != nil {
		return err
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		logger.LogError(c, fmt.Sprintf("update auth file status failed, status code: %d,error: %s", resp.StatusCode, string(respBody)))
	}
	ofResp := struct {
		Disabled bool   `json:"disabled"`
		Status   string `json:"status"`
	}{}
	err = common.Unmarshal(respBody, &ofResp)
	if err != nil {
		return err
	}
	if ofResp.Status != "ok" {
		return errors.New("update oauth file status failed")
	}
	userId := c.GetInt("id")
	if userId == 0 {
		return errors.New("user not found")
	}
	clio, err := model.GetAPIKeyByUserId(userId)
	if err != nil {
		return err
	}
	return updateOAuthChannelKey(req.AuthType, clio.Id, false)

}

func getOAuthChannelByType(authType int) (*model.Channel, error) {
	switch authType {
	case constant.AuthCodex,
		constant.AuthAnthropic,
		constant.AuthAntigravity,
		constant.AuthGeminiCli,
		constant.AuthKimi,
		constant.AuthQwen,
		constant.AuthIFlow:
		return model.GetChannelById(authType, true)
	default:
		return nil, errors.New("auth type not support")
	}
}

func updateOAuthChannelKey(authType int, cliUserID string, add bool) error {
	cliUserID = strings.TrimSpace(cliUserID)
	if cliUserID == "" {
		return errors.New("cli user id is empty")
	}

	channel, err := getOAuthChannelByType(authType)
	if err != nil {
		return err
	}

	key := strings.TrimSpace(channel.Key)
	if key == "" {
		return nil
	}
	if key == "init_password" {
		channel.Key = ""
	}

	keys := splitOAuthChannelKeys(channel.Key)
	if add {
		if containsOAuthChannelKey(keys, cliUserID) {
			return nil
		}
		keys = append(keys, cliUserID)
	} else {
		filtered := make([]string, 0, len(keys))
		removed := false
		for _, keyItem := range keys {
			if keyItem == cliUserID {
				removed = true
				continue
			}
			filtered = append(filtered, keyItem)
		}
		if !removed {
			return nil
		}
		keys = filtered
	}

	channel.Key = strings.Join(keys, "\n")
	if err = model.UpdateCliUserOAuth(channel); err != nil {
		return err
	}

	model.InitChannelCache()
	service.ResetProxyClientCache()
	return nil
}

func splitOAuthChannelKeys(key string) []string {
	rawKeys := strings.Split(strings.TrimSpace(key), "\n")
	keys := make([]string, 0, len(rawKeys))
	for _, keyItem := range rawKeys {
		keyItem = strings.TrimSpace(keyItem)
		if keyItem == "" {
			continue
		}
		keys = append(keys, keyItem)
	}
	return keys
}

func containsOAuthChannelKey(keys []string, target string) bool {
	for _, key := range keys {
		if key == target {
			return true
		}
	}
	return false
}

func oauthCall(c *gin.Context, cliUrl string, cliPassWord string, oreq *OAuthCallReq) (*authState, error) {
	if strings.HasSuffix(cliUrl, "/") {
		cliUrl = cliUrl[:len(cliUrl)-1]
	}
	cliUrl = cliUrl + "/v0/management/oauth-callback"

	oreqBytes, err := json.Marshal(oreq)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", cliUrl, bytes.NewReader(oreqBytes))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+cliPassWord)
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		logger.LogError(c, fmt.Sprintf("oauth call failed, status code: %d,error: %s", resp.StatusCode, string(respBody)))
		return nil, errors.New(string(respBody))
	}
	var as authState
	err = json.Unmarshal(respBody, &as)
	if err != nil {
		return nil, err
	}
	//判断是否成功
	if as.Status == "ok" && as.Error == "" {
		userId := c.GetInt("id")
		if userId == 0 {
			return nil, errors.New("user not found")
		}
		clio, err1 := model.GetAPIKeyByUserId(userId)
		if err1 != nil {
			return nil, err1
		}
		var modelAuth int
		switch oreq.Provider {
		case "codex":
			modelAuth = constant.AuthCodex
		case "anthropic":
			modelAuth = constant.AuthAnthropic
		case "antigravity":
			modelAuth = constant.AuthAntigravity
		case "gemini":
			modelAuth = constant.AuthGeminiCli

		}
		if err = updateOAuthChannelKey(modelAuth, clio.Id, true); err != nil {
			return nil, err
		}

		return &as, nil
	}
	return &as, nil

}
