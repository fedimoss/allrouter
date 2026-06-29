package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType int `json:"channel_type"`
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Scan(&abilities).Error
	return abilities, err
}

// GetGroupEnabledModels 获取指定分组下所有已启用（enabled）的模型列表。
// 采用“先读缓存、未命中再查库并回填”的策略，避免每次请求都访问数据库，提升查询性能。
func GetGroupEnabledModels(group string) []string {
	// 根据分组生成对应的模型列表缓存键
	key := getGroupEnabledModelsCacheKey(group)
	// 优先尝试从缓存中读取该分组已启用的模型列表
	if cachedModels, ok := getStringSliceModelListCache(key); ok {
		// 缓存命中（ok == true），直接返回缓存结果，跳过数据库查询
		return cachedModels
	}

	// 缓存未命中，回退到数据库查询该分组已启用的模型
	models := getGroupEnabledModelsFromDB(group)
	// 将数据库查询结果回填到缓存，供后续相同分组的请求命中
	setStringSliceModelListCache(key, models)
	return models
}

// getGroupEnabledModelsFromDB 直接从数据库查询指定分组下已启用的模型。
// 查询 abilities 表，过滤条件为：分组匹配且 enabled = true，并按 model 去重。
func getGroupEnabledModelsFromDB(group string) []string {
	var models []string
	// Find distinct models
	// 注意：group 是 SQL 保留字，需用 commonGroupCol 按当前数据库类型对列名进行转义；
	//       Distinct("model") 对模型去重，Pluck 将单列查询结果扫描进 models 切片
	DB.Table("abilities").Where(commonGroupCol+" = ? and enabled = ?", group, true).Distinct("model").Pluck("model", &models)
	return models
}

// GetEnabledModels 获取系统中所有已启用（enabled）的模型列表（不区分分组）。
// 同样采用“先读缓存、未命中再查库并回填”的策略。
func GetEnabledModels() []string {
	// 获取全局已启用模型列表的缓存键（无分组维度）
	key := getEnabledModelsCacheKey()
	// 优先尝试从缓存中读取已启用的模型列表
	if cachedModels, ok := getStringSliceModelListCache(key); ok {
		// 缓存命中，直接返回缓存结果，跳过数据库查询
		return cachedModels
	}

	// 缓存未命中，回退到数据库查询所有已启用的模型
	models := getEnabledModelsFromDB()
	// 将数据库查询结果回填到缓存，供后续请求命中
	setStringSliceModelListCache(key, models)
	return models
}

// getEnabledModelsFromDB 直接从数据库查询所有已启用的模型。
// 查询 abilities 表，过滤条件为 enabled = true，并按 model 去重。
func getEnabledModelsFromDB() []string {
	var models []string
	// Find distinct models
	// Distinct("model") 对模型去重，Pluck 将单列查询结果扫描进 models 切片
	DB.Table("abilities").Where("enabled = ?", true).Distinct("model").Pluck("model", &models)
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

func getPriority(group string, model string, retry int) (int, error) {

	var priorities []int
	err := DB.Model(&Ability{}).
		Select("DISTINCT(priority)").
		Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).
		Order("priority DESC").              // 按优先级降序排序
		Pluck("priority", &priorities).Error // Pluck用于将查询的结果直接扫描到一个切片中

	if err != nil {
		// 处理错误
		return 0, err
	}

	if len(priorities) == 0 {
		// 如果没有查询到优先级，则返回错误
		return 0, errors.New("数据库一致性被破坏")
	}

	// 确定要使用的优先级
	var priorityToUse int
	if retry >= len(priorities) {
		// 如果重试次数大于优先级数，则使用最小的优先级
		priorityToUse = priorities[len(priorities)-1]
	} else {
		priorityToUse = priorities[retry]
	}
	return priorityToUse, nil
}

func getChannelQuery(group string, model string, retry int) (*gorm.DB, error) {
	maxPrioritySubQuery := DB.Model(&Ability{}).Select("MAX(priority)").Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true)
	channelQuery := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = (?)", group, model, true, maxPrioritySubQuery)
	if retry != 0 {
		priority, err := getPriority(group, model, retry)
		if err != nil {
			return nil, err
		} else {
			channelQuery = DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = ?", group, model, true, priority)
		}
	}

	return channelQuery, nil
}

