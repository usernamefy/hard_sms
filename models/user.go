package models

import (
	"time"

	"gorm.io/gorm"
)

// DB 由 database 包初始化时注入，模型层统一通过它访问数据库
var DB *gorm.DB

// SetDB 注入数据库连接
func SetDB(db *gorm.DB) {
	DB = db
}

// User 用户表模型（映射 tbl_users，前缀由 NamingStrategy 全局配置）
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"` // 用户名
	Password  string         `gorm:"type:varchar(32);not null" json:"-"`                    // 密码（MD5 加密存储）
	RealName  string         `gorm:"type:varchar(50)" json:"realName"`                      // 真实姓名
	Role      string         `gorm:"type:varchar(20);default:admin" json:"role"`            // 角色
	Status    int            `gorm:"default:1" json:"status"`                               // 状态：1 启用 0 禁用
	LastLogin *time.Time     `json:"lastLogin"`                                             // 最后登录时间
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// GetUserByID 根据 ID 查询用户
func GetUserByID(id uint) (*User, error) {
	var user User
	if err := DB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// DisplayName 用户展示名：优先真实姓名，其次用户名
func (u *User) DisplayName() string {
	if u == nil {
		return "-"
	}
	if u.RealName != "" {
		return u.RealName
	}
	return u.Username
}

// ListEnabledUsers 查询全部启用状态的用户（供管理员下拉选择）
func ListEnabledUsers() ([]User, error) {
	var users []User
	err := DB.Where("status = ?", 1).Order("id ASC").Find(&users).Error
	return users, err
}

// GetUserByUsername 根据用户名查询用户
func GetUserByUsername(username string) (*User, error) {
	var user User
	if err := DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UserQuery 用户列表查询条件
type UserQuery struct {
	Keyword  string // 用户名/姓名模糊搜索
	Page     int    // 页码，从 1 开始
	PageSize int    // 每页条数
}

// ListUsers 分页查询用户列表，返回列表和总数
func ListUsers(q UserQuery) ([]User, int64, error) {
	db := DB.Model(&User{})
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		db = db.Where("username LIKE ? OR real_name LIKE ?", kw, kw)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 10
	}
	var users []User
	if err := db.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// CreateUser 创建用户
func CreateUser(user *User) error {
	return DB.Create(user).Error
}

// UpdateUser 更新用户（零值字段不更新）
func UpdateUser(user *User, fields map[string]interface{}) error {
	return DB.Model(user).Updates(fields).Error
}
