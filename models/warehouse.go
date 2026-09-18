package models

import (
	"time"

	"gorm.io/gorm"
)

// Warehouse 仓库表模型（映射 tbl_warehouses，前缀由 NamingStrategy 全局配置）
type Warehouse struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"type:varchar(100);not null" json:"name"` // 仓库名称
	Code      string `gorm:"type:varchar(50)" json:"code"`           // 仓库编码
	Address   string `gorm:"type:varchar(200)" json:"address"`       // 仓库地址
	ManagerID uint   `json:"managerId"`                              // 管理员 ID，0 表示未指定
	Remark    string `gorm:"type:varchar(200)" json:"remark"`        // 备注
	Status    int    `gorm:"default:1" json:"status"`                // 状态：1 启用 0 禁用

	Manager       *User `gorm:"foreignKey:ManagerID" json:"-"` // 关联的管理员
	LocationCount int64 `gorm:"-" json:"locationCount"`        // 仓位数量（查询时填充）

	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// GetWarehouseByID 根据 ID 查询仓库（带管理员信息）
func GetWarehouseByID(id uint) (*Warehouse, error) {
	var warehouse Warehouse
	if err := DB.Preload("Manager").First(&warehouse, id).Error; err != nil {
		return nil, err
	}
	return &warehouse, nil
}

// ListWarehouses 分页查询仓库列表（带管理员信息），并填充仓位数量
func ListWarehouses(page, pageSize int) ([]Warehouse, int64, error) {
	total, err := CountWarehouses()
	if err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	var warehouses []Warehouse
	if err := DB.Preload("Manager").Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&warehouses).Error; err != nil {
		return nil, 0, err
	}

	if len(warehouses) > 0 {
		ids := make([]uint, 0, len(warehouses))
		for _, w := range warehouses {
			ids = append(ids, w.ID)
		}
		var counts []struct {
			WarehouseID uint
			Cnt         int64
		}
		if err := DB.Model(&Location{}).
			Select("warehouse_id, COUNT(*) AS cnt").
			Where("warehouse_id IN ?", ids).
			Group("warehouse_id").
			Find(&counts).Error; err != nil {
			return nil, 0, err
		}
		for i := range warehouses {
			for _, c := range counts {
				if c.WarehouseID == warehouses[i].ID {
					warehouses[i].LocationCount = c.Cnt
					break
				}
			}
		}
	}
	return warehouses, total, nil
}

// CreateWarehouse 创建仓库
func CreateWarehouse(warehouse *Warehouse) error {
	return DB.Create(warehouse).Error
}

// UpdateWarehouse 更新仓库指定字段
func UpdateWarehouse(warehouse *Warehouse, fields map[string]interface{}) error {
	return DB.Model(warehouse).Updates(fields).Error
}

// CountWarehouses 统计仓库总数
func CountWarehouses() (int64, error) {
	var total int64
	err := DB.Model(&Warehouse{}).Count(&total).Error
	return total, err
}
