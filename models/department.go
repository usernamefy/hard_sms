package models

import (
	"time"

	"gorm.io/gorm"
)

// Department 部门表模型（映射 tbl_departments，用户所属部门）
type Department struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"type:varchar(50);not null;uniqueIndex" json:"name"` // 部门名称
	Status    int            `json:"status"`                                            // 状态：1 启用 0 禁用（不设 gorm 默认值，避免 Create 时零值被省略）
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ListEnabledDepartments 查询全部启用状态的部门（用户表单下拉用）
func ListEnabledDepartments() ([]Department, error) {
	var departments []Department
	err := DB.Where("status = ?", 1).Order("id ASC").Find(&departments).Error
	return departments, err
}

// GetDepartmentByID 根据 ID 查询部门
func GetDepartmentByID(id uint) (*Department, error) {
	var department Department
	if err := DB.First(&department, id).Error; err != nil {
		return nil, err
	}
	return &department, nil
}

// fillUserDepartmentNames 批量填充用户的部门名称（查询时计算，不落库）
func fillUserDepartmentNames(db *gorm.DB, users []User) {
	ids := make([]uint, 0, len(users))
	for i := range users {
		if users[i].DepartmentID != 0 {
			ids = appendUniqueID(ids, users[i].DepartmentID)
		}
	}
	if len(ids) == 0 {
		return
	}
	var departments []Department
	if err := db.Where("id IN ?", ids).Find(&departments).Error; err != nil {
		return
	}
	names := make(map[uint]string, len(departments))
	for _, d := range departments {
		names[d.ID] = d.Name
	}
	for i := range users {
		users[i].DepartmentName = names[users[i].DepartmentID]
	}
}

// EnsureDepartments 初始部门数据（幂等）：技术部、销售部、运营部
func EnsureDepartments() error {
	seed := []string{"技术部", "销售部", "运营部"}
	for _, name := range seed {
		var count int64
		if err := DB.Model(&Department{}).Where("name = ?", name).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := DB.Create(&Department{Name: name, Status: 1}).Error; err != nil {
			return err
		}
	}
	return nil
}
