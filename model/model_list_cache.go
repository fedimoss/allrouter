package model

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// modelListCachePrefix 是模型列表相关缓存的统一键前缀。
// 本文件（model_list_cache.go）负责模型列表缓存的整体读写：
// 包括“分组下已启用模型”、“全局已启用模型”，以及每个模型“优先归属”的渠道类型。
// 采用统一前缀，便于在渠道/能力数据变更时一次性扫描并清除全部相关缓存。
const modelListCachePrefix = "model_list_cache:"

// modelListCacheRedisEnabled 判断模型列表缓存是否可用。
// 要求 Redis 已启用，且 Redis 客户端（RDB）已初始化，二者缺一则视为缓存不可用。
func modelListCacheRedisEnabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

// modelListCacheTTL 返回模型列表缓存的全局过期时间（TTL）。
// 时长由公共配置 RedisKeyCacheSeconds 决定（单位秒），到期后缓存自动失效。
func modelListCacheTTL() time.Duration {
	return time.Duration(common.RedisKeyCacheSeconds()) * time.Second
}

// getGroupEnabledModelsCacheKey 生成“指定分组下已启用模型列表”的缓存键。
// 对分组名做去空白处理，避免前后空格导致同一分组生成不同的键。
func getGroupEnabledModelsCacheKey(group string) string {
	return modelListCachePrefix + "group_enabled_models:" + strings.TrimSpace(group)
}

// getEnabledModelsCacheKey 生成“全局已启用模型列表”的缓存键（不区分分组）。
func getEnabledModelsCacheKey() string {
	return modelListCachePrefix + "enabled_models"
}

// getPreferredOwnerCacheKey 生成“模型优先归属渠道类型”结果的缓存键。
// 入参中的模型名与分组均为切片，顺序不同会拼出不同字符串；
// 这里先复制切片（避免修改调用方原始切片），再排序后拼接，
// 从而保证“相同元素集合（无论顺序）”始终生成相同的缓存键。
// 拼接结果经过 HMAC 摘要，避免过长的明文键，并保持键稳定可复现。
func getPreferredOwnerCacheKey(modelNames []string, groups []string) string {
	// 复制入参切片，避免后续排序影响调用方传入的原始切片
	modelNames = append([]string(nil), modelNames...)
	groups = append([]string(nil), groups...)
	// 对两份切片分别排序，使“元素集合相同但顺序不同”时也能生成相同的键
	sort.Strings(modelNames)
	sort.Strings(groups)

	return fmt.Sprintf(
		"%spreferred_owner:%s:%s",
		modelListCachePrefix,
		common.GenerateHMAC(strings.Join(groups, "\x00")),     // 分组用 \x00 连接后做 HMAC 摘要
		common.GenerateHMAC(strings.Join(modelNames, "\x00")), // 模型名用 \x00 连接后做 HMAC 摘要
	)
}

// getStringSliceModelListCache 从 Redis 读取字符串切片形式的模型列表缓存。
// 第二个返回值 bool 表示是否命中：true 表示命中并返回数据，false 表示未命中或缓存不可用。
func getStringSliceModelListCache(key string) ([]string, bool) {
	// 缓存不可用时直接返回未命中
	if !modelListCacheRedisEnabled() {
		return nil, false
	}
	raw, err := common.RedisGet(key)
	// 读取失败或值为空，均视为未命中
	if err != nil || raw == "" {
		return nil, false
	}
	var value []string
	// 反序列化失败说明缓存内容已损坏：删除该坏键并视为未命中，避免后续持续命中坏数据
	if err := common.Unmarshal([]byte(raw), &value); err != nil {
		_ = common.RedisDelKey(key)
		return nil, false
	}
	return value, true
}

