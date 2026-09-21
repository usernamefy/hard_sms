package controllers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"sms/models"
)

type OperationLogController struct{}

// OperationLogCtl 操作日志控制器实例
var OperationLogCtl = &OperationLogController{}

// recordOperation 记录操作日志；失败只写应用日志，不影响主流程
func recordOperation(ctx *gin.Context, module, action, detail, productName string) {
	session := sessions.Default(ctx)
	uid, _ := session.Get("user_id").(uint)
	name := ""
	if realname, ok := session.Get("realname").(string); ok && realname != "" {
		name = realname
	} else if username, ok := session.Get("username").(string); ok && username != "" {
		name = username
	}
	if err := models.CreateOperationLog(&models.OperationLog{
		UserID:      uid,
		UserName:    name,
		Module:      module,
		Action:      action,
		ProductName: productName,
		Detail:      detail,
	}); err != nil {
		log.Printf("记录操作日志失败 module=%s action=%s: %v", module, action, err)
	}
}

// productNamesSummary 商品名称汇总（日志快照）：最多取前 3 个，更多时以"等N种商品"结尾
func productNamesSummary(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) <= 3 {
		return strings.Join(names, "、")
	}
	return strings.Join(names[:3], "、") + " 等" + strconv.Itoa(len(names)) + "种商品"
}

// OperationLogActionClass 操作类型 → 标签样式（模板函数）
func OperationLogActionClass(action string) string {
	switch action {
	case "商品入库":
		return "tag-success"
	case "借用":
		return "tag-warning"
	case "归还":
		return "tag-info"
	default:
		return "tag-info"
	}
}

// List 操作日志列表页（page / keyword / action）
func (c *OperationLogController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	logs, total, err := models.ListOperationLogs(models.OperationLogQuery{
		Keyword:  ctx.Query("keyword"),
		Action:   ctx.Query("action"),
		Page:     page,
		PageSize: 10,
	})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询操作日志失败"})
		return
	}

	pageSize := 10
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	ctx.HTML(http.StatusOK, "log_list.html", userPageData(ctx, gin.H{
		"title":      "操作日志 - 库存管理系统",
		"logs":       logs,
		"keyword":    ctx.Query("keyword"),
		"action":     ctx.Query("action"),
		"actions":    []string{"商品入库", "借用", "归还"},
		"page":       page,
		"total":      total,
		"totalPages": totalPages,
	}))
}