func GetChannel(group string, model string, retry int) (*Channel, error) {
	var abilities []Ability

	var err error = nil
	channelQuery, err := getChannelQuery(group, model, retry)
	if err != nil {
		return nil, err
	}
	if common.UsingSQLite || common.UsingPostgreSQL {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	} else {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	}
	if err != nil {
		return nil, err
	}
	channel := Channel{}
	if len(abilities) > 0 {
		// Randomly choose one
		weightSum := uint(0)
		for _, ability_ := range abilities {
			weightSum += ability_.Weight + 10
		}
		// Randomly choose one
		weight := common.GetRandomInt(int(weightSum))
		for _, ability_ := range abilities {
			weight -= int(ability_.Weight) + 10
			//log.Printf("weight: %d, ability weight: %d", weight, *ability_.Weight)
			if weight <= 0 {
				channel.Id = ability_.ChannelId
				break
			}
		}
	} else {
		return nil, nil
	}
	err = DB.First(&channel, "id = ?", channel.Id).Error
	return &channel, err
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	// 若调用方未传入外部事务（tx == nil），说明本次新增已直接落库，
	// 需要失效模型列表缓存，使后续查询能读到包含新增能力后的最新模型列表；
	// 若传入了外部事务，则缓存失效交由事务提交方负责，此处直接返回成功
	if tx == nil {
		return InvalidateModelListCache()
	}
	return nil
}

// DeleteAbilities 删除该渠道（channel）的所有能力（abilities）记录。
// 删除后通过 invalidateModelListCacheAfter 失效模型列表缓存，确保查询结果不再包含已删除的能力。
func (channel *Channel) DeleteAbilities() error {
	// 按 channel_id 删除该渠道对应的全部能力记录；
	// invalidateModelListCacheAfter 会在删除操作无错误时失效模型列表缓存，有错误则原样向上返回
	return invalidateModelListCacheAfter(DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error)
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}

	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		// 提交事务；提交失败直接返回错误（事务内的删除与新增随之不生效）
		if err = tx.Commit().Error; err != nil {
			return err
		}
		// 事务提交成功后失效模型列表缓存，使后续查询反映更新后的能力状态
		return InvalidateModelListCache()
	}

	// 使用外部事务时，缓存失效交由事务提交方负责，此处直接返回成功
	return nil
}

// UpdateAbilityStatus 按渠道 ID 批量更新该渠道下所有能力的启用状态（enabled）。
// 更新后通过 invalidateModelListCacheAfter 失效模型列表缓存。
func UpdateAbilityStatus(channelId int, status bool) error {
	// 以 channel_id 定位能力记录，仅更新 enabled 字段为新状态；
	// invalidateModelListCacheAfter 在更新无错误时失效缓存，有错误则原样向上返回
	return invalidateModelListCacheAfter(DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error)
}

// UpdateAbilityStatusByTag 按标签（tag）批量更新所有匹配能力的启用状态（enabled）。
// 更新后通过 invalidateModelListCacheAfter 失效模型列表缓存。
func UpdateAbilityStatusByTag(tag string, status bool) error {
	// 以 tag 定位能力记录，仅更新 enabled 字段为新状态；
	// invalidateModelListCacheAfter 在更新无错误时失效缓存，有错误则原样向上返回
	return invalidateModelListCacheAfter(DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error)
}

func UpdateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	ability := Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	if priority != nil {
		ability.Priority = priority
	}
	if weight != nil {
		ability.Weight = *weight
	}
	return invalidateModelListCacheAfter(DB.Model(&Ability{}).Where("tag = ?", tag).Updates(ability).Error)
}

var fixLock = sync.Mutex{}

func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if common.UsingSQLite {
		err := DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *Channel, _ int) int { return c.Id })
		// Delete all abilities of this channel
		err = DB.Where("channel_id IN ?", ids).Delete(&Ability{}).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		// Then add new abilities
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	// 重建渠道缓存，使渠道相关的内存索引反映修复后的最新数据
	InitChannelCache()
	// 修复完成后失效模型列表缓存，使后续查询能读到重建后的能力数据；
	// 若缓存失效失败，则连同成功/失败计数一并返回该错误
	if err := InvalidateModelListCache(); err != nil {
		return successCount, failCount, err
	}
	// 缓存失效成功，返回修复的成功数、失败数，且无错误
	return successCount, failCount, nil
}
