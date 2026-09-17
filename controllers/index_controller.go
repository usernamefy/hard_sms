package controllers

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type IndexController struct{}

// IndexCtl 首页控制器实例
var IndexCtl = &IndexController{}

// Home 后台管理首页
func (c *IndexController) Home(ctx *gin.Context) {
	session := sessions.Default(ctx)
	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"title":    "主页 - 库存管理系统",
		"username": session.Get("username"),
		"realname": session.Get("realname"),
	})
}
