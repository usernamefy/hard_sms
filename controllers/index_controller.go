package controllers

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"sms/models"
)

type IndexController struct{}

// IndexCtl 首页控制器实例
var IndexCtl = &IndexController{}

// Home 后台管理首页
func (c *IndexController) Home(ctx *gin.Context) {
	session := sessions.Default(ctx)
	// 统计与最近操作：查失败按零值展示，不影响页面
	borrowingQty, err := models.ActiveBorrowedTotal()
	if err != nil {
		borrowingQty = 0
	}
	productCount, _ := models.CountProducts()
	warehouseCount, _ := models.CountWarehouses()
	recentLogs, _ := models.ListRecentOperationLogs(10)

	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"title":          "主页 - 库存管理系统",
		"username":       session.Get("username"),
		"realname":       session.Get("realname"),
		"productCount":   productCount,
		"warehouseCount": warehouseCount,
		"borrowingQty":   borrowingQty,
		"recentLogs":     recentLogs,
	})
}
