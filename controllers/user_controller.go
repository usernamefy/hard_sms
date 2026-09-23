package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"

	"sms/models"
	"sms/utils"

	"github.com/gin-gonic/gin"
)

type UserController struct{}

// UserCtl 用户控制器实例
var UserCtl = &UserController{}

type loginForm struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required"`
}

type userForm struct {
	Username    string `form:"username" binding:"required"`
	Password    string `form:"password"`
	RealName    string `form:"realName"`
	DepartmentID uint  `form:"departmentId"`
	Role        string `form:"role"`
	Status      int    `form:"status"`
}

// ==================== 登录相关 ====================

// Login 渲染登录页
func (c *UserController) Login(ctx *gin.Context) {
	session := sessions.Default(ctx)
	if uid, ok := session.Get("user_id").(uint); ok && uid > 0 {
		ctx.Redirect(http.StatusFound, "/")
		return
	}
	ctx.HTML(http.StatusOK, "login.html", gin.H{
		"title": "登录 - 库存管理系统",
		"error": ctx.Query("error"),
	})
}

// DoLogin 处理登录表单提交
func (c *UserController) DoLogin(ctx *gin.Context) {
	var form loginForm
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.Redirect(http.StatusFound, "/login?error=用户名和密码不能为空")
		return
	}

	user, err := models.GetUserByUsername(form.Username)
	if err != nil || user.Password != utils.Md5(form.Password) {
		ctx.Redirect(http.StatusFound, "/login?error=用户名或密码错误")
		return
	}
	if user.Status != 1 {
		ctx.Redirect(http.StatusFound, "/login?error=账号已被禁用，请联系管理员")
		return
	}

	session := sessions.Default(ctx)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	session.Set("realname", user.RealName)
	if err := session.Save(); err != nil {
		ctx.Redirect(http.StatusFound, "/login?error=会话创建失败，请重试")
		return
	}
	// 更新最后登录时间
	now := time.Now()
	models.DB.Model(user).Update("last_login", &now)

	ctx.Redirect(http.StatusFound, "/")
}

// Logout 退出登录，清空会话
func (c *UserController) Logout(ctx *gin.Context) {
	session := sessions.Default(ctx)
	session.Clear()
	_ = session.Save()
	ctx.Redirect(http.StatusFound, "/login")
}

// userPageData 为后台页面注入顶栏展示的当前登录人信息
func userPageData(ctx *gin.Context, data gin.H) gin.H {
	session := sessions.Default(ctx)
	data["realname"] = session.Get("realname")
	data["username"] = session.Get("username")
	return data
}

// ==================== 用户管理 ====================

// List 用户列表页
func (c *UserController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	users, total, err := models.ListUsers(models.UserQuery{
		Keyword:  ctx.Query("keyword"),
		Page:     page,
		PageSize: 10,
	})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询用户列表失败"})
		return
	}

	pageSize := 10
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	ctx.HTML(http.StatusOK, "user_list.html", userPageData(ctx, gin.H{
		"title":      "用户管理 - 库存管理系统",
		"users":      users,
		"keyword":    ctx.Query("keyword"),
		"page":       page,
		"total":      total,
		"totalPages": totalPages,
	}))
}

// renderUserForm 渲染用户新增/编辑表单（加载启用部门下拉）
func (c *UserController) renderUserForm(ctx *gin.Context, title, action string, user *models.User, isEdit bool, errMsg string) {
	departments, err := models.ListEnabledDepartments()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "加载部门列表失败"})
		return
	}
	ctx.HTML(http.StatusOK, "user_form.html", userPageData(ctx, gin.H{
		"title":       title,
		"action":      action,
		"user":        user,
		"isEdit":      isEdit,
		"departments": departments,
		"error":       errMsg,
	}))
}

// validateDepartmentID 部门下拉值校验：0 表示未设置，其余必须为启用中的部门
func validateDepartmentID(id uint) bool {
	if id == 0 {
		return true
	}
	department, err := models.GetDepartmentByID(id)
	return err == nil && department.Status == 1
}

// Add 渲染新增用户页
func (c *UserController) Add(ctx *gin.Context) {
	c.renderUserForm(ctx, "新增用户 - 库存管理系统", "/users/add",
		&models.User{Status: 1, Role: "admin"}, false, ctx.Query("error"))
}

// DoAdd 处理新增用户提交
func (c *UserController) DoAdd(ctx *gin.Context) {
	var form userForm
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.Redirect(http.StatusFound, "/users/add?error=用户名为必填项")
		return
	}
	if len(form.Password) < 6 {
		ctx.Redirect(http.StatusFound, "/users/add?error=密码长度至少 6 位")
		return
	}
	if _, err := models.GetUserByUsername(form.Username); err == nil {
		ctx.Redirect(http.StatusFound, "/users/add?error=用户名已存在")
		return
	}

	if form.Role == "" {
		form.Role = "admin"
	}
	if !validateDepartmentID(form.DepartmentID) {
		ctx.Redirect(http.StatusFound, "/users/add?error=所选部门不存在或已禁用")
		return
	}
	user := &models.User{
		Username:     form.Username,
		Password:     utils.Md5(form.Password),
		RealName:     form.RealName,
		DepartmentID: form.DepartmentID,
		Role:         form.Role,
		Status:       form.Status,
	}
	if err := models.CreateUser(user); err != nil {
		ctx.Redirect(http.StatusFound, "/users/add?error=创建失败："+err.Error())
		return
	}
	ctx.Redirect(http.StatusFound, "/users")
}

// Edit 渲染编辑用户页
func (c *UserController) Edit(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	user, err := models.GetUserByID(uint(id))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"msg": "用户不存在"})
		return
	}
	c.renderUserForm(ctx, "编辑用户 - 库存管理系统", "/users/edit/"+ctx.Param("id"), user, true, ctx.Query("error"))
}

// DoEdit 处理编辑用户提交
func (c *UserController) DoEdit(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	user, err := models.GetUserByID(uint(id))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"msg": "用户不存在"})
		return
	}

	var form userForm
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.Redirect(http.StatusFound, "/users/edit/"+ctx.Param("id")+"?error=用户名为必填项")
		return
	}

	// 用户名被修改时检查新用户名是否已被占用
	if form.Username != user.Username {
		if _, err := models.GetUserByUsername(form.Username); err == nil {
			ctx.Redirect(http.StatusFound, "/users/edit/"+ctx.Param("id")+"?error=用户名已存在")
			return
		}
	}

	fields := map[string]interface{}{
		"username":      form.Username,
		"real_name":     form.RealName,
		"department_id": form.DepartmentID,
		"role":          form.Role,
		"status":        form.Status,
	}
	if !validateDepartmentID(form.DepartmentID) {
		ctx.Redirect(http.StatusFound, "/users/edit/"+ctx.Param("id")+"?error=所选部门不存在或已禁用")
		return
	}
	// 编辑时密码留空表示不修改
	if form.Password != "" {
		if len(form.Password) < 6 {
			ctx.Redirect(http.StatusFound, "/users/edit/"+ctx.Param("id")+"?error=密码长度至少 6 位")
			return
		}
		fields["password"] = utils.Md5(form.Password)
	}
	if err := models.UpdateUser(user, fields); err != nil {
		ctx.Redirect(http.StatusFound, "/users/edit/"+ctx.Param("id")+"?error=保存失败："+err.Error())
		return
	}
	ctx.Redirect(http.StatusFound, "/users")
}
