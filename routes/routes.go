package routes

import (
	"html/template"
	"strconv"
	"strings"
	"time"

	"sms/config"
	"sms/controllers"
	"sms/middlewares"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// fmtTime 格式化时间（nil 显示 -）
func fmtTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

// money 金额展示：￥ + 千分位 + 两位小数（未填写显示 -）
func money(v *float64) string {
	if v == nil {
		return "-"
	}
	s := strconv.FormatFloat(*v, 'f', 2, 64)
	dot := strings.IndexByte(s, '.')
	intPart, frac := s[:dot], s[dot:]
	var buf []byte
	for i := 0; i < len(intPart); i++ {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, intPart[i])
	}
	return "￥" + string(buf) + frac
}

// Setup 创建并注册所有路由
func Setup() *gin.Engine {
	router := gin.Default()

	router.SetFuncMap(template.FuncMap{
		"fmtTime": fmtTime,
		"money":   money,
		"inc":     func(i int) int { return i + 1 },
		"dec":     func(i int) int { return i - 1 },
	})

	// 静态资源
	router.Static("/static", "./static")

	// HTML 模板：views 按模块分子目录（auth/common/home/product/user/warehouse），
	// 模板名仍是文件名（各目录内不重名），ctx.HTML 用法不变
	router.LoadHTMLGlob("views/*/*.html")

	// 基于 Cookie 的会话存储
	store := cookie.NewStore([]byte(config.App.SessionKey))
	store.Options(sessions.Options{
		MaxAge:   config.App.SessionMaxAge,
		Path:     "/",
		HttpOnly: true,
	})
	router.Use(sessions.Sessions(config.App.CookieName, store))

	// 公开路由：登录
	router.GET("/login", controllers.UserCtl.Login)
	router.POST("/login", controllers.UserCtl.DoLogin)

	// 后台路由：需要登录
	admin := router.Group("/", middlewares.Auth())
	{
		admin.GET("/", controllers.IndexCtl.Home)

		// 用户管理
		admin.GET("/users", controllers.UserCtl.List)
		admin.GET("/users/add", controllers.UserCtl.Add)
		admin.POST("/users/add", controllers.UserCtl.DoAdd)
		admin.GET("/users/edit/:id", controllers.UserCtl.Edit)
		admin.POST("/users/edit/:id", controllers.UserCtl.DoEdit)

		// 仓库管理
		admin.GET("/warehouses", controllers.WarehouseCtl.List)
		admin.GET("/warehouses/add", controllers.WarehouseCtl.Add)
		admin.POST("/warehouses/add", controllers.WarehouseCtl.DoAdd)
		admin.GET("/warehouses/detail/:id", controllers.WarehouseCtl.Detail)
		admin.GET("/warehouses/edit/:id", controllers.WarehouseCtl.Edit)
		admin.POST("/warehouses/edit/:id", controllers.WarehouseCtl.DoEdit)

		// 仓位管理：列表直接嵌在仓库详情页，这里只保留新增/编辑/删除
		admin.GET("/warehouses/detail/:id/locations/add", controllers.WarehouseCtl.LocationAdd)
		admin.POST("/warehouses/detail/:id/locations/add", controllers.WarehouseCtl.LocationDoAdd)
		admin.GET("/warehouses/detail/:id/locations/edit/:locId", controllers.WarehouseCtl.LocationEdit)
		admin.POST("/warehouses/detail/:id/locations/edit/:locId", controllers.WarehouseCtl.LocationDoEdit)
		admin.POST("/warehouses/detail/:id/locations/delete/:locId", controllers.WarehouseCtl.LocationDelete)

		// 商品入库
		admin.GET("/products", controllers.ProductCtl.List)
		admin.GET("/products/add", controllers.ProductCtl.Add)
		admin.POST("/products/add", controllers.ProductCtl.DoAdd)
		admin.GET("/products/api/sn", controllers.ProductCtl.GenerateSN)
		admin.GET("/products/edit/:id", controllers.ProductCtl.Edit)
		admin.POST("/products/edit/:id", controllers.ProductCtl.DoEdit)
		admin.GET("/products/:id", controllers.ProductCtl.Detail)

		admin.GET("/logout", controllers.UserCtl.Logout)
	}

	return router
}