// setStringSliceModelListCache 将字符串切片形式的模型列表写入 Redis 缓存。
// 序列化或写入失败仅记录日志、不向上抛错，避免缓存写入失败阻断业务主流程。
func setStringSliceModelListCache(key string, value []string) {
	// 缓存不可用时直接跳过
	if !modelListCacheRedisEnabled() {
		return
	}
	data, err := common.Marshal(value)
	// 序列化失败：记录日志后直接返回，无法写入缓存
	if err != nil {
		common.SysError(fmt.Sprintf("failed to marshal model list cache: key=%s, error=%v", key, err))
		return
	}
	// 写入 Redis 并设置过期时间；失败仅记录日志，不向上抛错
	if err := common.RedisSet(key, string(data), modelListCacheTTL()); err != nil {
		common.SysError(fmt.Sprintf("failed to write model list cache: key=%s, error=%v", key, err))
	}
}

// getPreferredOwnerModelListCache 从 Redis 读取“模型优先归属渠道类型”缓存（map 结构）。
// 第二个返回值 bool 表示是否命中：true 表示命中并返回数据，false 表示未命中或缓存不可用。
func getPreferredOwnerModelListCache(key string) (map[string]int, bool) {
	// 缓存不可用时直接返回未命中
	if !modelListCacheRedisEnabled() {
		return nil, false
	}
	raw, err := common.RedisGet(key)
	// 读取失败或值为空，均视为未命中
	if err != nil || raw == "" {
		return nil, false
	}
	value := make(map[string]int)
	// 反序列化失败说明缓存内容已损坏：删除该坏键并视为未命中，避免后续持续命中坏数据
	if err := common.Unmarshal([]byte(raw), &value); err != nil {
		_ = common.RedisDelKey(key)
		return nil, false
	}
	return value, true
}

// setPreferredOwnerModelListCache 将“模型优先归属渠道类型”结果（map）写入 Redis 缓存。
// 序列化或写入失败仅记录日志、不向上抛错，避免缓存写入失败阻断业务主流程。
func setPreferredOwnerModelListCache(key string, value map[string]int) {
	// 缓存不可用时直接跳过
	if !modelListCacheRedisEnabled() {
		return
	}
	data, err := common.Marshal(value)
	// 序列化失败：记录日志后直接返回，无法写入缓存
	if err != nil {
		common.SysError(fmt.Sprintf("failed to marshal preferred owner cache: key=%s, error=%v", key, err))
		return
	}
	// 写入 Redis 并设置过期时间；失败仅记录日志，不向上抛错
	if err := common.RedisSet(key, string(data), modelListCacheTTL()); err != nil {
		common.SysError(fmt.Sprintf("failed to write preferred owner cache: key=%s, error=%v", key, err))
	}
}

// InvalidateModelListCache 失效（清除）所有模型列表相关缓存。
// 通过 SCAN 游标遍历所有以 modelListCachePrefix 为前缀的键并批量删除，
// 用于渠道/能力数据发生变更（新增、删除、启用/禁用、修改标签等）之后，
// 强制后续查询重新从数据库加载最新数据。
func InvalidateModelListCache() error {
	// 缓存不可用时无需清除，直接返回
	if !modelListCacheRedisEnabled() {
		return nil
	}

	ctx := context.Background()
	var cursor uint64
	// 使用 SCAN 分批迭代，避免 KEYS 在键数量大时阻塞 Redis
	for {
		// 以游标扫描匹配前缀的键，每批最多返回 1000 个
		keys, nextCursor, err := common.RDB.Scan(ctx, cursor, modelListCachePrefix+"*", 1000).Result()
		// 扫描出错直接返回该错误
		if err != nil {
			return err
		}
		// 本批扫描到键则批量删除；为空则跳过删除
		if len(keys) > 0 {
			if err := common.RDB.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		// 推进游标；游标回到 0 表示全量扫描结束
		cursor = nextCursor
		if cursor == 0 {
			return nil
		}
	}
}

// invalidateModelListCacheAfter 是一个便捷封装：仅当传入错误为 nil（即前置操作成功）时，
// 才执行缓存失效；若前置操作已出错，则原样透传该错误，避免掩盖原始错误。
func invalidateModelListCacheAfter(err error) error {
	// 前置操作已出错，直接返回该错误，不执行缓存失效
	if err != nil {
		return err
	}
	// 前置操作成功，失效模型列表缓存
	return InvalidateModelListCache()
}
