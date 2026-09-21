package model

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/bytedance/gopkg/util/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CliUser struct {
	Id string `json:"id" gorm:"primaryKey"`
	// 根据你的业务需求添加字段，例如：
	UserId    string     `json:"user_id" gorm:"type:varchar(50);uniqueIndex"` //唯一约束防止高并发下重复插入
	Status    int        `json:"status" gorm:"type:int;default:1"`
	CreatedAt *time.Time `json:"created_at" gorm:"timestamptz"`
	UpdatedAt *time.Time `json:"updated_at" gorm:"timestamptz"`
}

type CliUserOAuth struct {
	Id         string `json:"id" gorm:"primaryKey"`
	CliUserId  string `json:"cli_user_id" gorm:"type:varchar(50)"`
	CliOauthId string `json:"cli_oauth_id" gorm:"type:varchar(50)"`
}
type CliOAuth struct {
	Id        string     `json:"id" gorm:"primaryKey"`
	Oauth     string     `json:"oauth" gorm:"type:text"`
	ModelType int        `json:"model_type" gorm:"type:int4"`
	CreatedAt *time.Time `json:"created_at" gorm:"timestamptz"`
	UpdatedAt *time.Time `json:"updated_at" gorm:"timestamptz"`

	UserName string `json:"user_name,omitempty" gorm:"-"`
}

type DeleteCliInfo struct {
	ReMainModelCount int    `json:"remain_model_count"`
	CliUserId        string `json:"cli_user_id"`
	AuthType         int    `json:"auth_type"`
}

type CliUserOAuthCountInfo struct {
	Count      int64 `json:"count"`
	ModelTypes []int `json:"model_types"`
}

func (CliUser) TableName() string {
	return "cli_user"
}
func (CliUserOAuth) TableName() string {
	return "cli_user_oauth"
}
func (CliOAuth) TableName() string {
	return "cli_oauth"
}

func GetCliUserByCon(cu *CliUser) ([]CliUser, error) {
	// 根据你的业务需求添加查询条件，例如：
	// db.Where("status = ?", cu.Status).Find(&users)
	db := DB.Model(&CliUser{})
	var cUsers []CliUser
	if strings.TrimSpace(cu.UserId) != "" {
		db.Where("user_id = ?", cu.UserId)
	}

	err := db.Find(&cUsers).Limit(2).Error

	if err != nil {
		logger.Errorf("get cli user err: %v", err)
		return nil, err
	}
	return cUsers, nil
}

func InsertNewCliUser(userId string, tx *gorm.DB) (*CliUser, error) {
	now := time.Now()
	cu := &CliUser{
		Id:        uuid.New().String(),
		UserId:    userId,
		Status:    1,
		CreatedAt: &now,
		UpdatedAt: &now,
	}
	var err error
	if tx != nil {
		err = tx.Create(cu).Error
	} else {
		err = DB.Create(cu).Error
	}

	if err != nil {
		logger.Errorf("insert cli user err: %v", err)
		return nil, err
	}
	return cu, nil
}

func GetAPIKeyByUserId(userId int) (*CliUser, error) {
	uid := strconv.Itoa(userId)
	now := time.Now()
	cliUser := &CliUser{
		UserId: uid,
	}
	// FirstOrCreate 在数据库层面保证原子性，配合 user_id 唯一索引防止并发重复插入
	result := DB.Where("user_id = ?", uid).Attrs(CliUser{
		Id:        uuid.New().String(),
		Status:    1,
		CreatedAt: &now,
		UpdatedAt: &now,
	}).FirstOrCreate(cliUser)
	if result.Error != nil {
		return nil, result.Error
	}
	return cliUser, nil
}

func getCliUserOAuthLookupIDs(userId int) (*CliUser, []string, error) {
	userIdStr := strconv.Itoa(userId)
	cliUser, err := GetAPIKeyByUserId(userId)
	if err != nil {
		return nil, nil, err
	}
	lookupIDs := []string{cliUser.Id}
	if cliUser.Id != userIdStr {
		lookupIDs = append(lookupIDs, userIdStr)
	}
	return cliUser, lookupIDs, nil
}

// 更新渠道中的用户已经授权的认证（cli proxy api）
func UpdateCliUserOAuth(channedl *Channel) error {
	err := DB.Model(channedl).Update("key", channedl.Key).Where("id = ?", channedl.Id).Error
	if err != nil {
		return err
	}
	DB.Model(channedl).First(channedl, "id = ?", channedl.Id)
	err = channedl.UpdateAbilities(nil)
	return nil
}

