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

// 借用单状态（不设 gorm default 标签，值由代码显式赋值）
const (
	BorrowOrderStatusCancelled = 0 // 已取消（预留）
	BorrowOrderStatusActive    = 1 // 借用中
	BorrowOrderStatusReturned  = 2 // 已归还
)

// BorrowOrder 借用单主表模型（映射 tbl_borrow_orders）
type BorrowOrder struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	BorrowNo         string     `gorm:"type:varchar(50);not null;uniqueIndex" json:"borrowNo"` // 借用单号（自动生成）
	BorrowerID       uint       `gorm:"not null;index" json:"borrowerId"`                      // 借用人（tbl_users.id）
	BorrowerName     string     `gorm:"type:varchar(50);not null" json:"borrowerName"`         // 借用人展示名（借用时快照）
	Department       string     `gorm:"type:varchar(50)" json:"department"`                    // 借用人部门，可空
	BorrowDate       time.Time  `gorm:"not null" json:"borrowDate"`                            // 借用时间（系统自动）
	Days             int        `gorm:"not null" json:"days"`                                  // 借用天数（1~365）
	ExpectReturnDate time.Time  `gorm:"type:date;not null" json:"expectReturnDate"`            // 预计归还时间 = 借用时间 + 天数
	ActualReturnDate *time.Time `json:"actualReturnDate"`                                      // 实际归还时间
	Status           int        `json:"status"`                                                // 1 借用中 / 2 已归还 / 0 已取消（预留）
	Remark           string     `gorm:"type:varchar(500)" json:"remark"`

	// 展示字段：查询时填充，不落库
	Items         []BorrowItem `gorm:"-" json:"items"` // 明细（详情页填充）
	ItemCount     int          `gorm:"-" json:"itemCount"`
	TotalQuantity int          `gorm:"-" json:"totalQuantity"`
	PendingQty    int          `gorm:"-" json:"pendingQty"` // 未归还数量合计
	Overdue       bool         `gorm:"-" json:"overdue"`    // 借用中且已过预计归还时间

	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BorrowItem 借用明细表模型（映射 tbl_borrow_items）
type BorrowItem struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	OrderID          uint   `gorm:"not null;index" json:"orderId"`
	ProductID        uint   `gorm:"not null;index" json:"productId"`
	ProductName      string `gorm:"type:varchar(100);not null" json:"productName"` // 商品名称快照
	ProductSN        string `gorm:"type:varchar(50);not null" json:"productSn"`    // SN 快照
	Quantity         int    `gorm:"not null" json:"quantity"`                      // 借用数量
	ReturnedQuantity int    `json:"returnedQuantity"`                              // 已归还数量（列有 DEFAULT 0，零值可安全 Create）

	PendingQuantity int    `gorm:"-" json:"pendingQuantity"` // 未归还数量（查询时填充）
	WarehouseName   string `gorm:"-" json:"warehouseName"`   // 商品当前所在仓库（查询时填充）
	LocationName    string `gorm:"-" json:"locationName"`    // 商品当前所在仓位（查询时填充）

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BorrowItemInput 新建借用单的单行输入（已解析为数值）
type BorrowItemInput struct {
	ProductID uint
	Quantity  int
}

// borrowNoPrefix 指定日期的借用单号前缀：BR + yyyyMMdd
func borrowNoPrefix(t time.Time) string {
	return "BR" + t.Format("20060102")
}

