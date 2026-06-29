package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
)

// UserBase struct remains the same as it represents the cached data structure
type UserBase struct {
	Id         int    `json:"id"`
	ProviderId int    `json:"provider_id"`
	Group      string `json:"group"`
	Email      string `json:"email"`
	Quota      int    `json:"quota"`
	Status     int    `json:"status"`
	Username   string `json:"username"`
	Setting    string `json:"setting"`
}

// UserAccessTokenCache 是基于访问令牌（Access Token）的用户缓存结构体。
// 当用户通过 API 令牌（而非 Cookie 会话）发起请求时，系统会把该用户的关键信息
// 缓存到 Redis 中，避免每次请求都查询数据库，从而提升鉴权性能。
// 该结构体仅保留鉴权与路由分发所必需的字段，不含敏感信息（如密码）。
type UserAccessTokenCache struct {
	Id          int    `json:"id"`           // 用户唯一标识 ID
	ProviderId  int    `json:"provider_id"`  // 供应商（渠道）ID，用于区分不同上游供应商
	Username    string `json:"username"`     // 用户名，用于日志展示与身份标识
	DisplayName string `json:"display_name"` // 用户显示名称（昵称），用于前端展示
	Role        int    `json:"role"`         // 用户角色（如普通用户、管理员），用于权限校验
	Status      int    `json:"status"`       // 用户状态（如启用、禁用），禁用用户将被拒绝访问
	Group       string `json:"group"`        // 用户所属分组，决定可使用的模型与计费倍率
	Email       string `json:"email"`        // 用户邮箱，用于通知与身份标识
}

// ToUser 将访问令牌缓存对象转换为完整的 User 对象。
// 缓存中只保存了鉴权所需的部分字段，转换后其余字段保持零值，
// 调用方应仅依赖这些已填充的鉴权相关字段。
func (cache *UserAccessTokenCache) ToUser() *User {
	// 当缓存对象本身为 nil（即未命中或未初始化）时，直接返回 nil，避免空指针
	if cache == nil {
		return nil
	}
	// 将缓存中保存的字段逐一映射到 User 结构体并返回
	return &User{
		Id:          cache.Id,          // 用户 ID
		ProviderId:  cache.ProviderId,  // 供应商 ID
		Username:    cache.Username,    // 用户名
		DisplayName: cache.DisplayName, // 显示名称
		Role:        cache.Role,        // 角色
		Status:      cache.Status,      // 状态
		Group:       cache.Group,       // 分组
		Email:       cache.Email,       // 邮箱
	}
}

// newUserAccessTokenCache 根据完整的 User 对象构造一个访问令牌缓存对象。
// 用于在首次查询数据库命中用户后，将其鉴权相关字段提取并写入 Redis 缓存。
func newUserAccessTokenCache(user *User) *UserAccessTokenCache {
	// 当传入的用户对象为 nil 时，无法提取任何信息，直接返回 nil
	if user == nil {
		return nil
	}
	// 从 User 中提取需要缓存的字段，构造缓存对象并返回
	return &UserAccessTokenCache{
		Id:          user.Id,          // 用户 ID
		ProviderId:  user.ProviderId,  // 供应商 ID
		Username:    user.Username,    // 用户名
		DisplayName: user.DisplayName, // 显示名称
		Role:        user.Role,        // 角色
		Status:      user.Status,      // 状态
		Group:       user.Group,       // 分组
		Email:       user.Email,       // 邮箱
	}
}

func (user *UserBase) WriteContext(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyProviderId, user.ProviderId)
	common.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
	common.SetContextKey(c, constant.ContextKeyUserQuota, user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserStatus, user.Status)
	common.SetContextKey(c, constant.ContextKeyUserEmail, user.Email)
	common.SetContextKey(c, constant.ContextKeyUserName, user.Username)
	common.SetContextKey(c, constant.ContextKeyUserSetting, user.GetSetting())
}

