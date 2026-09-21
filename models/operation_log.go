package models

import (
	"time"
)

// OperationLog 操作日志表模型（映射 tbl_operation_logs，日志不可改删）
type OperationLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index" json:"userId"`                     // 操作人（tbl_users.id）
	UserName    string    `gorm:"type:varchar(50)" json:"userName"`        // 操作人展示名（快照）
	Module      string    `gorm:"type:varchar(20);not null" json:"module"` // 模块
	Action      string    `gorm:"type:varchar(20);not null" json:"action"` // 操作类型
	ProductName string    `gorm:"type:varchar(200)" json:"productName"`    // 商品名称（快照，多个以、分隔）
	Detail      string    `gorm:"type:varchar(500)" json:"detail"`         // 操作内容
	CreatedAt   time.Time `json:"createdAt"`
}

// CreateOperationLog 写入一条操作日志
func CreateOperationLog(log *OperationLog) error {
	return DB.Create(log).Error
}

// ListRecentOperationLogs 查询最近 limit 条操作日志（主页"最近操作"）
func ListRecentOperationLogs(limit int) ([]OperationLog, error) {
	if limit < 1 {
		limit = 10
	}
	var logs []OperationLog
	err := DB.Order("id DESC").Limit(limit).Find(&logs).Error
	return logs, err
}

// OperationLogQuery 操作日志列表查询条件
type OperationLogQuery struct {
	Keyword  string // 操作人/操作内容模糊搜索
	Module   string // 模块精确筛选
	Action   string // 操作类型精确筛选
	Page     int
	PageSize int
}

// ListOperationLogs 分页查询操作日志
func ListOperationLogs(q OperationLogQuery) ([]OperationLog, int64, error) {
	db := DB.Model(&OperationLog{})
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		db = db.Where("user_name LIKE ? OR detail LIKE ?", kw, kw)
	}
	if q.Module != "" {
		db = db.Where("module = ?", q.Module)
	}
	if q.Action != "" {
		db = db.Where("action = ?", q.Action)
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
	var logs []OperationLog
	if err := db.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// CountProducts 商品总数（主页统计）
func CountProducts() (int64, error) {
	var total int64
	err := DB.Model(&Product{}).Count(&total).Error
	return total, err
}
