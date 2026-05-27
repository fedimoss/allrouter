package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func getProviderUserById(providerId int, userId int, selectAll bool) (*model.User, error) {
	if providerId <= 0 || userId <= 0 {
		return nil, errors.New("invalid provider or user id")
	}
	user := model.User{}
	query := model.DB.Where("id = ? AND provider_id = ?", userId, providerId)
	if !selectAll {
		query = query.Omit("password")
	}
	if err := query.First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func listProviderUsers(providerId int, pageInfo *common.PageInfo) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64
	query := model.DB.Unscoped().Model(&model.User{}).Where("provider_id = ?", providerId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Omit("password").Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func searchProviderUsers(providerId int, keyword string, group string, startIdx int, num int) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64
	query := model.DB.Unscoped().Model(&model.User{}).Where("provider_id = ?", providerId)
	keyword = strings.TrimSpace(keyword)
	group = strings.TrimSpace(group)
	if group != "" {
		query = query.Where(&model.User{Group: group})
	}
	if keyword != "" {
		likeCondition := "username LIKE ? OR email LIKE ? OR display_name LIKE ?"
		if keywordInt, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR "+likeCondition, keywordInt, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		} else {
			query = query.Where(likeCondition, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Omit("password").Order("id desc").Limit(num).Offset(startIdx).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func GetProviderUsers(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	users, total, err := listProviderUsers(provider.Id, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

// 服务商用户管理的树形型结构
func GetTreeProviderUsers(c *gin.Context) {
	userId := c.GetInt("id")

	if userId == 0 {
		common.ApiError(c, errors.New("invalid user id"))
		return
	}
	parentIdstr := c.Query("parent_id")
	if parentIdstr == "" {
		parentIdstr = c.Query("parentId")
	}
	if parentIdstr == "" {
		parentIdstr = strconv.Itoa(userId)
	}
	parentId, err := strconv.Atoi(parentIdstr)
	if err != nil {
		common.ApiError(c, errors.New("format parent id"))
		return
	}
	pageInfo := common.GetPageQuery(c)

	users, err := model.GetTreeChilendUsers(userId, parentId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

func SearchProviderUsers(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	users, total, err := searchProviderUsers(provider.Id, c.Query("keyword"), c.Query("group"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, pageInfo)
}

func GetProviderUser(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := getProviderUserById(provider.Id, userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, user)
}

func GetProviderUserInvitees(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	inviterId, err := strconv.Atoi(c.Param("id"))
	if err != nil || inviterId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var inviter model.User
	if err := model.DB.Unscoped().Omit("password").Where("id = ? AND provider_id = ?", inviterId, provider.Id).First(&inviter).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	users, total, err := model.GetInvitedUsersByInviterIdInProvider(provider.Id, inviterId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(users)
	common.ApiSuccess(c, gin.H{
		"inviter": gin.H{
			"id":           inviter.Id,
			"username":     inviter.Username,
			"display_name": inviter.DisplayName,
			"role":         inviter.Role,
			"aff_count":    inviter.AffCount,
		},
		"page":      pageInfo.Page,
		"page_size": pageInfo.PageSize,
		"total":     pageInfo.Total,
		"items":     pageInfo.Items,
	})
}

func CreateProviderUser(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	var user model.User
	if err := common.DecodeJson(c.Request.Body, &user); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user.Username = strings.TrimSpace(user.Username)
	if user.Username == "" || user.Password == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := common.Validate.Struct(&user); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": err.Error()})
		return
	}
	exist, err := model.CheckUserExistOrDeletedInProvider(provider.Id, user.Username, user.Email)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if exist {
		common.ApiErrorI18n(c, i18n.MsgUserExists)
		return
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	cleanUser := model.User{
		ProviderId:   provider.Id,
		Username:     user.Username,
		Password:     user.Password,
		DisplayName:  user.DisplayName,
		Role:         common.RoleCommonUser,
		Status:       common.UserStatusEnabled,
		Group:        user.Group,
		Remark:       user.Remark,
		SignupSource: "provider_admin",
	}
	if cleanUser.Group == "" {
		cleanUser.Group = "default"
	}
	if err := cleanUser.Insert(0); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cleanUser)
}

func UpdateProviderUser(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	var req model.User
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.Id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	originUser, err := getProviderUserById(provider.Id, req.Id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if originUser.Role != common.RoleCommonUser {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	req.ProviderId = provider.Id
	req.Role = common.RoleCommonUser
	if req.Password == "" {
		req.Password = "$I_LOVE_U"
	}
	if err := common.Validate.Struct(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": err.Error()})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username != "" && req.Username != originUser.Username {
		exists, err := model.UsernameConflictsWithProviderLoginScope(provider.Id, req.Username, req.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if exists {
			common.ApiErrorI18n(c, i18n.MsgUserExists)
			return
		}
	}
	if req.Password == "$I_LOVE_U" {
		req.Password = ""
	}
	updatePassword := req.Password != ""
	cleanUser := model.User{
		Id:          req.Id,
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Group:       req.Group,
		Remark:      req.Remark,
	}
	if cleanUser.Username == "" {
		cleanUser.Username = originUser.Username
	}
	if cleanUser.DisplayName == "" {
		cleanUser.DisplayName = cleanUser.Username
	}
	if cleanUser.Group == "" {
		cleanUser.Group = originUser.Group
	}
	if err := cleanUser.Edit(updatePassword); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ManageProviderUser(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	var req ManageRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.Id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := getProviderUserById(provider.Id, req.Id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Role != common.RoleCommonUser {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	switch req.Action {
	case "disable":
		user.Status = common.UserStatusDisabled
	case "enable":
		user.Status = common.UserStatusEnabled
	case "delete":
		if err := user.Delete(); err != nil {
			common.ApiError(c, err)
			return
		}
		model.RecordLog(user.Id, model.LogTypeManage, fmt.Sprintf("provider owner %s deleted provider user %s", c.GetString("username"), user.Username))
		common.ApiSuccess(c, gin.H{"status": user.Status, "role": user.Role})
		return
	default:
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"status": user.Status, "role": user.Role})
}

func DeleteProviderUser(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := getProviderUserById(provider.Id, userId, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "user not found"})
			return
		}
		common.ApiError(c, err)
		return
	}
	if user.Role != common.RoleCommonUser {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionHigherLevel)
		return
	}
	if err := user.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