func (user *UserBase) GetSetting() dto.UserSetting {
	setting := dto.UserSetting{}
	if user.Setting != "" {
		err := common.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

// getUserCacheKey returns the key for user cache
func getUserCacheKey(userId int) string {
	return fmt.Sprintf("user:%d", userId)
}

// normalizeUserAccessToken 对访问令牌进行标准化处理。
// 客户端可能在请求头中传入 "Bearer <token>" 形式或附带多余空白字符，
// 此函数统一去除这些干扰，确保同一令牌总能生成相同的缓存键。
func normalizeUserAccessToken(token string) string {
	// 第一步：去除令牌首尾的空白字符（如空格、制表符、换行符）
	token = strings.TrimSpace(token)
	// 第二步：去除可能存在的前缀 "Bearer "（仅去除首次出现，最多一次），兼容 Authorization 头格式
	token = strings.Replace(token, "Bearer ", "", 1)
	// 第三步：再次去除首尾空白，处理去掉前缀后可能残留的空格，返回标准化后的令牌
	return strings.TrimSpace(token)
}

// getUserAccessTokenCacheKey 根据供应商 ID 和访问令牌生成 Redis 缓存键。
// 为避免将原始令牌明文写入缓存键（存在泄露风险），最终键通过 HMAC 摘要处理，
// 同时保证同一 (providerId, token) 组合始终生成相同的键。
func getUserAccessTokenCacheKey(providerId int, token string) string {
	// 先对令牌进行标准化（去空白、去 Bearer 前缀），保证键的一致性
	token = normalizeUserAccessToken(token)
	// 标准化后若令牌为空字符串，说明调用方未提供有效令牌，返回空键供上层判断跳过
	if token == "" {
		return ""
	}
	// 将供应商 ID 与令牌拼接成原始键内容，用 ":" 分隔以保证唯一性
	rawKey := fmt.Sprintf("%d:%s", providerId, token)
	// 对原始键做 HMAC 摘要并加上统一前缀，得到最终的 Redis 缓存键
	return fmt.Sprintf("user_access_token:%s", common.GenerateHMAC(rawKey))
}

// invalidateUserCache clears user cache
func invalidateUserCache(userId int) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisDelKey(getUserCacheKey(userId))
}

// invalidateUserAccessTokenCache 使给定供应商下的一个或多个访问令牌缓存失效（删除）。
// 典型场景：用户登出、令牌被撤销、修改密码或状态变更后，需要清除旧缓存以防止旧令牌继续生效。
// 支持传入多个 token，会自动去重以避免重复删除同一个键。
func invalidateUserAccessTokenCache(providerId int, tokens ...string) error {
	// 若 Redis 未启用，则不存在需要清除的缓存，直接返回 nil（无错误）
	if !common.RedisEnabled {
		return nil
	}
	// 使用 map 记录已处理过的缓存键，用于去重，避免对相同键重复执行删除操作
	seen := make(map[string]struct{}, len(tokens))
	// 遍历调用方传入的每一个令牌（可变参数）
	for _, token := range tokens {
		// 根据供应商 ID 与令牌生成对应的 Redis 缓存键
		key := getUserAccessTokenCacheKey(providerId, token)
		// 若生成的键为空（说明令牌无效或为空），跳过当前令牌，处理下一个
		if key == "" {
			continue
		}
		// 若该键已在 seen 中存在（即本次已处理过），跳过以避免重复删除
		if _, ok := seen[key]; ok {
			continue
		}
		// 将当前键标记为已处理，防止后续相同令牌再次触发删除
		seen[key] = struct{}{}
		// 从 Redis 中删除该缓存键，使对应令牌的缓存失效；若删除失败立即返回错误
		if err := common.RedisDelKey(key); err != nil {
			return err
		}
	}
	// 所有令牌处理完毕且无错误，返回 nil
	return nil
}

// InvalidateUserCache is the exported version of invalidateUserCache.
// 供 controller 等上层包在用户状态变更（如禁用、删除、角色变更）后主动清理缓存。
func InvalidateUserCache(userId int) error {
	return invalidateUserCache(userId)
}

// updateUserCache updates all user cache fields using hash
func updateUserCache(user User) error {
	if !common.RedisEnabled {
		return nil
	}

	return common.RedisHSetObj(
		getUserCacheKey(user.Id),
		user.ToBaseUser(),
		time.Duration(common.RedisKeyCacheSeconds())*time.Second,
	)
}

// cacheSetUserAccessToken 将用户信息以访问令牌为键写入 Redis 缓存。
// 当首次通过数据库验证了某令牌对应的用户后调用此函数，使得后续相同令牌的请求
// 可直接命中缓存，无需再次查询数据库。
func cacheSetUserAccessToken(token string, user *User) error {
	// 前置条件：Redis 必须已启用，且用户对象不能为 nil；否则无需缓存，直接返回
	if !common.RedisEnabled || user == nil {
		return nil
	}
	// 根据用户的供应商 ID 与传入令牌，生成对应的 Redis 缓存键
	key := getUserAccessTokenCacheKey(user.ProviderId, token)
	// 若键为空（说明令牌无效），无法写入缓存，直接返回
	if key == "" {
		return nil
	}
	// 将用户对象转换为缓存结构体，并以 Hash 形式写入 Redis，同时设置过期时间
	return common.RedisHSetObj(
		key,                           // 缓存键
		newUserAccessTokenCache(user), // 缓存内容（提取后的用户字段）
		time.Duration(common.RedisKeyCacheSeconds())*time.Second, // 过期时长（秒），到期自动失效
	)
}

// cacheGetUserByAccessToken 通过访问令牌从 Redis 缓存中读取用户信息。
// 用于鉴权流程中的快速查询：若缓存命中则直接返回用户，未命中（返回错误）时
// 调用方应回退到数据库查询并重新写入缓存。
func cacheGetUserByAccessToken(providerId int, token string) (*User, error) {
	// 若 Redis 未启用，则无法从缓存读取，返回错误提示调用方回退到数据库查询
	if !common.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	// 根据供应商 ID 与令牌生成对应的 Redis 缓存键
	key := getUserAccessTokenCacheKey(providerId, token)
	// 若键为空（说明令牌为空或无效），返回错误提示令牌为空
	if key == "" {
		return nil, fmt.Errorf("access token is empty")
	}
	// 声明缓存结构体，用于接收从 Redis 反序列化出来的数据
	var userCache UserAccessTokenCache
	// 从 Redis 中按键读取 Hash 数据并反序列化到 userCache；若读取失败（含缓存不存在）则返回该错误
	if err := common.RedisHGetObj(key, &userCache); err != nil {
		return nil, err
	}
	// 将缓存对象转换为完整 User 对象返回，供上层鉴权使用
	return userCache.ToUser(), nil
}

// GetUserCache gets complete user cache from hash
func GetUserCache(userId int) (userCache *UserBase, err error) {
	var user *User
	var fromDB bool
	defer func() {
		// Update Redis cache asynchronously on successful DB read
		if shouldUpdateRedis(fromDB, err) && user != nil {
			gopool.Go(func() {
				if err := updateUserCache(*user); err != nil {
					common.SysLog("failed to update user status cache: " + err.Error())
				}
			})
		}
	}()

	// Try getting from Redis first
	userCache, err = cacheGetUserBase(userId)
	if err == nil {
		return userCache, nil
	}

	// If Redis fails, get from DB
	fromDB = true
	user, err = GetUserById(userId, false)
	if err != nil {
		return nil, err // Return nil and error if DB lookup fails
	}

	// Create cache object from user data
	userCache = &UserBase{
		Id:         user.Id,
		ProviderId: user.ProviderId,
		Group:      user.Group,
		Quota:      user.Quota,
		Status:     user.Status,
		Username:   user.Username,
		Setting:    user.Setting,
		Email:      user.Email,
	}

	return userCache, nil
}

func cacheGetUserBase(userId int) (*UserBase, error) {
	if !common.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	var userCache UserBase
	// Try getting from Redis first
	err := common.RedisHGetObj(getUserCacheKey(userId), &userCache)
	if err != nil {
		return nil, err
	}
	return &userCache, nil
}

// Add atomic quota operations using hash fields
func cacheIncrUserQuota(userId int, delta int64) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHIncrBy(getUserCacheKey(userId), "Quota", delta)
}

