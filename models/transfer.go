package models

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// 样品异动类型与原因字典
const (
	TransferTypeOwner     = 1 // 人员转移
	TransferTypeWarehouse = 2 // 移仓
)

var TransferOwnerReasons = []string{"人员离职", "组织架构调整", "部门合并", "其他"}
var TransferWarehouseReasons = []string{"库存调整", "仓库搬迁", "优化存储", "其他"}

// TransferRecord 样品异动记录表模型（映射 tbl_transfer_records，每个样品一条异动记录）
type TransferRecord struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TransferNo  string    `gorm:"type:varchar(50);not null;uniqueIndex" json:"transferNo"` // 异动单号（自动生成 TR+日期+序号）
	Type        int       `gorm:"not null" json:"type"`                                    // 1 人员转移 / 2 移仓
	ProductID   uint      `gorm:"not null;index" json:"productId"`
	ProductName string    `gorm:"type:varchar(100);not null" json:"productName"` // 商品名称快照
	ProductSN   string    `gorm:"type:varchar(50);not null" json:"productSn"`    // SN 快照
	// 人员转移（移仓时为空）
	FromOwnerID    uint   `gorm:"index" json:"fromOwnerId"`
	FromOwnerName  string `gorm:"type:varchar(50)" json:"fromOwnerName"`
	FromDepartment string `gorm:"type:varchar(50)" json:"fromDepartment"`
	ToOwnerID      uint   `gorm:"index" json:"toOwnerId"`
	ToOwnerName    string `gorm:"type:varchar(50)" json:"toOwnerName"`
	ToDepartment   string `gorm:"type:varchar(50)" json:"toDepartment"`
	// 移仓（人员转移时为空）
	FromWarehouseID   uint   `gorm:"index" json:"fromWarehouseId"`
	FromWarehouseName string `gorm:"type:varchar(100)" json:"fromWarehouseName"`
	FromLocationID    uint   `json:"fromLocationId"` // 0 表示原无仓位
	FromLocationName  string `gorm:"type:varchar(100)" json:"fromLocationName"`
	ToWarehouseID     uint   `gorm:"index" json:"toWarehouseId"`
	ToWarehouseName   string `gorm:"type:varchar(100)" json:"toWarehouseName"`
	ToLocationID      uint   `json:"toLocationId"` // 0 表示无仓位
	ToLocationName    string `gorm:"type:varchar(100)" json:"toLocationName"`
	// 公共
	Reason       string    `gorm:"type:varchar(50)" json:"reason"`
	Remark       string    `gorm:"type:varchar(500)" json:"remark"`
	OperatorID   uint      `json:"operatorId"`
	OperatorName string    `gorm:"type:varchar(50)" json:"operatorName"`
	CreatedAt    time.Time `json:"createdAt"` // 异动时间
	UpdatedAt    time.Time `json:"updatedAt"`
}

// transferNoPrefix 指定日期的异动单号前缀：TR + yyyyMMdd
func transferNoPrefix(t time.Time) string {
	return "TR" + t.Format("20060102")
}

