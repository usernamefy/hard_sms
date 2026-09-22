package models

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// ReturnOrder 归还单模型（映射 tbl_return_orders，一次归还操作一条记录）
type ReturnOrder struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	ReturnNo           string    `gorm:"type:varchar(50);not null;uniqueIndex" json:"returnNo"` // 归还单号（自动生成 RT+日期+序号）
	BorrowOrderID      uint      `gorm:"not null;index" json:"borrowOrderId"`                   // 借用单，关联 tbl_borrow_orders.id
	BorrowItemID       uint      `gorm:"not null;index" json:"borrowItemId"`                    // 借用明细，关联 tbl_borrow_items.id
	ProductID          uint      `gorm:"not null" json:"productId"`
	ProductName        string    `gorm:"type:varchar(100);not null" json:"productName"` // 商品名称快照
	ProductSN          string    `gorm:"type:varchar(50);not null" json:"productSn"`    // SN 快照
	Quantity           int       `gorm:"not null" json:"quantity"`                      // 归还数量
	IsLost             int       `json:"isLost"`                                        // 是否商品丢失：1 是 0 否（列 DEFAULT 0，零值可安全 Create）
	CompensationAmount *float64  `gorm:"type:decimal(10,2)" json:"compensationAmount"`  // 赔偿金额（丢失时填写）
	Remark             string    `gorm:"type:varchar(500)" json:"remark"`
	ReturnedByID       uint      `json:"returnedById"`                             // 归还操作人，关联 tbl_users.id
	ReturnedByName     string    `gorm:"type:varchar(50)" json:"returnedByName"`   // 归还操作人展示名（快照）
	CreatedAt          time.Time `json:"createdAt"`                                // 归还时间
	UpdatedAt          time.Time `json:"updatedAt"`
}

// returnNoPrefix 指定日期的归还单号前缀：RT + yyyyMMdd
func returnNoPrefix(t time.Time) string {
	return "RT" + t.Format("20060102")
}

// nextReturnNo 查询指定前缀下的最大归还单号，返回序号 +1 的下一个候选
func nextReturnNo(db *gorm.DB, prefix string) (string, error) {
	var rows []ReturnOrder
	if err := db.Select("id", "return_no").Where("return_no LIKE ?", prefix+"%").
		Order("return_no DESC").Limit(1).Find(&rows).Error; err != nil {
		return "", err
	}
	seq := 1
	if len(rows) == 1 && len(rows[0].ReturnNo) > len(prefix) {
		if n, err := strconv.Atoi(rows[0].ReturnNo[len(prefix):]); err == nil {
			seq = n + 1
		}
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// GenerateReturnNo 生成当日候选归还单号（RT + yyyyMMdd + 3 位序号），供表单预填
func GenerateReturnNo() (string, error) {
	return nextReturnNo(DB, returnNoPrefix(time.Now()))
}

// CreateReturnOrder 创建归还单：事务内行锁锁定借用明细及其借用单，
// 校验归还数量后累加明细 returned_quantity，全部还清时借用单自动转为已归还；
// 归还单号冲突（唯一索引 1062）时事务内重取序号重试，最多 3 次
func CreateReturnOrder(order *ReturnOrder) error {
	const maxRetry = 3
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		err := DB.Transaction(func(tx *gorm.DB) error {
			// 行锁锁定借用明细
			var item BorrowItem
			if err := tx.Raw("SELECT * FROM tbl_borrow_items WHERE id = ? FOR UPDATE", order.BorrowItemID).
				Scan(&item).Error; err != nil {
				return err
			}
			if item.ID == 0 {
				return errors.New("借用明细不存在")
			}
			// 行锁锁定所属借用单，防止并发归还重复累计
			var borrow BorrowOrder
			if err := tx.Raw("SELECT * FROM tbl_borrow_orders WHERE id = ? AND deleted_at IS NULL FOR UPDATE", item.OrderID).
				Scan(&borrow).Error; err != nil {
				return err
			}
			if borrow.ID == 0 {
				return errors.New("借用单不存在")
			}
			if borrow.Status != BorrowOrderStatusActive {
				return fmt.Errorf("借用单「%s」当前状态不可归还", borrow.BorrowNo)
			}

			pending := item.Quantity - item.ReturnedQuantity
			if order.Quantity < 1 || order.Quantity > pending {
				return fmt.Errorf("归还数量需在 1 ~ %d 之间（可归还 %d）", pending, pending)
			}

			no, err := nextReturnNo(tx, returnNoPrefix(time.Now()))
			if err != nil {
				return err
			}
			order.ReturnNo = no
			order.BorrowOrderID = borrow.ID
			order.ProductID = item.ProductID
			order.ProductName = item.ProductName
			order.ProductSN = item.ProductSN
			if order.CreatedAt.IsZero() {
				order.CreatedAt = time.Now()
			}
			if err := tx.Create(order).Error; err != nil {
				return err
			}

			// 累加明细已归还数量
			if err := tx.Model(&BorrowItem{ID: item.ID}).
				Update("returned_quantity", gorm.Expr("returned_quantity + ?", order.Quantity)).Error; err != nil {
				return err
			}

			// 回补商品在库数量（行锁防并发，校验不超出入库数量）
			var product Product
			if err := tx.Raw("SELECT id, name, quantity, in_stock_quantity FROM tbl_products WHERE id = ? FOR UPDATE", item.ProductID).
				Scan(&product).Error; err != nil {
				return err
			}
			if product.ID != 0 {
				if product.InStockQuantity+order.Quantity > product.Quantity {
					return fmt.Errorf("「%s」在库数量将超出入库数量，请核对库存", product.Name)
				}
				if err := tx.Model(&Product{ID: product.ID}).
					Update("in_stock_quantity", gorm.Expr("in_stock_quantity + ?", order.Quantity)).Error; err != nil {
					return err
				}
			}

			// 全部还清则借用单转为已归还
			var remain int
			if err := tx.Model(&BorrowItem{}).Where("order_id = ?", borrow.ID).
				Select("COALESCE(SUM(quantity - returned_quantity), 0)").Scan(&remain).Error; err != nil {
				return err
			}
			if remain <= 0 {
				return tx.Model(&BorrowOrder{ID: borrow.ID}).Updates(map[string]interface{}{
					"status":             BorrowOrderStatusReturned,
					"actual_return_date": order.CreatedAt,
				}).Error
			}
			return nil
		})
		if err == nil {
			return nil
		}
		lastErr = err
		// 并发下唯一索引冲突则重试，其余错误（如可归还数量不足）直接返回
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			continue
		}
		return err
	}
	return fmt.Errorf("归还单号生成冲突，请重试（%v）", lastErr)
}