func cacheDecrUserQuota(userId int, delta int64) error {
	return cacheIncrUserQuota(userId, -delta)
}

// Helper functions to get individual fields if needed
func getUserGroupCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Group, nil
}

func getUserQuotaCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Quota, nil
}

func getUserStatusCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Status, nil
}

func getUserNameCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Username, nil
}

func getUserSettingCache(userId int) (dto.UserSetting, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return dto.UserSetting{}, err
	}
	return cache.GetSetting(), nil
}

// New functions for individual field updates
func updateUserStatusCache(userId int, status bool) error {
	if !common.RedisEnabled {
		return nil
	}
	statusInt := common.UserStatusEnabled
	if !status {
		statusInt = common.UserStatusDisabled
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Status", fmt.Sprintf("%d", statusInt))
}

func updateUserQuotaCache(userId int, quota int) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Quota", fmt.Sprintf("%d", quota))
}

func updateUserGroupCache(userId int, group string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Group", group)
}

func UpdateUserGroupCache(userId int, group string) error {
	return updateUserGroupCache(userId, group)
}

func updateUserNameCache(userId int, username string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Username", username)
}

func updateUserSettingCache(userId int, setting string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Setting", setting)
}

// GetUserLanguage returns the user's language preference from cache
// Uses the existing GetUserCache mechanism for efficiency
func GetUserLanguage(userId int) string {
	userCache, err := GetUserCache(userId)
	if err != nil {
		return ""
	}
	return userCache.GetSetting().Language
}