// clioauth列表数据
func GetCliOAuthPages(userId int, pageInfo *common.PageInfo, modetype int64, keyword string) (co []*CliOAuth, total int64, err error) {
	_, lookupIDs, err := getCliUserOAuthLookupIDs(userId)
	if err != nil {
		return nil, 0, err
	}

	linkQuery := DB.Model(&CliUserOAuth{}).
		Distinct("cli_user_oauth.cli_oauth_id").
		Joins("JOIN cli_oauth ON cli_user_oauth.cli_oauth_id = cli_oauth.id").
		Where("cli_user_oauth.cli_user_id IN ?", lookupIDs)
	if modetype != 0 {
		linkQuery = linkQuery.Where("cli_oauth.model_type = ?", modetype)
	}
	if keyword != "" {
		linkQuery = linkQuery.Where("cli_oauth.id LIKE ?", "%"+keyword+"%")
	}

	err = linkQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	var ids []string
	err = linkQuery.Order("cli_user_oauth.cli_oauth_id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Pluck("cli_user_oauth.cli_oauth_id", &ids).Error
	if err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []*CliOAuth{}, total, nil
	}

	err = DB.Model(&CliOAuth{}).Where("id IN ?", ids).Order("updated_at desc,created_at desc").Find(&co).Error
	return co, total, err
}

type cliOAuthUserRow struct {
	Id        string     `gorm:"column:id"`
	Oauth     string     `gorm:"column:oauth"`
	ModelType int        `gorm:"column:model_type"`
	CreatedAt *time.Time `gorm:"column:created_at"`
	UpdatedAt *time.Time `gorm:"column:updated_at"`
	UserId    string     `gorm:"column:user_id"`
}

func allCliOAuthUserPairs(modetype int64, keyword string) *gorm.DB {
	query := DB.Table("cli_user_oauth").
		Select("cli_user_oauth.cli_oauth_id, COALESCE(cli_user.user_id, cli_user_oauth.cli_user_id) AS user_id").
		Joins("JOIN cli_oauth ON cli_oauth.id = cli_user_oauth.cli_oauth_id").
		Joins("LEFT JOIN cli_user ON cli_user.id = cli_user_oauth.cli_user_id")
	if modetype != 0 {
		query = query.Where("cli_oauth.model_type = ?", modetype)
	}
	if keyword != "" {
		query = query.Where("cli_oauth.id LIKE ?", "%"+keyword+"%")
	}
	return query.Group("cli_user_oauth.cli_oauth_id, COALESCE(cli_user.user_id, cli_user_oauth.cli_user_id)")
}