// nextBorrowNo 查询指定前缀下的最大单号，返回序号 +1 的下一个候选
func nextBorrowNo(db *gorm.DB, prefix string) (string, error) {
	var rows []BorrowOrder
	if err := db.Select("id", "borrow_no").Where("borrow_no LIKE ?", prefix+"%").
		Order("borrow_no DESC").Limit(1).Find(&rows).Error; err != nil {
		return "", err
	}
	seq := 1
	if len(rows) == 1 && len(rows[0].BorrowNo) > len(prefix) {
		if n, err := strconv.Atoi(rows[0].BorrowNo[len(prefix):]); err == nil {
			seq = n + 1
		}
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// GenerateBorrowNo 生成当日候选借用单号（BR + yyyyMMdd + 3 位序号），供表单预填与接口调用
func GenerateBorrowNo() (string, error) {
	return nextBorrowNo(DB, borrowNoPrefix(time.Now()))
}

// mergeBorrowInputs 同一商品多行合并求和（保持首次出现顺序）
func mergeBorrowInputs(inputs []BorrowItemInput) []BorrowItemInput {
	merged := make([]BorrowItemInput, 0, len(inputs))
	index := make(map[uint]int, len(inputs))
	for _, in := range inputs {
		if j, ok := index[in.ProductID]; ok {
			merged[j].Quantity += in.Quantity
			continue
		}
		index[in.ProductID] = len(merged)
		merged = append(merged, in)
	}
	return merged
}

// activeBorrowedQtyFor 单个商品在借用中单据里的未归还数量合计
func activeBorrowedQtyFor(db *gorm.DB, productID uint) int {
	var qty int
	db.Table("tbl_borrow_items AS i").
		Select("COALESCE(SUM(i.quantity - i.returned_quantity), 0)").
		Joins("JOIN tbl_borrow_orders AS o ON o.id = i.order_id AND o.deleted_at IS NULL").
		Where("o.status = ? AND i.product_id = ?", BorrowOrderStatusActive, productID).
		Scan(&qty)
	return qty
}

// ActiveBorrowedQtyMap 批量查询多个商品的未归还借用数量，返回 productID -> 数量
func ActiveBorrowedQtyMap(db *gorm.DB, productIDs []uint) map[uint]int {
	result := make(map[uint]int, len(productIDs))
	if len(productIDs) == 0 {
		return result
	}
	type row struct {
		ProductID uint
		Qty       int
	}
	var rows []row
	db.Table("tbl_borrow_items AS i").
		Select("i.product_id AS product_id, COALESCE(SUM(i.quantity - i.returned_quantity), 0) AS qty").
		Joins("JOIN tbl_borrow_orders AS o ON o.id = i.order_id AND o.deleted_at IS NULL").
		Where("o.status = ? AND i.product_id IN ?", BorrowOrderStatusActive, productIDs).
		Group("i.product_id").
		Scan(&rows)
	for _, r := range rows {
		result[r.ProductID] = r.Qty
	}
	return result
}

// CreateBorrowOrder 创建借用单：同商品多行先合并，事务内逐行锁定商品并校验可借数量，
// 借用单号冲突（唯一索引 1062）时事务内重取序号重试，最多 3 次
func CreateBorrowOrder(order *BorrowOrder, inputs []BorrowItemInput) error {
	merged := mergeBorrowInputs(inputs)
	const maxRetry = 3
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		err := DB.Transaction(func(tx *gorm.DB) error {
			no, err := nextBorrowNo(tx, borrowNoPrefix(order.BorrowDate))
			if err != nil {
				return err
			}
			order.BorrowNo = no

			items := make([]BorrowItem, 0, len(merged))
			for _, in := range merged {
				// 行锁锁定商品行，防止并发借用超卖（Scan 不带行时主键为 0）
				var product Product
				if err := tx.Raw("SELECT * FROM tbl_products WHERE id = ? AND deleted_at IS NULL FOR UPDATE", in.ProductID).
					Scan(&product).Error; err != nil {
					return err
				}
				if product.ID == 0 {
					return errors.New("所选商品不存在或已被删除")
				}
				if product.Status != ProductStatusInStock {
					return fmt.Errorf("「%s」当前不在库，无法借用", product.Name)
				}
				available := product.Quantity - activeBorrowedQtyFor(tx, product.ID)
				if available < 0 {
					available = 0
				}
				if in.Quantity > available {
					return fmt.Errorf("「%s」借用数量超过可借数量（可借 %d）", product.Name, available)
				}
				items = append(items, BorrowItem{
					ProductID:   product.ID,
					ProductName: product.Name,
					ProductSN:   product.SN,
					Quantity:    in.Quantity,
				})
			}

			order.Status = BorrowOrderStatusActive
			if err := tx.Create(order).Error; err != nil {
				return err
			}
			for i := range items {
				items[i].OrderID = order.ID
			}
			return tx.Create(&items).Error
		})
		if err == nil {
			return nil
		}
		lastErr = err
		// 并发下唯一索引冲突则重试，其余错误（如可借数量不足）直接返回
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			continue
		}
		return err
	}
	return fmt.Errorf("借用单号生成冲突，请重试（%v）", lastErr)
}

