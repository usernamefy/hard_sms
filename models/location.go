package models

import (
	"time"

	"gorm.io/gorm"
)

// Location 仓位表模型（映射 tbl_locations，前缀由 NamingStrategy 全局配置）
type Location struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	WarehouseID  uint   `gorm:"not null;index" json:"warehouseId"`      // 所属仓库 ID
	Name         string `gorm:"type:varchar(100);not null" json:"name"` // 仓位名称
	Code         string `gorm:"type:varchar(50)" json:"code"`           // 仓位编码
	CurrentStock int    `gorm:"default:0" json:"currentStock"`          // 当前库存
	Remark       string `gorm:"type:varchar(200)" json:"remark"`        // 备注
	Status       int    `gorm:"default:1" json:"status"`                // 状态：1 启用 0 禁用

	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// GetLocationByID 根据 ID 查询仓位
func GetLocationByID(id uint) (*Location, error) {
	var location Location
	if err := DB.First(&location, id).Error; err != nil {
		return nil, err
	}
	return &location, nil
}

// ListLocationsByWarehouse 查询指定仓库下的全部仓位
func ListLocationsByWarehouse(warehouseID uint) ([]Location, error) {
	var locations []Location
	err := DB.Where("warehouse_id = ?", warehouseID).Order("id ASC").Find(&locations).Error
	return locations, err
}

// CountLocationsByWarehouse 统计指定仓库下的仓位数量
func CountLocationsByWarehouse(warehouseID uint) (int64, error) {
	var total int64
	err := DB.Model(&Location{}).Where("warehouse_id = ?", warehouseID).Count(&total).Error
	return total, err
}

// CreateLocation 创建仓位
func CreateLocation(location *Location) error {
	return DB.Create(location).Error
}

// UpdateLocation 更新仓位指定字段
func UpdateLocation(location *Location, fields map[string]interface{}) error {
	return DB.Model(location).Updates(fields).Error
}

// DeleteLocation 删除仓位（软删除）
func DeleteLocation(id uint) error {
	return DB.Delete(&Location{}, id).Error
}
