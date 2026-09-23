package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// 消耗原因固定字典
var ConsumeReasons = []string{"正常使用", "损坏报废", "过期处理", "测试消耗", "其他"}

// ConsumeRecord 消耗记录表模型（映射 tbl_consume_records，一次消耗操作一条记录）
type ConsumeRecord struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	ConsumeNo          string    `gorm:"type:varchar(50);not null;uniqueIndex" json:"consumeNo"` // 消耗单号（自动生成 CO+日期+序号）
	ProductID          uint      `gorm:"not null;index" json:"productId"`
	ProductName        string    `gorm:"type:varchar(100);not null" json:"productName"` // 商品名称快照
	ProductSN          string    `gorm:"type:varchar(50);not null" json:"productSn"`    // SN 快照
	Quantity           int       `gorm:"not null" json:"quantity"`                      // 消耗数量
	Reason             string    `gorm:"type:varchar(50)" json:"reason"`                // 消耗原因
	Remark             string    `gorm:"type:varchar(500)" json:"remark"`
	ConsumerID         uint      `json:"consumerId"`                           // 消耗人，关联 tbl_users.id
	ConsumerName       string    `gorm:"type:varchar(50)" json:"consumerName"` // 消耗人展示名（快照）
	ConsumerDepartment string    `gorm:"type:varchar(50)" json:"consumerDepartment"` // 消耗人部门（快照）
	CreatedAt          time.Time `json:"createdAt"`                                  // 消耗时间
	UpdatedAt          time.Time `json:"updatedAt"`
}

// consumeNoPrefix 指定日期的消耗单号前缀：CO + yyyyMMdd
func consumeNoPrefix(t time.Time) string {
	return "CO" + t.Format("20060102")
}

// nextConsumeNo 查询指定前缀下的最大消耗单号，返回序号 +1 的下一个候选
func nextConsumeNo(db *gorm.DB, prefix string) (string, error) {
	var rows []ConsumeRecord
	if err := db.Select("id", "consume_no").Where("consume_no LIKE ?", prefix+"%").
		Order("consume_no DESC").Limit(1).Find(&rows).Error; err != nil {
		return "", err
	}
	seq := 1
	if len(rows) == 1 && len(rows[0].ConsumeNo) > len(prefix) {
		if n, err := strconv.Atoi(rows[0].ConsumeNo[len(prefix):]); err == nil {
			seq = n + 1
		}
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// GenerateConsumeNo 生成当日候选消耗单号（CO + yyyyMMdd + 3 位序号），供表单预填
func GenerateConsumeNo() (string, error) {
	return nextConsumeNo(DB, consumeNoPrefix(time.Now()))
}

// CreateConsumeRecord 创建消耗记录：事务内行锁锁定商品，校验在库数量后
// 同时扣减入库数量（当前库存）与在库数量；消耗单号冲突（唯一索引 1062）时重试，最多 3 次
func CreateConsumeRecord(record *ConsumeRecord) error {
	const maxRetry = 3
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		err := DB.Transaction(func(tx *gorm.DB) error {
			// 行锁锁定商品行，防止并发消耗超扣
			var product Product
			if err := tx.Raw("SELECT * FROM tbl_products WHERE id = ? AND deleted_at IS NULL FOR UPDATE", record.ProductID).
				Scan(&product).Error; err != nil {
				return err
			}
			if product.ID == 0 {
				return errors.New("商品不存在")
			}
			if product.Status != ProductStatusInStock {
				return fmt.Errorf("「%s」当前不在库，无法消耗", product.Name)
			}
			if record.Quantity < 1 || record.Quantity > product.InStockQuantity {
				return fmt.Errorf("「%s」消耗数量需在 1 ~ %d 之间（在库 %d）", product.Name, product.InStockQuantity, product.InStockQuantity)
			}

			no, err := nextConsumeNo(tx, consumeNoPrefix(time.Now()))
			if err != nil {
				return err
			}
			record.ConsumeNo = no
			record.ProductName = product.Name
			record.ProductSN = product.SN
			if record.CreatedAt.IsZero() {
				record.CreatedAt = time.Now()
			}
			if err := tx.Create(record).Error; err != nil {
				return err
			}

			// 消耗直接从库存扣除：当前库存（入库数量）与在库数量同步减少
			if err := tx.Model(&Product{ID: product.ID}).Updates(map[string]interface{}{
				"quantity":          gorm.Expr("quantity - ?", record.Quantity),
				"in_stock_quantity": gorm.Expr("in_stock_quantity - ?", record.Quantity),
			}).Error; err != nil {
				return err
			}
			return nil
		})
		if err == nil {
			return nil
		}
		lastErr = err
		// 并发下唯一索引冲突则重试，其余错误（如在库数量不足）直接返回
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			continue
		}
		return err
	}
	return fmt.Errorf("消耗单号生成冲突，请重试（%v）", lastErr)
}