// nextTransferNo 查询指定前缀下的最大异动单号，返回序号 +1 的下一个候选
func nextTransferNo(db *gorm.DB, prefix string) (string, error) {
	var rows []TransferRecord
	if err := db.Select("id", "transfer_no").Where("transfer_no LIKE ?", prefix+"%").
		Order("transfer_no DESC").Limit(1).Find(&rows).Error; err != nil {
		return "", err
	}
	seq := 1
	if len(rows) == 1 && len(rows[0].TransferNo) > len(prefix) {
		if n, err := strconv.Atoi(rows[0].TransferNo[len(prefix):]); err == nil {
			seq = n + 1
		}
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// GenerateTransferNo 生成当日候选异动单号（TR + yyyyMMdd + 3 位序号），供日志与扩展使用
func GenerateTransferNo() (string, error) {
	return nextTransferNo(DB, transferNoPrefix(time.Now()))
}

// TransferSample 样品异动候选样品行（JSON）
type TransferSample struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	SKU           string `json:"sku"`
	SN            string `json:"sn"`
	Quantity      int    `json:"quantity"`
	WarehouseName string `json:"warehouseName"`
	LocationName  string `json:"locationName"`
	InboundDate   string `json:"inboundDate"`
	AgeDays       int64  `json:"ageDays"`
}

// ListTransferSamples 样品异动候选列表：按归属人或仓库（可叠加仓位）过滤
func ListTransferSamples(ownerID, warehouseID, locationID uint) ([]TransferSample, error) {
	db := DB.Model(&Product{})
	if ownerID != 0 {
		db = db.Where("owner_id = ?", ownerID)
	}
	if warehouseID != 0 {
		db = db.Where("warehouse_id = ?", warehouseID)
	}
	if locationID != 0 {
		db = db.Where("location_id = ?", locationID)
	}
	var products []Product
	if err := db.Order("id DESC").Limit(500).Find(&products).Error; err != nil {
		return nil, err
	}
	fillProductDisplay(DB, products)

	samples := make([]TransferSample, 0, len(products))
	for i := range products {
		p := &products[i]
		inbound := ""
		if !p.InboundDate.IsZero() {
			inbound = p.InboundDate.Format("2006-01-02")
		}
		warehouseName := p.WarehouseName
		locationName := "-"
		if p.LocationName != "" {
			locationName = p.LocationName
		}
		samples = append(samples, TransferSample{
			ID:            p.ID,
			Name:          p.Name,
			SKU:           p.SKU,
			SN:            p.SN,
			Quantity:      p.Quantity,
			WarehouseName: warehouseName,
			LocationName:  locationName,
			InboundDate:   inbound,
			AgeDays:       p.AgeDays,
		})
	}
	return samples, nil
}

// TransferProductUpdate 单个样品的异动变更（记录与商品更新一一对应）
type TransferProductUpdate struct {
	ProductID uint
	// 移仓结果位置
	ToWarehouseID   uint
	ToWarehouseName string
	ToLocationID    uint
	ToLocationName  string
}

// CreateTransferRecords 批量创建样品异动：事务内逐单行锁锁定商品并校验前置状态
// （人员转移校验原归属人、移仓校验源仓库），套用变更后写入异动记录；
// 异动单号冲突（唯一索引 1062）时重取序号重试，最多 3 次
func CreateTransferRecords(records []TransferRecord, updates []TransferProductUpdate) error {
	if len(records) == 0 {
		return errors.New("请选择要异动的样品")
	}
	const maxRetry = 3
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		err := DB.Transaction(func(tx *gorm.DB) error {
			prefix := transferNoPrefix(time.Now())
			no, err := nextTransferNo(tx, prefix)
			if err != nil {
				return err
			}
			base, err := strconv.Atoi(no[len(prefix):])
			if err != nil {
				return err
			}
			now := time.Now()
			for idx := range records {
				record := &records[idx]
				// 行锁锁定商品行，防止并发异动
				var product Product
				if err := tx.Raw("SELECT * FROM tbl_products WHERE id = ? AND deleted_at IS NULL FOR UPDATE", record.ProductID).
					Scan(&product).Error; err != nil {
					return err
				}
				if product.ID == 0 {
					return errors.New("部分样品不存在或已被删除，请刷新后重试")
				}

				switch record.Type {
				case TransferTypeOwner:
					// 校验样品仍归属原归属人
					if product.OwnerID == nil || *product.OwnerID != record.FromOwnerID {
						return fmt.Errorf("「%s」的归属人已变更，请刷新后重试", product.Name)
					}
					if err := tx.Model(&Product{ID: product.ID}).Update("owner_id", record.ToOwnerID).Error; err != nil {
						return err
					}
				case TransferTypeWarehouse:
					var upd TransferProductUpdate
					if idx < len(updates) {
						upd = updates[idx]
					}
					// 校验样品仍在源仓库
					if product.WarehouseID != record.FromWarehouseID {
						return fmt.Errorf("「%s」的仓库已变更，请刷新后重试", product.Name)
					}
					if err := tx.Model(&Product{ID: product.ID}).Updates(map[string]interface{}{
						"warehouse_id": upd.ToWarehouseID,
						"location_id":  locationPtr(upd.ToLocationID),
					}).Error; err != nil {
						return err
					}
				default:
					return errors.New("未知的异动类型")
				}

				record.TransferNo = fmt.Sprintf("%s%03d", prefix, base+idx)
				record.ProductName = product.Name
				record.ProductSN = product.SN
				record.CreatedAt = now
				if err := tx.Create(record).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err == nil {
			return nil
		}
		lastErr = err
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			continue
		}
		return err
	}
	return fmt.Errorf("异动单号生成冲突，请重试（%v）", lastErr)
}

// locationPtr 0 → nil（无仓位）
func locationPtr(id uint) *uint {
	if id == 0 {
		return nil
	}
	return &id
}
