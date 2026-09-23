package models

// InventoryQuery 库存查询条件（各字段为空/0 时不参与筛选）
type InventoryQuery struct {
	Name            string // 商品名称模糊
	SKU             string // SKU 编码模糊
	SN              string // SN 码模糊
	Category        string // 一级分类精确
	OwnerID         uint   // 归属人（tbl_users.id）精确
	OwnerDepartment string // 归属部门名称精确（按归属人用户当前部门匹配）
	WarehouseID     uint   // 仓库精确
	LocationID      uint   // 仓位精确
}

// inventoryResultLimit 库存查询结果上限（样品规模查询，防止全量拉取失控）
const inventoryResultLimit = 500

// ListInventory 库存查询：按条件返回商品（含仓库仓位名称与库存口径字段），不分页
func ListInventory(q InventoryQuery) ([]Product, error) {
	db := DB.Model(&Product{})
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.SKU != "" {
		db = db.Where("sku LIKE ?", "%"+q.SKU+"%")
	}
	if q.SN != "" {
		db = db.Where("sn LIKE ?", "%"+q.SN+"%")
	}
	if q.Category != "" {
		db = db.Where("category = ?", q.Category)
	}
	if q.OwnerID != 0 {
		db = db.Where("owner_id = ?", q.OwnerID)
	}
	if q.OwnerDepartment != "" {
		// 归属部门跟随归属人用户当前部门
		db = db.Where("owner_id IN (SELECT u.id FROM tbl_users u "+
			"JOIN tbl_departments d ON d.id = u.department_id "+
			"WHERE d.name = ? AND u.deleted_at IS NULL)", q.OwnerDepartment)
	}
	if q.WarehouseID != 0 {
		db = db.Where("warehouse_id = ?", q.WarehouseID)
	}
	if q.LocationID != 0 {
		db = db.Where("location_id = ?", q.LocationID)
	}
	var products []Product
	if err := db.Order("id DESC").Limit(inventoryResultLimit).Find(&products).Error; err != nil {
		return nil, err
	}
	fillProductDisplay(DB, products)
	return products, nil
}
