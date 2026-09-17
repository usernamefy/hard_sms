package middlewares

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Auth 登录校验中间件：未登录的请求重定向到登录页
func Auth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		if _, ok := session.Get("user_id").(uint); !ok {
			ctx.Redirect(http.StatusFound, "/login")
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