// fillBorrowDisplay 批量填充单据的明细汇总与逾期标记（查询时计算，不落库）
func fillBorrowDisplay(db *gorm.DB, orders []BorrowOrder) {
	if len(orders) == 0 {
		return
	}
	orderIDs := make([]uint, 0, len(orders))
	for i := range orders {
		orderIDs = append(orderIDs, orders[i].ID)
	}
	var items []BorrowItem
	if err := db.Where("order_id IN ?", orderIDs).Order("id ASC").Find(&items).Error; err != nil {
		return
	}

	// 明细行展示商品当前所在仓库与仓位（快照只存了名称与 SN，仓库仓位取当前值）
	productIDs := make([]uint, 0, len(items))
	for _, item := range items {
		productIDs = appendUniqueID(productIDs, item.ProductID)
	}
	productWarehouse := make(map[uint]uint, len(productIDs))
	productLocation := make(map[uint]uint, len(productIDs))
	warehouseName := make(map[uint]string)
	locationName := make(map[uint]string)
	if len(productIDs) > 0 {
		var products []Product
		if err := db.Select("id", "warehouse_id", "location_id").Where("id IN ?", productIDs).Find(&products).Error; err == nil {
			warehouseIDs := make([]uint, 0, len(products))
			locationIDs := make([]uint, 0, len(products))
			for _, p := range products {
				productWarehouse[p.ID] = p.WarehouseID
				warehouseIDs = appendUniqueID(warehouseIDs, p.WarehouseID)
				if p.LocationID != nil {
					productLocation[p.ID] = *p.LocationID
					locationIDs = appendUniqueID(locationIDs, *p.LocationID)
				}
			}
			var warehouses []Warehouse
			if err := db.Where("id IN ?", warehouseIDs).Find(&warehouses).Error; err == nil {
				for _, w := range warehouses {
					warehouseName[w.ID] = w.Name
				}
			}
			var locations []Location
			if err := db.Where("id IN ?", locationIDs).Find(&locations).Error; err == nil {
				for _, l := range locations {
					locationName[l.ID] = l.Name
				}
			}
		}
	}

	today := time.Now()
	for i := range orders {
		order := &orders[i]
		order.Items = make([]BorrowItem, 0, 4)
		for _, item := range items {
			if item.OrderID != order.ID {
				continue
			}
			item.PendingQuantity = item.Quantity - item.ReturnedQuantity
			item.WarehouseName = warehouseName[productWarehouse[item.ProductID]]
			item.LocationName = locationName[productLocation[item.ProductID]]
			order.Items = append(order.Items, item)
			order.ItemCount++
			order.TotalQuantity += item.Quantity
			order.PendingQty += item.PendingQuantity
		}
		order.Overdue = order.Status == BorrowOrderStatusActive &&
			!order.ExpectReturnDate.IsZero() && today.After(order.ExpectReturnDate)
	}
}

// BorrowQuery 借用单列表查询条件
type BorrowQuery struct {
	Keyword  string // 借用单号/借用人模糊搜索
	Status   int    // 状态精确筛选，0 表示全部
	Page     int    // 页码，从 1 开始
	PageSize int    // 每页条数
}

// ListBorrowOrders 分页查询借用单列表（含明细汇总填充）
func ListBorrowOrders(q BorrowQuery) ([]BorrowOrder, int64, error) {
	db := DB.Model(&BorrowOrder{})
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		db = db.Where("borrow_no LIKE ? OR borrower_name LIKE ?", kw, kw)
	}
	if q.Status != 0 {
		db = db.Where("status = ?", q.Status)
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
	var orders []BorrowOrder
	if err := db.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	fillBorrowDisplay(DB, orders)
	return orders, total, nil
}

// GetBorrowByID 根据 ID 查询借用单（含明细与汇总填充）
func GetBorrowByID(id uint) (*BorrowOrder, error) {
	var order BorrowOrder
	if err := DB.First(&order, id).Error; err != nil {
		return nil, err
	}
	orders := []BorrowOrder{order}
	fillBorrowDisplay(DB, orders)
	return &orders[0], nil
}

// BorrowProductNames 返回借用单明细的商品名称快照（按明细顺序），供操作日志记录
func BorrowProductNames(orderID uint) []string {
	var items []BorrowItem
	if err := DB.Select("product_name").Where("order_id = ?", orderID).Order("id ASC").Find(&items).Error; err != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.ProductName)
	}
	return names
}

// SearchBorrowableProducts 按商品名称/SKU 模糊搜索在库商品（新建借用搜索预览），含可借数量
func SearchBorrowableProducts(keyword string, limit int) ([]Product, error) {
	db := DB.Where("status = ?", ProductStatusInStock)
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR sku LIKE ?", like, like)
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	var products []Product
	if err := db.Order("id DESC").Limit(limit).Find(&products).Error; err != nil {
		return nil, err
	}
	fillProductDisplay(DB, products)
	return products, nil
}
