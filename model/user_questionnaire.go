package model

import (
	"github.com/QuantumNous/new-api/common"
)

// UserQuestionnaire 用户问卷提交记录
// 主站(provider_id=0)与服务商站点(provider_id>0)共用，表结构手动创建(自动迁移已禁用)。
// SurveyData 为整张问卷表单的 JSON，字段随问卷内容变化，不入库单列。
type UserQuestionnaire struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ProviderId int    `json:"provider_id" gorm:"not null;default:0;index"`
	UserId     int    `json:"user_id" gorm:"not null;index"`
	SurveyData string `json:"survey_data" gorm:"type:text"` // 问卷数据 JSON 文本
	CreatedAt  int64  `json:"created_at" gorm:"bigint"`
}

func (UserQuestionnaire) TableName() string {
	return "user_questionnaires"
}

// CreateUserQuestionnaire 保存一条问卷提交记录，providerId 由服务端根据站点上下文写入
func CreateUserQuestionnaire(providerId int, userId int, surveyData string) (*UserQuestionnaire, error) {
	record := &UserQuestionnaire{
		ProviderId: providerId,
		UserId:     userId,
		SurveyData: surveyData,
		CreatedAt:  common.GetTimestamp(),
	}
	if err := DB.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// GetUserQuestionnairesInProvider 获取用户在指定站点的问卷提交记录，严格按站点隔离，按提交时间倒序
func GetUserQuestionnairesInProvider(providerId int, userId int) ([]UserQuestionnaire, error) {
	var records []UserQuestionnaire
	err := DB.
		Where("provider_id = ? AND user_id = ?", providerId, userId).
		Order("id DESC").
		Find(&records).Error
	return records, err
}

// AdminQuestionnaireRecord 管理端问卷提交记录视图（联表带用户/分站信息）
type AdminQuestionnaireRecord struct {
	Id           int    `json:"id"`
	ProviderId   int    `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	SurveyData   string `json:"survey_data"`
	CreatedAt    int64  `json:"created_at"`
}

// GetUserQuestionnairesAdmin 管理端分页查询问卷提交记录。
// providerId 为站点 ID：0=主站，>0=服务商站点，-1=全部站点；按提交时间倒序。
func GetUserQuestionnairesAdmin(providerId int, startIdx int, pageSize int) ([]AdminQuestionnaireRecord, int64, error) {
	// 注意：因 LEFT JOIN users（users 表也有 provider_id 列），所有列引用必须加 user_questionnaires 前缀
	base := DB.Model(&UserQuestionnaire{})
	if providerId >= 0 {
		base = base.Where("user_questionnaires.provider_id = ?", providerId)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []AdminQuestionnaireRecord
	err := base.
		Select("user_questionnaires.id, user_questionnaires.provider_id, " +
			"COALESCE(providers.name, '') AS provider_name, user_questionnaires.user_id, " +
			"COALESCE(users.username, '') AS username, COALESCE(users.display_name, '') AS display_name, " +
			"user_questionnaires.survey_data, user_questionnaires.created_at").
		Joins("LEFT JOIN users ON users.id = user_questionnaires.user_id").
		Joins("LEFT JOIN providers ON providers.id = user_questionnaires.provider_id").
		Order("user_questionnaires.id DESC").
		Offset(startIdx).Limit(pageSize).
		Scan(&records).Error
	return records, total, err
}

// DeleteUserQuestionnaireById 按 id 删除任意站点问卷记录（仅主站管理员使用）。
func DeleteUserQuestionnaireById(id int) error {
	return DB.Where("id = ?", id).Delete(&UserQuestionnaire{}).Error
}

// DeleteUserQuestionnaire 删除问卷提交记录，严格按站点隔离（providerId=0 仅可删主站记录）。
func DeleteUserQuestionnaire(id int, providerId int) error {
	query := DB.Where("id = ? AND provider_id = ?", id, providerId)
	return query.Delete(&UserQuestionnaire{}).Error
}
