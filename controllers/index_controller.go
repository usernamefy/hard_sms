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
	// 借用中 = 借用中单据的未归还数量合计（查询失败按 0 展示）
	borrowingQty, err := models.ActiveBorrowedTotal()
	if err != nil {
		borrowingQty = 0
	}
	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"title":        "主页 - 库存管理系统",
		"username":     session.Get("username"),
		"realname":     session.Get("realname"),
		"borrowingQty": borrowingQty,
	})
}