// ConsumableProduct 可消耗商品行（在库且有库存），供消耗页搜索与选中回显
type ConsumableProduct struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	SN            string `json:"sn"`
	SKU           string `json:"sku"`
	InStockQty    int    `json:"inStockQty"`
	WarehouseName string `json:"warehouseName"` // 所在仓库（查询时填充）
	LocationName  string `json:"locationName"`  // 所在仓位（查询时填充）
}

// SearchConsumableProducts 按 SN 码模糊搜索在库且有库存的商品
func SearchConsumableProducts(keyword string, limit int) ([]ConsumableProduct, error) {
	db := DB.Model(&Product{}).
		Select("id, name, sn, sku, in_stock_quantity AS in_stock_qty").
		Where("status = ? AND deleted_at IS NULL", ProductStatusInStock).
		Where("in_stock_quantity > 0")
	if kw := strings.TrimSpace(keyword); kw != "" {
		db = db.Where("sn LIKE ?", "%"+kw+"%")
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	var products []ConsumableProduct
	if err := db.Order("id DESC").Limit(limit).Scan(&products).Error; err != nil {
		return nil, err
	}
	fillConsumableLocation(DB, products)
	return products, nil
}

// GetConsumableProduct 按 ID 查询单个可消耗商品行（校验失败回显用，不过滤库存）
func GetConsumableProduct(id uint) (*ConsumableProduct, error) {
	var products []ConsumableProduct
	if err := DB.Model(&Product{}).
		Select("id, name, sn, sku, in_stock_quantity AS in_stock_qty").
		Where("id = ? AND deleted_at IS NULL", id).
		Scan(&products).Error; err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, errors.New("商品不存在")
	}
	fillConsumableLocation(DB, products)
	return &products[0], nil
}

// fillConsumableLocation 批量填充可消耗商品的仓库与仓位名称（查询时计算，不落库）
func fillConsumableLocation(db *gorm.DB, products []ConsumableProduct) {
	if len(products) == 0 {
		return
	}
	ids := make([]uint, 0, len(products))
	for i := range products {
		ids = append(ids, products[i].ID)
	}
	var rows []Product
	if err := db.Select("id", "warehouse_id", "location_id").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return
	}
	warehouseByID := make(map[uint]Product, len(rows))
	warehouseIDs := make([]uint, 0, len(rows))
	locationIDs := make([]uint, 0, len(rows))
	for _, p := range rows {
		warehouseByID[p.ID] = p
		warehouseIDs = appendUniqueID(warehouseIDs, p.WarehouseID)
		if p.LocationID != nil {
			locationIDs = appendUniqueID(locationIDs, *p.LocationID)
		}
	}
	warehouseName := make(map[uint]string)
	if len(warehouseIDs) > 0 {
		var warehouses []Warehouse
		if err := db.Where("id IN ?", warehouseIDs).Find(&warehouses).Error; err == nil {
			for _, w := range warehouses {
				warehouseName[w.ID] = w.Name
			}
		}
	}
	locationName := make(map[uint]string)
	if len(locationIDs) > 0 {
		var locations []Location
		if err := db.Where("id IN ?", locationIDs).Find(&locations).Error; err == nil {
			for _, l := range locations {
				locationName[l.ID] = l.Name
			}
		}
	}
	for i := range products {
		p, ok := warehouseByID[products[i].ID]
		if !ok {
			continue
		}
		products[i].WarehouseName = warehouseName[p.WarehouseID]
		if p.LocationID != nil {
			products[i].LocationName = locationName[*p.LocationID]
		}
	}
}
