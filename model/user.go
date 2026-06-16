package model

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const UserNameMaxLength = 20

// User if you add sensitive fields, don't forget to clean them in setupLogin function.
// Otherwise, the sensitive information will be saved on local storage in plain text!
type User struct {
	Id                         int            `json:"id"`
	ProviderId                 int            `json:"provider_id" gorm:"type:int;default:0;index;uniqueIndex:ux_user_provider_aff"`
	Username                   string         `json:"username" gorm:"index" validate:"max=20"`
	Password                   string         `json:"password" gorm:"not null;" validate:"min=8,max=20"`
	OriginalPassword           string         `json:"original_password" gorm:"-:all"` // this field is only for Password change verification, don't save it to database!
	DisplayName                string         `json:"display_name" gorm:"index" validate:"max=20"`
	Role                       int            `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status                     int            `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Email                      string         `json:"email" gorm:"index" validate:"max=50"`
	GitHubId                   string         `json:"github_id" gorm:"column:github_id;index"`
	DiscordId                  string         `json:"discord_id" gorm:"column:discord_id;index"`
	OidcId                     string         `json:"oidc_id" gorm:"column:oidc_id;index"`
	WeChatId                   string         `json:"wechat_id" gorm:"column:wechat_id;index"`
	TelegramId                 string         `json:"telegram_id" gorm:"column:telegram_id;index"`
	VerificationCode           string         `json:"verification_code" gorm:"-:all"`                                    // this field is only for Email verification, don't save it to database!
	AccessToken                *string        `json:"access_token" gorm:"type:char(32);column:access_token;uniqueIndex"` // this token is for system management
	Quota                      int            `json:"quota" gorm:"type:int;default:0"`
	RewardQuota                int            `json:"reward_quota" gorm:"type:int;default:0;column:reward_quota"`
	UsedQuota                  int            `json:"used_quota" gorm:"type:int;default:0;column:used_quota"` // used quota
	TotalTokenUsed             int64          `json:"total_token_used" gorm:"type:bigint;default:0;column:total_token_used"`
	RequestCount               int            `json:"request_count" gorm:"type:int;default:0;"` // request number
	Group                      string         `json:"group" gorm:"type:varchar(64);default:'default'"`
	InviteConsumeRebateEnabled bool           `json:"invite_consume_rebate_enabled" gorm:"type:boolean;default:false;column:invite_consume_rebate_enabled"`
	AffCode                    string         `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex:ux_user_provider_aff"`
	AffCount                   int            `json:"aff_count" gorm:"type:int;default:0;column:aff_count"`
	AffQuota                   int            `json:"aff_quota" gorm:"type:int;default:0;column:aff_quota"`           // 邀请剩余额度
	AffHistoryQuota            int            `json:"aff_history_quota" gorm:"type:int;default:0;column:aff_history"` // 邀请历史额度
	InviterId                  int            `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
	CreatedAt                  int64          `json:"created_at" gorm:"bigint;column:created_at"`
	DeletedAt                  gorm.DeletedAt `gorm:"index"`
	LinuxDOId                  string         `json:"linux_do_id" gorm:"column:linux_do_id;index"`
	Setting                    string         `json:"setting" gorm:"type:text;column:setting"`
	Remark                     string         `json:"remark,omitempty" gorm:"type:varchar(255)" validate:"max=255"`
	StripeCustomer             string         `json:"stripe_customer" gorm:"type:varchar(64);column:stripe_customer;index"`
	PhoneCountryCode           string         `json:"phone_country_code" gorm:"type:varchar(8);column:phone_country_code" validate:"max=8"` // 手机号国家区号（E.164），如 +86
	PhoneNumber                string         `json:"phone_number" gorm:"type:varchar(20);column:phone_number" validate:"max=20"`           // 手机号本地号码，不含国家区号，如 13800000000
	Timezone                   string         `json:"timezone" gorm:"type:varchar(64);column:timezone" validate:"max=64"`                   // 时区标识（IANA），如 Asia/Shanghai
	Avatar                     string         `json:"avatar" gorm:"type:varchar(255);column:avatar" validate:"max=255"`                     // 头像                   // 头像 URL
	SignupSource               string         `json:"signup_source" gorm:"type:varchar(64);column:signup_source" validate:"max=64"`         // 注册来源
}

func (user *User) ToBaseUser() *UserBase {
	cache := &UserBase{
		Id:         user.Id,
		ProviderId: user.ProviderId,
		Group:      user.Group,
		Quota:      user.Quota,
		Status:     user.Status,
		Username:   user.Username,
		Setting:    user.Setting,
		Email:      user.Email,
	}
	return cache
}

func (user *User) GetAccessToken() string {
	if user.AccessToken == nil {
		return ""
	}
	return *user.AccessToken
}

func (user *User) SetAccessToken(token string) {
	user.AccessToken = &token
}

func GetInvitedUsersByInviterId(inviterId int, pageInfo *common.PageInfo) (users []*User, total int64, err error) {
	if inviterId <= 0 {
		return nil, 0, errors.New("invalid inviter id")
	}
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Unscoped().Model(&User{}).Where("inviter_id = ?", inviterId)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.
		Omit("password").
		Order("created_at desc").
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&users).Error
	return users, total, err
}

func GetInvitedUsersByInviterIdInProvider(providerId int, inviterId int, pageInfo *common.PageInfo) (users []*User, total int64, err error) {
	if providerId <= 0 || inviterId <= 0 {
		return nil, 0, errors.New("invalid provider or inviter id")
	}
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Unscoped().Model(&User{}).Where("provider_id = ? AND inviter_id = ?", providerId, inviterId)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.
		Omit("password").
		Order("created_at desc").
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&users).Error
	return users, total, err
}

func (user *User) GetSetting() dto.UserSetting {
	setting := dto.UserSetting{}
	if user.Setting != "" {
		err := json.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

func (user *User) SetSetting(setting dto.UserSetting) {
	settingBytes, err := json.Marshal(setting)
	if err != nil {
		common.SysLog("failed to marshal setting: " + err.Error())
		return
	}
	user.Setting = string(settingBytes)
}

// 根据用户角色生成默认的边栏配置
func generateDefaultSidebarConfigForRole(userRole int) string {
	defaultConfig := map[string]interface{}{}

	// 聊天区域 - 所有用户都可以访问
	defaultConfig["chat"] = map[string]interface{}{
		"enabled":    true,
		"playground": true,
		"chat":       true,
	}

	// 控制台区域 - 所有用户都可以访问
	defaultConfig["console"] = map[string]interface{}{
		"enabled":    true,
		"detail":     true,
		"token":      true,
		"log":        true,
		"midjourney": true,
		"task":       true,
	}

	// 个人中心区域 - 所有用户都可以访问
	defaultConfig["personal"] = map[string]interface{}{
		"enabled":  true,
		"topup":    true,
		"personal": true,
	}
	//商家区域 -所有用户都可以访问
	defaultConfig["merchant"] = map[string]interface{}{
		"enabled":       true,
		"oauth":         true,
		"certification": true,
	}

	// 管理员区域 - 根据角色决定
	if userRole == common.RoleAdminUser {
		// 管理员可以访问管理员区域，但不能访问系统设置
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"provider":   true,
			"redemption": true,
			"user":       true,
			"setting":    false, // 管理员不能访问系统设置
		}
	} else if userRole == common.RoleRootUser {
		// 超级管理员可以访问所有功能
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"provider":   true,
			"redemption": true,
			"user":       true,
			"setting":    true,
		}
	}
	// 普通用户不包含admin区域

	// 转换为JSON字符串
	configBytes, err := json.Marshal(defaultConfig)
	if err != nil {
		common.SysLog("生成默认边栏配置失败: " + err.Error())
		return ""
	}

	return string(configBytes)
}

// CheckUserExistOrDeleted check if user exist or deleted, if not exist, return false, nil, if deleted or exist, return true, nil
func CheckUserExistOrDeleted(username string, email string) (bool, error) {
	return CheckUserExistOrDeletedInProvider(0, username, email)
}

func CheckUserExistOrDeletedGlobally(username string, email string) (bool, error) {
	return UserIdentityConflictsGlobally(0, username, email)
}

func UserIdentityConflictsGlobally(excludeUserId int, username string, email string) (bool, error) {
	usernameConflict, emailConflict, err := UserIdentityConflictFieldsGlobally(excludeUserId, username, email)
	return usernameConflict || emailConflict, err
}

func UserIdentityConflictFieldsGlobally(excludeUserId int, username string, email string) (usernameConflict bool, emailConflict bool, err error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	if username == "" && email == "" {
		return false, false, nil
	}
	if username != "" {
		query := DB.Unscoped().Model(&User{}).Where("username = ?", username)
		if excludeUserId > 0 {
			query = query.Where("id <> ?", excludeUserId)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return false, false, err
		}
		usernameConflict = count > 0
	}
	if email != "" {
		query := DB.Unscoped().Model(&User{}).Where("email = ?", email)
		if excludeUserId > 0 {
			query = query.Where("id <> ?", excludeUserId)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return false, false, err
		}
		emailConflict = count > 0
	}
	return usernameConflict, emailConflict, nil
}

func CheckUserExistOrDeletedInProvider(providerId int, username string, email string) (bool, error) {
	var user User

	// err := DB.Unscoped().First(&user, "username = ? or email = ?", username, email).Error
	// check email if empty
	var err error
	if email == "" {
		err = DB.Unscoped().First(&user, "provider_id = ? AND username = ?", providerId, username).Error
	} else {
		err = DB.Unscoped().First(&user, "provider_id = ? AND (username = ? or email = ?)", providerId, username, email).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if providerId <= 0 {
				return false, nil
			}
			ownerUserId, ownerErr := providerOwnerUserId(providerId)
			if ownerErr != nil {
				return false, ownerErr
			}
			if ownerUserId <= 0 {
				return false, nil
			}
			ownerQuery := DB.Unscoped().Model(&User{}).Where("id = ? AND username = ?", ownerUserId, username)
			if email != "" {
				ownerQuery = DB.Unscoped().Model(&User{}).Where("id = ? AND (username = ? OR email = ?)", ownerUserId, username, email)
			}
			var count int64
			if countErr := ownerQuery.Count(&count).Error; countErr != nil {
				return false, countErr
			}
			return count > 0, nil
		}
		// other error, return false, err
		return false, err
	}
	// exist, return true, nil
	return true, nil
}

func providerOwnerUserId(providerId int) (int, error) {
	if providerId <= 0 {
		return 0, nil
	}
	var provider Provider
	if err := DB.Select("owner_user_id").Where("id = ?", providerId).First(&provider).Error; err != nil {
		return 0, err
	}
	return provider.OwnerUserId, nil
}

func UsernameConflictsWithProviderLoginScope(providerId int, username string, excludeUserId int) (bool, error) {
	username = strings.TrimSpace(username)
	if providerId <= 0 || username == "" {
		return false, nil
	}
	var count int64
	query := DB.Unscoped().Model(&User{}).Where("provider_id = ? AND username = ?", providerId, username)
	if excludeUserId > 0 {
		query = query.Where("id <> ?", excludeUserId)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	ownerUserId, err := providerOwnerUserId(providerId)
	if err != nil {
		return false, err
	}
	if ownerUserId <= 0 || ownerUserId == excludeUserId {
		return false, nil
	}
	count = 0
	if err := DB.Unscoped().Model(&User{}).Where("id = ? AND username = ?", ownerUserId, username).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func UsernameConflictsWithOwnedProviderUsers(ownerUserId int, username string) (bool, error) {
	username = strings.TrimSpace(username)
	if ownerUserId <= 0 || username == "" {
		return false, nil
	}
	var providerIds []int
	if err := DB.Model(&Provider{}).Where("owner_user_id = ?", ownerUserId).Pluck("id", &providerIds).Error; err != nil {
		return false, err
	}
	if len(providerIds) == 0 {
		return false, nil
	}
	var count int64
	if err := DB.Unscoped().Model(&User{}).Where("provider_id IN ? AND username = ?", providerIds, username).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func UsernameConflictsForUserLoginScope(userId int, providerId int, username string) (bool, error) {
	if providerId > 0 {
		return UsernameConflictsWithProviderLoginScope(providerId, username, userId)
	}
	return UsernameConflictsWithOwnedProviderUsers(userId, username)
}

func GetMaxUserId() int {
	var user User
	DB.Unscoped().Last(&user)
	return user.Id
}

func GetAllUsers(pageInfo *common.PageInfo) (users []*User, total int64, err error) {
	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get total count within transaction
	err = tx.Unscoped().Model(&User{}).Count(&total).Error //包括软删除的数据
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated users within same transaction
	err = tx.Unscoped().Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Omit("password").Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetUserRecordsByCondition 条件分页查询用户列表
// sortFields: 排序字段映射，如 {"quota":"desc","topup_quota":"asc"}
func GetUserRecordsByCondition(pageInfo *common.PageInfo, sortFields map[string]string, startTimestamp int64, endTimestamp int64, usedQuotaMin int, usedQuotaMax int, quotaMin int, quotaMax int, requestCountMin int, requestCountMax int, keyword string) (users []*User, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Unscoped().Model(&User{})

	// 注册时间范围筛选
	if startTimestamp != 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}

	// 消耗金额范围筛选
	if usedQuotaMin > 0 {
		query = query.Where("used_quota >= ?", usedQuotaMin)
	}
	if usedQuotaMax > 0 {
		query = query.Where("used_quota <= ?", usedQuotaMax)
	}
	if quotaMin > 0 {
		query = query.Where("quota >= ?", quotaMin)
	}
	if quotaMax > 0 {
		query = query.Where("quota <= ?", quotaMax)
	}
	if requestCountMin > 0 {
		query = query.Where("request_count >= ?", requestCountMin)
	}
	if requestCountMax > 0 {
		query = query.Where("request_count <= ?", requestCountMax)
	}

	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		if userID, parseErr := strconv.Atoi(keyword); parseErr == nil {
			query = query.Where("id = ? OR username LIKE ? OR display_name LIKE ?", userID, like, like)
		} else {
			query = query.Where("username LIKE ? OR display_name LIKE ?", like, like)
		}
	}

	// 统计总数(不需要 JOIN)
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 判断是否需要 JOIN 外部表进行排序
	needTopupSort := sortFields["topup_quota"] == "asc" || sortFields["topup_quota"] == "desc"
	needWelfareSort := sortFields["welfare_quota"] == "asc" || sortFields["welfare_quota"] == "desc"

	if needTopupSort {
		// LEFT JOIN logs 表统计每个用户充值总额
		topupSub := LOG_DB.Model(&Log{}).
			Select("user_id, SUM(quota) as topup_total").
			Where(logTypeCol+" = ?", LogTypeTopup).
			Group("user_id")
		query = query.Joins("LEFT JOIN (?) lt ON lt.user_id = users.id", topupSub)
	}

	if needWelfareSort {
		// LEFT JOIN redemptions 表统计每个用户兑换码充值总额
		redemptionSub := DB.Model(&Redemption{}).
			Select("used_user_id, SUM(quota) as redemption_total").
			Where("status = ?", common.RedemptionCodeStatusUsed).
			Group("used_user_id")
		query = query.Joins("LEFT JOIN (?) lr ON lr.used_user_id = users.id", redemptionSub)
	}

	// 构建动态 ORDER BY
	var orderParts []string
	dbFieldMap := map[string]string{
		"quota":         "users.quota",
		"request_count": "users.request_count",
		"used_quota":    "users.used_quota",
		"created_at":    "users.created_at",
	}
	for field, col := range dbFieldMap {
		if order, ok := sortFields[field]; ok && (order == "asc" || order == "desc") {
			orderParts = append(orderParts, col+" "+order)
		}
	}
	if needTopupSort {
		orderParts = append(orderParts, "COALESCE(lt.topup_total, 0) "+sortFields["topup_quota"])
	}
	if needWelfareSort {
		orderParts = append(orderParts, "(users.aff_history + COALESCE(lr.redemption_total, 0)) "+sortFields["welfare_quota"])
	}
	orderParts = append(orderParts, "users.id desc")

	// 分页查询
	err = query.Omit("password").Order(strings.Join(orderParts, ", ")).Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetUsersWelfareQuota 批量查询用户福利金额(兑换码充值 + 邀请奖励)
func GetUsersWelfareQuota(userIds []int) (map[int]int64, error) {
	if len(userIds) == 0 {
		return map[int]int64{}, nil
	}
	// 邀请奖励: 直接从 User.AffHistoryQuota 获取
	type affResult struct {
		Id              int   `json:"id"`
		AffHistoryQuota int64 `json:"aff_history_quota"`
	}
	var affResults []affResult
	err := DB.Unscoped().Model(&User{}).
		Select("id, aff_history as aff_history_quota").
		Where("id IN ?", userIds).
		Scan(&affResults).Error
	if err != nil {
		return nil, err
	}

	// 兑换码充值: 从 redemptions 表查询
	redemptionMap, err := GetUsersRedemptionQuota(userIds)
	if err != nil {
		return nil, err
	}

	// 合并: 福利 = 邀请奖励 + 兑换码充值
	m := make(map[int]int64, len(userIds))
	for _, r := range affResults {
		m[r.Id] = r.AffHistoryQuota + redemptionMap[r.Id]
	}
	return m, nil
}

func SearchUsers(keyword string, group string, startIdx int, num int) ([]*User, int64, error) {
	var users []*User
	var total int64
	var err error

	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 构建基础查询
	query := tx.Unscoped().Model(&User{})

	// 构建搜索条件
	likeCondition := "username LIKE ? OR email LIKE ? OR display_name LIKE ?"

	// 尝试将关键字转换为整数ID
	keywordInt, err := strconv.Atoi(keyword)
	if err == nil {
		// 如果是数字，同时搜索ID和其他字段
		likeCondition = "id = ? OR " + likeCondition
		if group != "" {
			query = query.Where("("+likeCondition+") AND "+commonGroupCol+" = ?",
				keywordInt, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", group)
		} else {
			query = query.Where(likeCondition,
				keywordInt, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
	} else {
		// 非数字关键字，只搜索字符串字段
		if group != "" {
			query = query.Where("("+likeCondition+") AND "+commonGroupCol+" = ?",
				"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", group)
		} else {
			query = query.Where(likeCondition,
				"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
	}

	// 获取总数
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = query.Omit("password").Order("id desc").Limit(num).Offset(startIdx).Find(&users).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func GetUserById(id int, selectAll bool) (*User, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	user := User{Id: id}
	var err error = nil
	if selectAll {
		err = DB.First(&user, "id = ?", id).Error
	} else {
		err = DB.Omit("password").First(&user, "id = ?", id).Error
	}
	return &user, err
}

func GetUserIdByAffCode(affCode string) (int, error) {
	if affCode == "" {
		return 0, errors.New("affCode 为空！")
	}
	var user User
	err := DB.Select("id").First(&user, "aff_code = ?", affCode).Error
	return user.Id, err
}

func GetUserIdByAffCodeInProvider(providerId int, affCode string) (int, error) {
	if affCode == "" {
		return 0, errors.New("affCode 涓虹┖锛?")
	}
	var user User
	query := DB.Select("id").Where("aff_code = ?", affCode)
	if providerId > 0 {
		query = query.Where("provider_id = ?", providerId)
	}
	err := query.First(&user).Error
	return user.Id, err
}

func DeleteUserById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	user := User{Id: id}
	return user.Delete()
}

func HardDeleteUserById(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	err := DB.Unscoped().Delete(&User{}, "id = ?", id).Error
	return err
}

func inviteUser(inviterId int) (err error) {
	user, err := GetUserById(inviterId, true)
	if err != nil {
		return err
	}
	user.AffCount++
	user.AffQuota += common.QuotaForInviter
	user.AffHistoryQuota += common.QuotaForInviter
	return DB.Save(user).Error
}

func recordRewardAndIncreaseQuotaTx(tx *gorm.DB, providerId int, userId int, quota int, sourceType string, sourceId int, description string) error {
	if tx == nil || userId <= 0 || quota <= 0 || sourceType == "" {
		return nil
	}
	if err := tx.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
		"quota":        gorm.Expr("quota + ?", quota),
		"reward_quota": gorm.Expr("reward_quota + ?", quota),
	}).Error; err != nil {
		return err
	}
	if providerId <= 0 {
		return nil
	}
	return CreateRewardRecordTx(tx, &RewardRecord{
		ProviderId:  providerId,
		UserId:      userId,
		SourceType:  sourceType,
		SourceId:    sourceId,
		Quota:       quota,
		Description: description,
	})
}

func grantInviterRewardTx(tx *gorm.DB, inviterId int, inviteeId int, rewardQuota int) error {
	if tx == nil || inviterId <= 0 || inviteeId <= 0 {
		return nil
	}
	var inviter User
	if err := tx.Select("id", "provider_id").Where("id = ?", inviterId).Take(&inviter).Error; err != nil {
		return err
	}
	updates := map[string]interface{}{
		"aff_count": gorm.Expr("aff_count + ?", 1),
	}
	if rewardQuota > 0 {
		updates["aff_quota"] = gorm.Expr("aff_quota + ?", rewardQuota)
		updates["aff_history"] = gorm.Expr("aff_history + ?", rewardQuota)
	}
	//上级用户记录邀请人数，邀请历史（如果有的话），邀请额度 （如果有的话）
	if err := tx.Model(&User{}).Where("id = ?", inviterId).Updates(updates).Error; err != nil {
		return err
	}
	//有邀请奖励时在进行奖励发放。避免有邀请人没设置邀请奖励不记录邀请人数
	if rewardQuota <= 0 || inviter.ProviderId <= 0 {
		return nil
	}
	return CreateRewardRecordTx(tx, &RewardRecord{
		ProviderId:  inviter.ProviderId,
		UserId:      inviterId,
		SourceType:  "inviter_reward",
		SourceId:    inviteeId,
		Quota:       rewardQuota,
		Description: "inviter reward",
	})
}

func (user *User) TransferAffQuotaToQuota(quota int) error {
	// 检查quota是否小于最小额度
	if float64(quota) < common.QuotaPerUnit {
		return fmt.Errorf("转移额度最小为%s！", logger.LogQuota(int(common.QuotaPerUnit)))
	}

	// 开始数据库事务
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback() // 确保在函数退出时事务能回滚

	// 加锁查询用户以确保数据一致性
	err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, user.Id).Error
	if err != nil {
		return err
	}

	// 再次检查用户的AffQuota是否足够
	if user.AffQuota < quota {
		return errors.New("邀请额度不足！")
	}

	// 更新用户额度
	user.AffQuota -= quota
	user.Quota += quota
	user.RewardQuota += quota

	// 保存用户状态
	if err := tx.Save(user).Error; err != nil {
		return err
	}

	// 提交事务
	return tx.Commit().Error
}

func (user *User) Insert(inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	rewardCfg, err := GetProviderRewardConfig(user.ProviderId)
	if err != nil {
		return err
	}
	user.Quota = rewardCfg.QuotaForNewUser
	user.RewardQuota = rewardCfg.QuotaForNewUser
	user.InviterId = inviterId
	//user.SetAccessToken(common.GetUUID())
	user.AffCode = common.GetRandomString(4)

	// 初始化用户设置，包括默认的边栏配置
	if user.Setting == "" {
		defaultSetting := dto.UserSetting{}
		// 这里暂时不设置SidebarModules，因为需要在用户创建后根据角色设置
		user.SetSetting(defaultSetting)
	}
	user.CreatedAt = time.Now().Unix()
	tx := DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	result := tx.Create(user)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if err := createInviteRecordTx(tx, inviterId, user); err != nil {
		tx.Rollback()
		return err
	}
	if rewardCfg.QuotaForNewUser > 0 && user.ProviderId > 0 {
		if err := CreateRewardRecordTx(tx, &RewardRecord{
			ProviderId:  user.ProviderId,
			UserId:      user.Id,
			SourceType:  "new_user",
			SourceId:    user.Id,
			Quota:       rewardCfg.QuotaForNewUser,
			Description: "new user reward",
		}); err != nil {
			tx.Rollback()
			return err
		}
	}
	if inviterId != 0 {
		if rewardCfg.QuotaForInvitee > 0 {
			if err := recordRewardAndIncreaseQuotaTx(tx, user.ProviderId, user.Id, rewardCfg.QuotaForInvitee, "invitee_reward", inviterId, "invitee reward"); err != nil {
				tx.Rollback()
				return err
			}
		}
		var inviter User
		if err := tx.Select("id", "provider_id").Where("id = ?", inviterId).Take(&inviter).Error; err != nil {
			tx.Rollback()
			return err
		}
		inviterRewardCfg, err := GetProviderRewardConfig(inviter.ProviderId)
		if err != nil {
			tx.Rollback()
			return err
		}
		if err := grantInviterRewardTx(tx, inviterId, user.Id, inviterRewardCfg.QuotaForInviter); err != nil {
			tx.Rollback()
			return err
		}
	}

	// 用户创建成功后，根据角色初始化边栏配置
	// 需要重新获取用户以确保有正确的ID和Role
	var createdUser User
	if err := tx.Where("id = ?", user.Id).First(&createdUser).Error; err == nil {
		//查询cli_user有没有id
		clis, err := GetCliUserByCon(&CliUser{
			UserId: strconv.Itoa(createdUser.Id),
		})
		if err != nil {
			tx.Rollback()
			return err
		}
		if len(clis) != 0 {
			tx.Rollback()
			return errors.New("cli user already exists ")
		}
		_, err = InsertNewCliUser(strconv.Itoa(createdUser.Id), tx)
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit().Error; err != nil {
			return err
		}
		// 生成基于角色的默认边栏配置
		defaultSidebarConfig := generateDefaultSidebarConfigForRole(createdUser.Role)
		if defaultSidebarConfig != "" {
			currentSetting := createdUser.GetSetting()
			currentSetting.SidebarModules = defaultSidebarConfig
			createdUser.SetSetting(currentSetting)
			createdUser.Update(false)
			common.SysLog(fmt.Sprintf("为新用户 %s (角色: %d) 初始化边栏配置", createdUser.Username, createdUser.Role))
		}
	}

	if rewardCfg.QuotaForNewUser > 0 {
		RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("new user reward %s", logger.LogQuota(rewardCfg.QuotaForNewUser)))
	}
	if inviterId != 0 {
		if rewardCfg.QuotaForInvitee > 0 {
			RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("invitee reward %s", logger.LogQuota(rewardCfg.QuotaForInvitee)))
		}
		var inviter User
		if err := DB.Select("id", "provider_id").Where("id = ?", inviterId).Take(&inviter).Error; err == nil {
			if inviterRewardCfg, err := GetProviderRewardConfig(inviter.ProviderId); err == nil && inviterRewardCfg.QuotaForInviter > 0 {
				RecordLog(inviterId, LogTypeSystem, fmt.Sprintf("inviter reward %s", logger.LogQuota(inviterRewardCfg.QuotaForInviter)))
			}
		}
	}
	return nil
}

// InsertWithTx inserts a new user within an existing transaction.
// This is used for OAuth registration where user creation and binding need to be atomic.
// Post-creation tasks (sidebar config, logs, inviter rewards) are handled after the transaction commits.
func (user *User) InsertWithTx(tx *gorm.DB, inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	rewardCfg, err := GetProviderRewardConfig(user.ProviderId)
	if err != nil {
		return err
	}
	user.Quota = rewardCfg.QuotaForNewUser
	user.RewardQuota = rewardCfg.QuotaForNewUser
	user.InviterId = inviterId
	user.AffCode = common.GetRandomString(4)

	// 初始化用户设置
	if user.Setting == "" {
		defaultSetting := dto.UserSetting{}
		user.SetSetting(defaultSetting)
	}
	user.CreatedAt = time.Now().Unix()

	result := tx.Create(user)
	if result.Error != nil {
		return result.Error
	}
	if err := createInviteRecordTx(tx, inviterId, user); err != nil {
		return err
	}
	if rewardCfg.QuotaForNewUser > 0 && user.ProviderId > 0 {
		if err := CreateRewardRecordTx(tx, &RewardRecord{
			ProviderId:  user.ProviderId,
			UserId:      user.Id,
			SourceType:  "new_user",
			SourceId:    user.Id,
			Quota:       rewardCfg.QuotaForNewUser,
			Description: "new user reward",
		}); err != nil {
			return err
		}
	}
	if inviterId != 0 {
		if rewardCfg.QuotaForInvitee > 0 {
			if err := recordRewardAndIncreaseQuotaTx(tx, user.ProviderId, user.Id, rewardCfg.QuotaForInvitee, "invitee_reward", inviterId, "invitee reward"); err != nil {
				return err
			}
		}
		var inviter User
		if err := tx.Select("id", "provider_id").Where("id = ?", inviterId).Take(&inviter).Error; err != nil {
			return err
		}
		inviterRewardCfg, err := GetProviderRewardConfig(inviter.ProviderId)
		if err != nil {
			return err
		}
		if err := grantInviterRewardTx(tx, inviterId, user.Id, inviterRewardCfg.QuotaForInviter); err != nil {
			return err
		}
	}

	return nil
}

// FinalizeOAuthUserCreation performs post-transaction tasks for OAuth user creation.
// This should be called after the transaction commits successfully.
func (user *User) FinalizeOAuthUserCreation(inviterId int) {
	// 鐢ㄦ埛鍒涘缓鎴愬姛鍚庯紝鏍规嵁瑙掕壊鍒濆鍖栬竟鏍忛厤缃�
	var createdUser User
	if err := DB.Where("id = ?", user.Id).First(&createdUser).Error; err == nil {
		defaultSidebarConfig := generateDefaultSidebarConfigForRole(createdUser.Role)
		if defaultSidebarConfig != "" {
			currentSetting := createdUser.GetSetting()
			currentSetting.SidebarModules = defaultSidebarConfig
			createdUser.SetSetting(currentSetting)
			createdUser.Update(false)
			common.SysLog(fmt.Sprintf("created provider user %s (role: %d) sidebar initialized", createdUser.Username, createdUser.Role))
		}
	}

	if rewardCfg, err := GetProviderRewardConfig(user.ProviderId); err == nil {
		if rewardCfg.QuotaForNewUser > 0 {
			RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("new user reward %s", logger.LogQuota(rewardCfg.QuotaForNewUser)))
		}
		if inviterId != 0 {
			if rewardCfg.QuotaForInvitee > 0 {
				RecordLog(user.Id, LogTypeSystem, fmt.Sprintf("invitee reward %s", logger.LogQuota(rewardCfg.QuotaForInvitee)))
			}
			var inviter User
			if err := DB.Select("id", "provider_id").Where("id = ?", inviterId).Take(&inviter).Error; err == nil {
				if inviterRewardCfg, err := GetProviderRewardConfig(inviter.ProviderId); err == nil && inviterRewardCfg.QuotaForInviter > 0 {
					RecordLog(inviterId, LogTypeSystem, fmt.Sprintf("inviter reward %s", logger.LogQuota(inviterRewardCfg.QuotaForInviter)))
				}
			}
		}
	}
}

func (user *User) Update(updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	newUser := *user
	DB.First(&user, user.Id)
	if err = DB.Model(user).Updates(newUser).Error; err != nil {
		return err
	}

	// Update cache
	return updateUserCache(*user)
}

// UpdateUserProfile 更新用户个人资料字段（支持空字符串以实现清空效果）
func UpdateUserProfile(userId int, updates map[string]interface{}) error {
	if userId == 0 {
		return errors.New("user id is empty")
	}
	if len(updates) == 0 {
		return nil
	}
	var user User
	if err := DB.First(&user, userId).Error; err != nil {
		return err
	}
	if err := DB.Model(&user).Updates(updates).Error; err != nil {
		return err
	}
	if err := DB.First(&user, userId).Error; err != nil {
		return err
	}
	return updateUserCache(user)
}

func (user *User) Edit(updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}

	newUser := *user
	updates := map[string]interface{}{
		"username":                      newUser.Username,
		"display_name":                  newUser.DisplayName,
		"group":                         newUser.Group,
		"invite_consume_rebate_enabled": newUser.InviteConsumeRebateEnabled,
		"remark":                        newUser.Remark,
	}
	if updatePassword {
		updates["password"] = newUser.Password
	}

	DB.First(&user, user.Id)
	if err = DB.Model(user).Updates(updates).Error; err != nil {
		return err
	}

	// Update cache
	return updateUserCache(*user)
}

func (user *User) ClearBinding(bindingType string) error {
	if user.Id == 0 {
		return errors.New("user id is empty")
	}

	bindingColumnMap := map[string]string{
		"email":    "email",
		"github":   "github_id",
		"discord":  "discord_id",
		"oidc":     "oidc_id",
		"wechat":   "wechat_id",
		"telegram": "telegram_id",
		"linuxdo":  "linux_do_id",
	}

	column, ok := bindingColumnMap[bindingType]
	if !ok {
		return errors.New("invalid binding type")
	}

	if err := DB.Model(&User{}).Where("id = ?", user.Id).Update(column, "").Error; err != nil {
		return err
	}

	if err := DB.Where("id = ?", user.Id).First(user).Error; err != nil {
		return err
	}

	return updateUserCache(*user)
}

func (user *User) Delete() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	if err := DB.Delete(user).Error; err != nil {
		return err
	}

	// 清除缓存
	return invalidateUserCache(user.Id)
}

func (user *User) HardDelete() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	err := DB.Unscoped().Delete(user).Error
	return err
}

// ValidateAndFill check password & user status
func (user *User) ValidateAndFill() (err error) {
	return user.ValidateAndFillInProvider(0)
}

func (user *User) ValidateAndFillInProvider(providerId int) (err error) {
	// When querying with struct, GORM will only query with non-zero fields,
	// that means if your field's value is 0, '', false or other zero values,
	// it won't be used to build query conditions
	password := user.Password
	username := strings.TrimSpace(user.Username)
	if username == "" || password == "" {
		return ErrUserEmptyCredentials
	}
	if providerId > 0 {
		matches := make(map[int]User, 2)
		var providerUsers []User
		if err = DB.Where("provider_id = ? AND (username = ? OR email = ?)", providerId, username, username).Find(&providerUsers).Error; err != nil {
			return fmt.Errorf("%w: %v", ErrDatabase, err)
		}
		for _, matchedUser := range providerUsers {
			matches[matchedUser.Id] = matchedUser
		}
		ownerUserId, ownerErr := providerOwnerUserId(providerId)
		if ownerErr != nil {
			return fmt.Errorf("%w: %v", ErrDatabase, ownerErr)
		}
		if ownerUserId > 0 {
			var owner User
			ownerErr = DB.Where("id = ? AND (username = ? OR email = ?)", ownerUserId, username, username).First(&owner).Error
			if ownerErr != nil && !errors.Is(ownerErr, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %v", ErrDatabase, ownerErr)
			}
			if ownerErr == nil {
				matches[owner.Id] = owner
			}
		}
		if len(matches) == 0 {
			return ErrInvalidCredentials
		}
		if len(matches) > 1 {
			return ErrProviderLoginConflict
		}
		for _, matchedUser := range matches {
			*user = matchedUser
		}
		okay := common.ValidatePasswordAndHash(password, user.Password)
		if !okay || user.Status != common.UserStatusEnabled {
			return ErrInvalidCredentials
		}
		return nil
	}
	// find by username or email
	err = DB.Where("provider_id = ? AND (username = ? OR email = ?)", providerId, username, username).First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	okay := common.ValidatePasswordAndHash(password, user.Password)
	if !okay || user.Status != common.UserStatusEnabled {
		return ErrInvalidCredentials
	}
	return nil
}

func (user *User) FillUserById() error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	DB.Where(User{Id: user.Id}).First(user)
	return nil
}

func (user *User) FillUserByEmail() error {
	if user.Email == "" {
		return errors.New("email 为空！")
	}
	DB.Where(User{Email: user.Email}).First(user)
	return nil
}

func (user *User) FillUserByGitHubId() error {
	if user.GitHubId == "" {
		return errors.New("GitHub id 为空！")
	}
	DB.Where(User{GitHubId: user.GitHubId}).First(user)
	return nil
}

// UpdateGitHubId updates the user's GitHub ID (used for migration from login to numeric ID)
func (user *User) UpdateGitHubId(newGitHubId string) error {
	if user.Id == 0 {
		return errors.New("user id is empty")
	}
	return DB.Model(user).Update("github_id", newGitHubId).Error
}

func (user *User) FillUserByDiscordId() error {
	if user.DiscordId == "" {
		return errors.New("discord id 为空！")
	}
	DB.Where(User{DiscordId: user.DiscordId}).First(user)
	return nil
}

func (user *User) FillUserByOidcId() error {
	if user.OidcId == "" {
		return errors.New("oidc id 为空！")
	}
	DB.Where(User{OidcId: user.OidcId}).First(user)
	return nil
}

func (user *User) FillUserByWeChatId() error {
	if user.WeChatId == "" {
		return errors.New("WeChat id 为空！")
	}
	DB.Where(User{WeChatId: user.WeChatId}).First(user)
	return nil
}

func (user *User) FillUserByTelegramId() error {
	if user.TelegramId == "" {
		return errors.New("Telegram id 为空！")
	}
	err := DB.Where(User{TelegramId: user.TelegramId}).First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("该 Telegram 账户未绑定")
	}
	return nil
}

func IsWeChatIdAlreadyTaken(wechatId string) bool {
	return DB.Unscoped().Where("wechat_id = ?", wechatId).Find(&User{}).RowsAffected == 1
}

func IsGitHubIdAlreadyTaken(githubId string) bool {
	return DB.Unscoped().Where("github_id = ?", githubId).Find(&User{}).RowsAffected == 1
}

func IsDiscordIdAlreadyTaken(discordId string) bool {
	return DB.Unscoped().Where("discord_id = ?", discordId).Find(&User{}).RowsAffected == 1
}

func IsOidcIdAlreadyTaken(oidcId string) bool {
	return DB.Where("oidc_id = ?", oidcId).Find(&User{}).RowsAffected == 1
}

func IsTelegramIdAlreadyTaken(telegramId string) bool {
	return DB.Unscoped().Where("telegram_id = ?", telegramId).Find(&User{}).RowsAffected == 1
}

func IsEmailAlreadyTaken(email string) bool {
	return IsEmailAlreadyTakenInProvider(0, email)
}

func IsEmailAlreadyTakenInProvider(providerId int, email string) bool {
	return DB.Unscoped().Where("provider_id = ? AND email = ?", providerId, email).Find(&User{}).RowsAffected == 1
}

func ResetUserPasswordByEmail(email string, password string) error {
	return ResetUserPasswordByEmailInProvider(0, email, password)
}

func ResetUserPasswordByEmailInProvider(providerId int, email string, password string) error {
	if email == "" || password == "" {
		return errors.New("邮箱地址或密码为空！")
	}
	hashedPassword, err := common.Password2Hash(password)
	if err != nil {
		return err
	}
	err = DB.Model(&User{}).Where("provider_id = ? AND email = ?", providerId, email).Update("password", hashedPassword).Error
	return err
}

func IsAdmin(userId int) bool {
	if userId == 0 {
		return false
	}
	var user User
	err := DB.Where("id = ?", userId).Select("role").Find(&user).Error
	if err != nil {
		common.SysLog("no such user " + err.Error())
		return false
	}
	return user.Role >= common.RoleAdminUser
}

//// IsUserEnabled checks user status from Redis first, falls back to DB if needed
//func IsUserEnabled(id int, fromDB bool) (status bool, err error) {
//	defer func() {
//		// Update Redis cache asynchronously on successful DB read
//		if shouldUpdateRedis(fromDB, err) {
//			gopool.Go(func() {
//				if err := updateUserStatusCache(id, status); err != nil {
//					common.SysError("failed to update user status cache: " + err.Error())
//				}
//			})
//		}
//	}()
//	if !fromDB && common.RedisEnabled {
//		// Try Redis first
//		status, err := getUserStatusCache(id)
//		if err == nil {
//			return status == common.UserStatusEnabled, nil
//		}
//		// Don't return error - fall through to DB
//	}
//	fromDB = true
//	var user User
//	err = DB.Where("id = ?", id).Select("status").Find(&user).Error
//	if err != nil {
//		return false, err
//	}
//
//	return user.Status == common.UserStatusEnabled, nil
//}

func ValidateAccessToken(token string) (*User, error) {
	return ValidateAccessTokenInProvider(token, 0)
}

func ValidateAccessTokenInProvider(token string, providerId int) (*User, error) {
	if token == "" {
		return nil, nil
	}
	token = strings.Replace(token, "Bearer ", "", 1)
	user := &User{}
	err := DB.Where("provider_id = ? AND access_token = ?", providerId, token).First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return user, nil
}

// GetUserQuota gets quota from Redis first, falls back to DB if needed
func GetUserQuota(id int, fromDB bool) (quota int, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserQuotaCache(id, quota); err != nil {
					common.SysLog("failed to update user quota cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		quota, err := getUserQuotaCache(id)
		if err == nil {
			return quota, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Model(&User{}).Where("id = ?", id).Select("quota").Find(&quota).Error
	if err != nil {
		return 0, err
	}

	return quota, nil
}

func GetUserUsedQuota(id int) (quota int, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("used_quota").Find(&quota).Error
	return quota, err
}

func GetUserEmail(id int) (email string, err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Select("email").Find(&email).Error
	return email, err
}

// GetUserGroup gets group from Redis first, falls back to DB if needed
func GetUserGroup(id int, fromDB bool) (group string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserGroupCache(id, group); err != nil {
					common.SysLog("failed to update user group cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		group, err := getUserGroupCache(id)
		if err == nil {
			return group, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	// group 是 SQL 保留字，查询时必须按数据库类型转义列名。
	// commonGroupCol 通常在数据库初始化时设置；为空时按当前数据库类型兜底，避免测试或早期初始化阶段查询失败。
	groupCol := commonGroupCol
	if groupCol == "" {
		groupCol = "`group`"
		if common.UsingPostgreSQL {
			groupCol = `"group"`
		}
	}
	err = DB.Model(&User{}).Where("id = ?", id).Select(groupCol).Find(&group).Error
	if err != nil {
		return "", err
	}

	return group, nil
}

// GetUserSetting gets setting from Redis first, falls back to DB if needed
func GetUserSetting(id int, fromDB bool) (settingMap dto.UserSetting, err error) {
	var setting string
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserSettingCache(id, setting); err != nil {
					common.SysLog("failed to update user setting cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		setting, err := getUserSettingCache(id)
		if err == nil {
			return setting, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	// can be nil setting
	var safeSetting sql.NullString
	err = DB.Model(&User{}).Where("id = ?", id).Select("setting").Find(&safeSetting).Error
	if err != nil {
		return settingMap, err
	}
	if safeSetting.Valid {
		setting = safeSetting.String
	} else {
		setting = ""
	}
	userBase := &UserBase{
		Setting: setting,
	}
	return userBase.GetSetting(), nil
}

func IncreaseUserQuota(id int, quota int, db bool) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheIncrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to increase user quota: " + err.Error())
		}
	})
	if !db && common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, quota)
		return nil
	}
	return increaseUserQuota(id, quota)
}

func increaseUserQuota(id int, quota int) (err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota + ?", quota)).Error
	if err != nil {
		return err
	}
	return err
}

// 奖励入账时同时增加 quota 和 reward_quota
func IncreaseUserRewardQuota(id int, quota int, db bool) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheIncrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to increase user quota: " + err.Error())
		}
	})
	if !db && common.BatchUpdateEnabled {
		// reward_quota must stay in sync with quota, so do not use the legacy
		// quota-only batch path for reward income.
		db = true
	}
	return increaseUserRewardQuota(id, quota)
}

func increaseUserRewardQuota(id int, quota int) error {
	return DB.Model(&User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"quota":        gorm.Expr("quota + ?", quota),
		"reward_quota": gorm.Expr("reward_quota + ?", quota),
	}).Error
}

func DecreaseUserQuota(id int, quota int, db bool) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	gopool.Go(func() {
		err := cacheDecrUserQuota(id, int64(quota))
		if err != nil {
			common.SysLog("failed to decrease user quota: " + err.Error())
		}
	})
	if !db && common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserQuota, id, -quota)
		return nil
	}
	return decreaseUserQuota(id, quota)
}

func decreaseUserQuota(id int, quota int) (err error) {
	err = DB.Model(&User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota - ?", quota)).Error
	if err != nil {
		return err
	}
	return err
}

type UserQuotaBreakdown struct {
	Total      int
	RewardUsed int
	PaidUsed   int
}

// 钱包消费时先扣 reward_quota，不够再扣充值余额。
func DecreaseUserQuotaPreferReward(id int, quota int) (UserQuotaBreakdown, error) {
	breakdown := UserQuotaBreakdown{Total: quota}
	if quota < 0 {
		return breakdown, errors.New("quota 不能为负数！")
	}
	if quota == 0 {
		return breakdown, nil
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Select("id", "quota", "reward_quota").
			Where("id = ?", id).
			First(&user).Error; err != nil {
			return err
		}
		if user.Quota < quota {
			return fmt.Errorf("user quota is not enough, user quota: %s, need quota: %s", logger.FormatQuota(user.Quota), logger.FormatQuota(quota))
		}
		rewardAvailable := user.RewardQuota
		if rewardAvailable > user.Quota {
			rewardAvailable = user.Quota
		}
		if rewardAvailable < 0 {
			rewardAvailable = 0
		}
		breakdown.RewardUsed = rewardAvailable
		if breakdown.RewardUsed > quota {
			breakdown.RewardUsed = quota
		}
		breakdown.PaidUsed = quota - breakdown.RewardUsed

		return tx.Model(&User{}).Where("id = ?", id).Updates(map[string]interface{}{
			"quota":        gorm.Expr("quota - ?", quota),
			"reward_quota": gorm.Expr("reward_quota - ?", breakdown.RewardUsed),
		}).Error
	})
	if err != nil {
		return breakdown, err
	}
	gopool.Go(func() {
		if err := cacheDecrUserQuota(id, int64(quota)); err != nil {
			common.SysLog("failed to decrease user quota: " + err.Error())
		}
	})
	return breakdown, nil
}

// 退款时按原本扣费来源退回，奖励部分退回 reward_quota
func IncreaseUserQuotaByBreakdown(id int, total int, reward int) error {
	if total < 0 || reward < 0 {
		return errors.New("quota 不能为负数！")
	}
	if reward > total {
		return errors.New("reward quota cannot exceed total quota")
	}
	if total == 0 {
		return nil
	}
	if err := DB.Model(&User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"quota":        gorm.Expr("quota + ?", total),
		"reward_quota": gorm.Expr("reward_quota + ?", reward),
	}).Error; err != nil {
		return err
	}
	gopool.Go(func() {
		if err := cacheIncrUserQuota(id, int64(total)); err != nil {
			common.SysLog("failed to increase user quota: " + err.Error())
		}
	})
	return nil
}

func DeltaUpdateUserQuota(id int, delta int) (err error) {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return IncreaseUserQuota(id, delta, false)
	} else {
		return DecreaseUserQuota(id, -delta, false)
	}
}

//func GetRootUserEmail() (email string) {
//	DB.Model(&User{}).Where("role = ?", common.RoleRootUser).Select("email").Find(&email)
//	return email
//}

func GetRootUser() (user *User) {
	DB.Where("role = ?", common.RoleRootUser).First(&user)
	return user
}

func UpdateUserUsedQuotaAndRequestCount(id int, quota int) {
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUsedQuota, id, quota)
		addNewRecord(BatchUpdateTypeRequestCount, id, 1)
		return
	}
	updateUserUsedQuotaAndRequestCount(id, quota, 1)
}

// 单独更新请求次数
func UpdateUserRequestCount(id int, count int) {
	if count == 0 {
		return
	}
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeRequestCount, id, count)
		return
	}
	updateUserRequestCount(id, count)
}

func updateUserUsedQuotaAndRequestCount(id int, quota int, count int) {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", count),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota and request count: " + err.Error())
		return
	}

	//// 更新缓存
	//if err := invalidateUserCache(id); err != nil {
	//	common.SysError("failed to invalidate user cache: " + err.Error())
	//}
}

func updateUserUsedQuota(id int, quota int) {
	err := DB.Model(&User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota": gorm.Expr("used_quota + ?", quota),
		},
	).Error
	if err != nil {
		common.SysLog("failed to update user used quota: " + err.Error())
	}
}

func UpdateUserAndTokenTotalTokenUsed(userId int, tokenId int, tokenUsed int) {
	if tokenUsed <= 0 {
		return
	}
	UpdateUserTotalTokenUsed(userId, tokenUsed)
	if tokenId > 0 {
		UpdateTokenTotalTokenUsed(tokenId, tokenUsed)
	}
}

func UpdateUserTotalTokenUsed(id int, tokenUsed int) {
	if id <= 0 || tokenUsed <= 0 {
		return
	}
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeUserTotalTokenUsed, id, tokenUsed)
		return
	}
	updateUserTotalTokenUsed(id, tokenUsed)
}

func updateUserTotalTokenUsed(id int, tokenUsed int) {
	err := DB.Model(&User{}).Where("id = ?", id).Update("total_token_used", gorm.Expr("total_token_used + ?", tokenUsed)).Error
	if err != nil {
		common.SysLog("failed to update user total token used: " + err.Error())
	}
}

func updateUserRequestCount(id int, count int) {
	err := DB.Model(&User{}).Where("id = ?", id).Update("request_count", gorm.Expr("request_count + ?", count)).Error
	if err != nil {
		common.SysLog("failed to update user request count: " + err.Error())
	}
}

// GetUsernameById gets username from Redis first, falls back to DB if needed
func GetUsernameById(id int, fromDB bool) (username string, err error) {
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) {
			gopool.Go(func() {
				if err := updateUserNameCache(id, username); err != nil {
					common.SysLog("failed to update user name cache: " + err.Error())
				}
			})
		}
	}()
	if !fromDB && common.RedisEnabled {
		username, err := getUserNameCache(id)
		if err == nil {
			return username, nil
		}
		// Don't return error - fall through to DB
	}
	fromDB = true
	err = DB.Model(&User{}).Where("id = ?", id).Select("username").Find(&username).Error
	if err != nil {
		return "", err
	}

	return username, nil
}

func IsLinuxDOIdAlreadyTaken(linuxDOId string) bool {
	var user User
	err := DB.Unscoped().Where("linux_do_id = ?", linuxDOId).First(&user).Error
	return !errors.Is(err, gorm.ErrRecordNotFound)
}

func (user *User) FillUserByLinuxDOId() error {
	if user.LinuxDOId == "" {
		return errors.New("linux do id is empty")
	}
	err := DB.Where("linux_do_id = ?", user.LinuxDOId).First(user).Error
	return err
}

func RootUserExists() bool {
	var user User
	err := DB.Where("role = ?", common.RoleRootUser).First(&user).Error
	if err != nil {
		return false
	}
	return true
}

// CountTotalUsers 统计用户总数
func CountTotalUsers() (int64, error) {
	var count int64
	err := DB.Unscoped().Model(&User{}).Count(&count).Error
	return count, err
}

// CountNewUsersByTimeRange 统计指定时间范围内的新注册用户数
func CountNewUsersByTimeRange(startTimestamp, endTimestamp int64) (int64, error) {
	var count int64
	tx := DB.Unscoped().Model(&User{})
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at < ?", endTimestamp)
	}
	err := tx.Count(&count).Error
	return count, err
}

// CountActiveUsersByTimeRange 统计指定时间范围内的活跃用户数（token消耗>0的用户）
// 通过 logs 表中 type=LogTypeConsume 且 quota>0 的记录去重统计 user_id
func CountActiveUsersByTimeRange(startTimestamp, endTimestamp int64) (int64, error) {
	var count int64
	tx := LOG_DB.Model(&Log{}).
		Where("type = ? AND quota > 0", LogTypeConsume)
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at < ?", endTimestamp)
	}
	err := tx.Distinct("user_id").Count(&count).Error
	return count, err
}