// GetAllCliOAuthPages returns one row for each unique user/OAuth association.
func GetAllCliOAuthPages(pageInfo *common.PageInfo, modetype int64, keyword string) ([]*CliOAuth, int64, error) {
	var total int64
	if err := DB.Table("(?) AS oauth_users", allCliOAuthUserPairs(modetype, keyword)).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*CliOAuth{}, 0, nil
	}

	var rows []cliOAuthUserRow
	err := DB.Table("(?) AS oauth_users", allCliOAuthUserPairs(modetype, keyword)).
		Select("cli_oauth.id, cli_oauth.oauth, cli_oauth.model_type, cli_oauth.created_at, cli_oauth.updated_at, oauth_users.user_id").
		Joins("JOIN cli_oauth ON cli_oauth.id = oauth_users.cli_oauth_id").
		Order("cli_oauth.updated_at DESC").
		Order("cli_oauth.created_at DESC").
		Order("cli_oauth.id DESC").
		Order("oauth_users.user_id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []*CliOAuth{}, total, nil
	}

	userIds := make([]int, 0, len(rows))
	seenUserIds := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		userId, parseErr := strconv.Atoi(strings.TrimSpace(row.UserId))
		if parseErr != nil {
			continue
		}
		if _, ok := seenUserIds[userId]; ok {
			continue
		}
		seenUserIds[userId] = struct{}{}
		userIds = append(userIds, userId)
	}

	var users []struct {
		Id       int    `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	if err = DB.Model(&User{}).Select("id, username").Where("id IN ?", userIds).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	userNames := make(map[int]string, len(users))
	for _, user := range users {
		userNames[user.Id] = user.Username
	}

	oauths := make([]*CliOAuth, 0, len(rows))
	for _, row := range rows {
		userId, _ := strconv.Atoi(strings.TrimSpace(row.UserId))
		oauths = append(oauths, &CliOAuth{
			Id:        row.Id,
			Oauth:     row.Oauth,
			ModelType: row.ModelType,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			UserName:  userNames[userId],
		})
	}
	return oauths, total, nil
}

// 删除用户的认证文件
func DeleteCliOAuth(userId int, oauthId string) (*DeleteCliInfo, error) {
	cliUser, lookupIDs, err := getCliUserOAuthLookupIDs(userId)
	if err != nil {
		return nil, err
	}

	var oauth CliOAuth
	err = DB.Model(&CliOAuth{}).
		Select("cli_oauth.id, cli_oauth.oauth, cli_oauth.model_type, cli_oauth.created_at, cli_oauth.updated_at").
		Joins("JOIN cli_user_oauth ON cli_user_oauth.cli_oauth_id = cli_oauth.id").
		Where("cli_oauth.id = ? AND cli_user_oauth.cli_user_id IN ?", oauthId, lookupIDs).
		Take(&oauth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found this oauth file!")
		}
		return nil, err
	}

	deleteInfo := &DeleteCliInfo{
		CliUserId: cliUser.Id,
		AuthType:  oauth.ModelType,
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		deleteResult := tx.Where("cli_oauth_id = ? AND cli_user_id IN ?", oauthId, lookupIDs).Delete(&CliUserOAuth{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}
		if deleteResult.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		var oauthRefCount int64
		err := tx.Model(&CliUserOAuth{}).Where("cli_oauth_id = ?", oauthId).Count(&oauthRefCount).Error
		if err != nil {
			return err
		}
		if oauthRefCount == 0 {
			err = tx.Where("id = ?", oauthId).Delete(&CliOAuth{}).Error
			if err != nil {
				return err
			}
		}
		//查询剩余统计
		var remainingCount int64
		err = tx.Model(&CliUserOAuth{}).
			Distinct("cli_user_oauth.cli_oauth_id").
			Joins("JOIN cli_oauth ON cli_user_oauth.cli_oauth_id = cli_oauth.id").
			Where("cli_user_oauth.cli_user_id IN ? AND cli_oauth.model_type = ?", lookupIDs, oauth.ModelType).
			Count(&remainingCount).Error
		if err != nil {
			return err
		}
		deleteInfo.ReMainModelCount = int(remainingCount)
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found this oauth file!")
		}
		return nil, err
	}

	return deleteInfo, nil

}

// 通过id和userid查找用户的认证文件
func GetOAuthByIdAndUserId(userId int, oauthId string) (*CliOAuth, error) {
	_, lookupIDs, err := getCliUserOAuthLookupIDs(userId)
	if err != nil {
		return nil, err
	}
	var cliOauth CliOAuth
	err = DB.Model(&CliOAuth{}).
		Joins("JOIN cli_user_oauth ON cli_user_oauth.cli_oauth_id = cli_oauth.id").
		Where("cli_oauth.id = ? AND cli_user_oauth.cli_user_id IN ?", oauthId, lookupIDs).
		First(&cliOauth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found this oauth file!")
		}
		return nil, err
	}
	return &cliOauth, nil
}

// GetOAuthByIdAdmin 管理员按 id 查找任意用户的认证文件（不限归属）
func GetOAuthByIdAdmin(oauthId string) (*CliOAuth, error) {
	var cliOauth CliOAuth
	err := DB.Model(&CliOAuth{}).
		Joins("JOIN cli_user_oauth ON cli_user_oauth.cli_oauth_id = cli_oauth.id").
		Where("cli_oauth.id = ?", oauthId).
		First(&cliOauth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found this oauth file!")
		}
		return nil, err
	}
	return &cliOauth, nil
}

// DeleteCliOAuthAdmin 管理员删除任意用户的认证文件关联（不限归属）
func DeleteCliOAuthAdmin(oauthId string) (*DeleteCliInfo, error) {
	var cliOauth CliOAuth
	err := DB.Model(&CliOAuth{}).
		Select("cli_oauth.id, cli_oauth.oauth, cli_oauth.model_type, cli_oauth.created_at, cli_oauth.updated_at").
		Joins("JOIN cli_user_oauth ON cli_user_oauth.cli_oauth_id = cli_oauth.id").
		Where("cli_oauth.id = ?", oauthId).
		Take(&cliOauth).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found this oauth file!")
		}
		return nil, err
	}

	deleteInfo := &DeleteCliInfo{
		AuthType: cliOauth.ModelType,
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		// 删除该认证文件的一个关联，保留的归属按 cli_user_id 排序取最小（用于渠道密钥清理定位）
		var ownerRow CliUserOAuth
		err := tx.Where("cli_oauth_id = ?", oauthId).
			Order("cli_user_id ASC").
			Take(&ownerRow).Error
		if err != nil {
			return err
		}
		deleteInfo.CliUserId = ownerRow.CliUserId

		deleteResult := tx.Where("cli_oauth_id = ? AND cli_user_id = ?", oauthId, ownerRow.CliUserId).Delete(&CliUserOAuth{})
		if deleteResult.Error != nil {
			return deleteResult.Error
		}
		if deleteResult.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		var oauthRefCount int64
		err = tx.Model(&CliUserOAuth{}).Where("cli_oauth_id = ?", oauthId).Count(&oauthRefCount).Error
		if err != nil {
			return err
		}
		if oauthRefCount == 0 {
			err = tx.Where("id = ?", oauthId).Delete(&CliOAuth{}).Error
			if err != nil {
				return err
			}
		}
		//查询剩余统计（按该归属用户的其余认证文件计）
		var remainingCount int64
		err = tx.Model(&CliUserOAuth{}).
			Distinct("cli_user_oauth.cli_oauth_id").
			Joins("JOIN cli_oauth ON cli_user_oauth.cli_oauth_id = cli_oauth.id").
			Where("cli_user_oauth.cli_user_id = ? AND cli_oauth.model_type = ?", ownerRow.CliUserId, cliOauth.ModelType).
			Count(&remainingCount).Error
		if err != nil {
			return err
		}
		deleteInfo.ReMainModelCount = int(remainingCount)
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found this oauth file!")
		}
		return nil, err
	}

	return deleteInfo, nil
}

// 获取用户成功的认证服务
func GetCliUserOAuthCountByUserId(userId int) (*CliUserOAuthCountInfo, error) {
	_, lookupIDs, err := getCliUserOAuthLookupIDs(userId)
	if err != nil {
		return nil, err
	}
	var modelTypes []int
	err = DB.Model(&CliUserOAuth{}).
		Distinct("cli_oauth.model_type").
		Joins("JOIN cli_oauth ON cli_user_oauth.cli_oauth_id = cli_oauth.id").
		Where("cli_user_oauth.cli_user_id IN ?", lookupIDs).
		Order("cli_oauth.model_type").
		Pluck("cli_oauth.model_type", &modelTypes).Error
	if err != nil {
		return nil, err
	}
	return &CliUserOAuthCountInfo{
		Count:      int64(len(modelTypes)),
		ModelTypes: modelTypes,
	}, nil
}

// cleanupDuplicateCliUsers 在迁移添加唯一索引前，清理重复的 user_id 记录（保留最早创建的那条）
func cleanupDuplicateCliUsers() {
	// 检查表是否存在
	if !DB.Migrator().HasTable("cli_user") {
		return
	}

	type dupRecord struct {
		UserId string
		Cnt    int
	}
	var dups []dupRecord
	DB.Raw("SELECT user_id, COUNT(*) as cnt FROM cli_user GROUP BY user_id HAVING COUNT(*) > 1").Scan(&dups)
	if len(dups) == 0 {
		return
	}
	logger.Infof("found %d user_ids with duplicate cli_user records, cleaning up...", len(dups))
	for _, d := range dups {
		var users []CliUser
		DB.Where("user_id = ?", d.UserId).Order("created_at ASC").Find(&users)
		if len(users) <= 1 {
			continue
		}
		// 保留第一条，删除其余
		idsToDelete := make([]string, 0, len(users)-1)
		for _, u := range users[1:] {
			idsToDelete = append(idsToDelete, u.Id)
		}
		DB.Where("id IN ?", idsToDelete).Delete(&CliUser{})
		logger.Infof("cleaned up %d duplicate cli_user records for user_id=%s", len(idsToDelete), d.UserId)
	}
}
