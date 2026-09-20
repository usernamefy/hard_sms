package models

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// 商品状态（不设 gorm default 标签，值由代码显式赋值）
const (
	ProductStatusShelved  = 0 // 已出库/报废（预留）
	ProductStatusInStock  = 1 // 在库（创建入库后默认）
	ProductStatusBorrowed = 2 // 已借出（预留，借用模块使用）
)

// ProductCategories 一级分类固定字典
var ProductCategories = []string{"交通工具", "电子设备", "办公设备", "其他"}

// IsValidProductCategory 判断一级分类是否在固定字典内
func IsValidProductCategory(category string) bool {
	for _, c := range ProductCategories {
		if c == category {
			return true
		}
	}
	return false
}

// Product 商品表模型（映射 tbl_products，一台实物一条记录，SN 全局唯一）
type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`          // 商品名称
	SN          string    `gorm:"type:varchar(50);not null;uniqueIndex" json:"sn"` // SN 码（自动生成，全局唯一）
	SKU         string    `gorm:"type:varchar(50)" json:"sku"`                     // 可空
	SPU         string    `gorm:"type:varchar(50)" json:"spu"`                     // 标准产品单位，可空
	Category    string    `gorm:"type:varchar(50);not null" json:"category"`       // 一级分类
	SubCategory string    `gorm:"type:varchar(100)" json:"subCategory"`            // 二级分类（自由文本）
	Price       *float64  `gorm:"type:decimal(10,2)" json:"price"`                 // 价格（元），可空
	OwnerName   string    `gorm:"type:varchar(50)" json:"ownerName"`               // 样品归属人
	WarehouseID uint      `gorm:"not null;index" json:"warehouseId"`               // 所在仓库
	LocationID  *uint     `gorm:"index" json:"locationId"`                         // 所在仓位，可空
	InboundDate time.Time `gorm:"type:date;not null" json:"inboundDate"`           // 入库日期（系统自动）
	Quantity    int       `gorm:"not null" json:"quantity"`                      // 入库数量
	Status      int       `json:"status"`                                        // 1 在库 / 2 已借出 / 0 已出库
	Image       string    `gorm:"type:varchar(255)" json:"image"`                  // 商品图片相对路径
	Remark      string    `gorm:"type:varchar(500)" json:"remark"`
	CreatedBy   *uint     `json:"createdBy"` // 创建人（tbl_users.id）

	// 展示字段：查询时填充，不落库
	WarehouseName string `gorm:"-" json:"warehouseName"`
	LocationName  string `gorm:"-" json:"locationName"`
	CreatedByName string `gorm:"-" json:"createdByName"`
	AgeDays       int64  `gorm:"-" json:"ageDays"` // 库龄（天）= 当前日期 - 入库日期

	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// PriceText 表单回显价格原文（未填写返回空串）
func (p *Product) PriceText() string {
	if p == nil || p.Price == nil {
		return ""
	}
	return strconv.FormatFloat(*p.Price, 'f', -1, 64)
}

// snPrefix 指定日期的 SN 前缀：SN + yyyyMMdd
func snPrefix(t time.Time) string {
	return "SN" + t.Format("20060102")
}

// nextSN 查询指定前缀下的最大 SN，返回序号 +1 的下一个候选
func nextSN(db *gorm.DB, prefix string) (string, error) {
	var rows []Product
	if err := db.Select("id", "sn").Where("sn LIKE ?", prefix+"%").
		Order("sn DESC").Limit(1).Find(&rows).Error; err != nil {
		return "", err
	}
	seq := 1
	if len(rows) == 1 && len(rows[0].SN) > len(prefix) {
		if n, err := strconv.Atoi(rows[0].SN[len(prefix):]); err == nil {
			seq = n + 1
		}
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// GenerateProductSN 生成当日候选 SN（SN + yyyyMMdd + 3 位序号），供表单预填与接口调用
func GenerateProductSN() (string, error) {
	return nextSN(DB, snPrefix(time.Now()))
}

// CreateProduct 创建商品入库记录；候选 SN 被占用或并发唯一键冲突时，
// 事务内重取当日序号重试，最多 3 次
func CreateProduct(product *Product) error {
	const maxRetry = 3
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		err := DB.Transaction(func(tx *gorm.DB) error {
			var count int64
			if err := tx.Model(&Product{}).Where("sn = ?", product.SN).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				if len(product.SN) != 13 {
					return errors.New("SN 码格式不正确")
				}
				sn, err := nextSN(tx, product.SN[:10])
				if err != nil {
					return err
				}
				product.SN = sn
			}
			return tx.Create(product).Error
		})
		if err == nil {
			return nil
		}
		lastErr = err
		// 并发下唯一索引冲突则重试，其余错误直接返回
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			continue
		}
		return err
	}
	return fmt.Errorf("SN 码生成冲突，请重试（%v）", lastErr)
}

// GetProductByID 根据 ID 查询商品（含展示字段填充）
func GetProductByID(id uint) (*Product, error) {
	var product Product
	if err := DB.First(&product, id).Error; err != nil {
		return nil, err
	}
	products := []Product{product}
	fillProductDisplay(DB, products)
	return &products[0], nil
}

// ProductQuery 商品列表查询条件
type ProductQuery struct {
	Keyword  string // 商品名称/SN/SKU/SPU 模糊搜索
	Category string // 一级分类精确筛选
	Page     int    // 页码，从 1 开始
	PageSize int    // 每页条数
}

// ListProducts 分页查询商品列表（含展示字段填充）
func ListProducts(q ProductQuery) ([]Product, int64, error) {
	db := DB.Model(&Product{})
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		db = db.Where("name LIKE ? OR sn LIKE ? OR sku LIKE ? OR spu LIKE ?", kw, kw, kw, kw)
	}
	if q.Category != "" {
		db = db.Where("category = ?", q.Category)
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
	var products []Product
	if err := db.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

// UpdateProduct 更新商品指定字段
func UpdateProduct(product *Product, fields map[string]interface{}) error {
	return DB.Model(product).Updates(fields).Error
}

// fillProductDisplay 批量填充仓库/仓位名称、创建人与库龄等展示字段（查询时计算，不落库）
func fillProductDisplay(db *gorm.DB, products []Product) {
	warehouseIDs := make([]uint, 0, len(products))
	locationIDs := make([]uint, 0, len(products))
	userIDs := make([]uint, 0, len(products))
	for i := range products {
		p := &products[i]
		if !p.InboundDate.IsZero() {
			p.AgeDays = int64(time.Since(p.InboundDate).Hours() / 24)
			if p.AgeDays < 0 {
				p.AgeDays = 0
			}
		}
		warehouseIDs = appendUniqueID(warehouseIDs, p.WarehouseID)
		if p.LocationID != nil {
			locationIDs = appendUniqueID(locationIDs, *p.LocationID)
		}
		if p.CreatedBy != nil {
			userIDs = appendUniqueID(userIDs, *p.CreatedBy)
		}
	}

	if len(warehouseIDs) > 0 {
		var warehouses []Warehouse
		if err := db.Where("id IN ?", warehouseIDs).Find(&warehouses).Error; err == nil {
			names := make(map[uint]string, len(warehouses))
			for _, w := range warehouses {
				names[w.ID] = w.Name
			}
			for i := range products {
				products[i].WarehouseName = names[products[i].WarehouseID]
			}
		}
	}
	if len(locationIDs) > 0 {
		var locations []Location
		if err := db.Where("id IN ?", locationIDs).Find(&locations).Error; err == nil {
			names := make(map[uint]string, len(locations))
			for _, l := range locations {
				names[l.ID] = l.Name
			}
			for i := range products {
				if products[i].LocationID != nil {
					products[i].LocationName = names[*products[i].LocationID]
				}
			}
		}
	}
	if len(userIDs) > 0 {
		var users []User
		if err := db.Where("id IN ?", userIDs).Find(&users).Error; err == nil {
			names := make(map[uint]string, len(users))
			for _, u := range users {
				names[u.ID] = u.DisplayName()
			}
			for i := range products {
				if products[i].CreatedBy != nil {
					products[i].CreatedByName = names[*products[i].CreatedBy]
				}
			}
		}
	}
}

// appendUniqueID 去重追加 ID（忽略 0 值）
func appendUniqueID(ids []uint, id uint) []uint {
	if id == 0 {
		return ids
	}
	for _, v := range ids {
		if v == id {
			return ids
		}
	}
	return append(ids, id)
}
