package controllers

import (
	"net/http"
	"strconv"

	"sms/models"

	"github.com/gin-gonic/gin"
)

type WarehouseController struct{}

// WarehouseCtl 仓库控制器实例
var WarehouseCtl = &WarehouseController{}

type warehouseForm struct {
	Name      string `form:"name" binding:"required"`
	Code      string `form:"code"`
	Address   string `form:"address"`
	ManagerID uint   `form:"managerId"`
	Remark    string `form:"remark"`
	Status    int    `form:"status"`
}

type locationForm struct {
	Name   string `form:"name" binding:"required"`
	Code   string `form:"code"`
	Remark string `form:"remark"`
	Status int    `form:"status"`
}

// 路径统一走 detail 静态段，避免与 /warehouses/add、/warehouses/edit/:id 产生路由冲突
func warehouseDetailPath(id string) string { return "/warehouses/detail/" + id }
func locationAddPath(id string) string     { return "/warehouses/detail/" + id + "/locations/add" }
func locationEditPath(id, locID string) string {
	return "/warehouses/detail/" + id + "/locations/edit/" + locID
}

// List 仓库列表页（支持 ?page= 分页）
func (c *WarehouseController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 10
	warehouses, total, err := models.ListWarehouses(page, pageSize)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询仓库列表失败"})
		return
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	ctx.HTML(http.StatusOK, "warehouse_list.html", userPageData(ctx, gin.H{
		"title":      "仓库列表 - 库存管理系统",
		"warehouses": warehouses,
		"page":       page,
		"total":      total,
		"totalPages": totalPages,
	}))
}

// Add 渲染新增仓库页（新增仓库时不填写仓位，仓位在详情中单独维护）
func (c *WarehouseController) Add(ctx *gin.Context) {
	users, err := models.ListEnabledUsers()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询管理员列表失败"})
		return
	}
	ctx.HTML(http.StatusOK, "warehouse_form.html", userPageData(ctx, gin.H{
		"title":     "新增仓库 - 库存管理系统",
		"action":    "/warehouses/add",
		"warehouse": models.Warehouse{Status: 1},
		"users":     users,
		"isEdit":    false,
		"error":     ctx.Query("error"),
	}))
}

// DoAdd 处理新增仓库提交
func (c *WarehouseController) DoAdd(ctx *gin.Context) {
	var form warehouseForm
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.Redirect(http.StatusFound, "/warehouses/add?error=仓库名称为必填项")
		return
	}
	warehouse := &models.Warehouse{
		Name:      form.Name,
		Code:      form.Code,
		Address:   form.Address,
		ManagerID: form.ManagerID,
		Remark:    form.Remark,
		Status:    form.Status,
	}
	if err := models.CreateWarehouse(warehouse); err != nil {
		ctx.Redirect(http.StatusFound, "/warehouses/add?error=创建失败："+err.Error())
		return
	}
	ctx.Redirect(http.StatusFound, "/warehouses")
}

// Detail 仓库详情页：上半部分展示仓库基本信息，下半部分直接展示该仓库的仓位列表
func (c *WarehouseController) Detail(ctx *gin.Context) {
	warehouse, err := findWarehouse(ctx)
	if err != nil {
		return
	}
	locations, err := models.ListLocationsByWarehouse(warehouse.ID)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询仓位列表失败"})
		return
	}
	ctx.HTML(http.StatusOK, "warehouse_detail.html", userPageData(ctx, gin.H{
		"title":     "仓库详情 - 库存管理系统",
		"warehouse": warehouse,
		"locations": locations,
	}))
}

// Edit 渲染编辑仓库页
func (c *WarehouseController) Edit(ctx *gin.Context) {
	warehouse, err := findWarehouse(ctx)
	if err != nil {
		return
	}
	users, err := models.ListEnabledUsers()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询管理员列表失败"})
		return
	}
	ctx.HTML(http.StatusOK, "warehouse_form.html", userPageData(ctx, gin.H{
		"title":     "编辑仓库 - 库存管理系统",
		"action":    "/warehouses/edit/" + ctx.Param("id"),
		"warehouse": warehouse,
		"users":     users,
		"isEdit":    true,
		"error":     ctx.Query("error"),
	}))
}

// DoEdit 处理编辑仓库提交
func (c *WarehouseController) DoEdit(ctx *gin.Context) {
	warehouse, err := findWarehouse(ctx)
	if err != nil {
		return
	}
	var form warehouseForm
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.Redirect(http.StatusFound, "/warehouses/edit/"+ctx.Param("id")+"?error=仓库名称为必填项")
		return
	}
	fields := map[string]interface{}{
		"name":       form.Name,
		"code":       form.Code,
		"address":    form.Address,
		"manager_id": form.ManagerID,
		"remark":     form.Remark,
		"status":     form.Status,
	}
	if err := models.UpdateWarehouse(warehouse, fields); err != nil {
		ctx.Redirect(http.StatusFound, "/warehouses/edit/"+ctx.Param("id")+"?error=保存失败："+err.Error())
		return
	}
	ctx.Redirect(http.StatusFound, warehouseDetailPath(ctx.Param("id")))
}

// LocationAdd 渲染新增仓位页（跳转到独立页面）
func (c *WarehouseController) LocationAdd(ctx *gin.Context) {
	warehouse, err := findWarehouse(ctx)
	if err != nil {
		return
	}
	ctx.HTML(http.StatusOK, "location_form.html", userPageData(ctx, gin.H{
		"title":     "创建仓位 - 库存管理系统",
		"warehouse": warehouse,
		"location":  models.Location{Status: 1},
		"action":    locationAddPath(ctx.Param("id")),
		"isEdit":    false,
		"error":     ctx.Query("error"),
	}))
}

// LocationDoAdd 处理新增仓位提交
func (c *WarehouseController) LocationDoAdd(ctx *gin.Context) {
	warehouse, err := findWarehouse(ctx)
	if err != nil {
		return
	}
	var form locationForm
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.Redirect(http.StatusFound, locationAddPath(ctx.Param("id"))+"?error=仓位名称为必填项")
		return
	}
	location := &models.Location{
		WarehouseID: warehouse.ID,
		Name:        form.Name,
		Code:        form.Code,
		Remark:      form.Remark,
		Status:      form.Status,
	}
	if err := models.CreateLocation(location); err != nil {
		ctx.Redirect(http.StatusFound, locationAddPath(ctx.Param("id"))+"?error=创建失败："+err.Error())
		return
	}
	ctx.Redirect(http.StatusFound, warehouseDetailPath(ctx.Param("id")))
}

// LocationEdit 渲染编辑仓位页
func (c *WarehouseController) LocationEdit(ctx *gin.Context) {
	warehouse, err := findWarehouse(ctx)
	if err != nil {
		return
	}
	location, err := findLocation(ctx)
	if err != nil {
		return
	}
	ctx.HTML(http.StatusOK, "location_form.html", userPageData(ctx, gin.H{
		"title":     "编辑仓位 - 库存管理系统",
		"warehouse": warehouse,
		"location":  location,
		"action":    locationEditPath(ctx.Param("id"), ctx.Param("locId")),
		"isEdit":    true,
		"error":     ctx.Query("error"),
	}))
}

// LocationDoEdit 处理编辑仓位提交
func (c *WarehouseController) LocationDoEdit(ctx *gin.Context) {
	if _, err := findWarehouse(ctx); err != nil {
		return
	}
	location, err := findLocation(ctx)
	if err != nil {
		return
	}
	var form locationForm
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.Redirect(http.StatusFound, locationEditPath(ctx.Param("id"), ctx.Param("locId"))+"?error=仓位名称为必填项")
		return
	}
	fields := map[string]interface{}{
		"name":   form.Name,
		"code":   form.Code,
		"remark": form.Remark,
		"status": form.Status,
	}
	if err := models.UpdateLocation(location, fields); err != nil {
		ctx.Redirect(http.StatusFound, locationEditPath(ctx.Param("id"), ctx.Param("locId"))+"?error=保存失败："+err.Error())
		return
	}
	ctx.Redirect(http.StatusFound, warehouseDetailPath(ctx.Param("id")))
}

// LocationDelete 删除仓位（软删除）
func (c *WarehouseController) LocationDelete(ctx *gin.Context) {
	if _, err := findWarehouse(ctx); err != nil {
		return
	}
	location, err := findLocation(ctx)
	if err != nil {
		return
	}
	if err := models.DeleteLocation(location.ID); err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "删除仓位失败"})
		return
	}
	ctx.Redirect(http.StatusFound, warehouseDetailPath(ctx.Param("id")))
}

// findWarehouse 解析 URL 中的仓库 ID 并查询，不存在时直接渲染 404
func findWarehouse(ctx *gin.Context) (*models.Warehouse, error) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	warehouse, err := models.GetWarehouseByID(uint(id))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"msg": "仓库不存在"})
		return nil, err
	}
	return warehouse, nil
}

// findLocation 解析 URL 中的仓位 ID 并查询，不存在时直接渲染 404
func findLocation(ctx *gin.Context) (*models.Location, error) {
	id, _ := strconv.Atoi(ctx.Param("locId"))
	location, err := models.GetLocationByID(uint(id))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"msg": "仓位不存在"})
		return nil, err
	}
	return location, nil
}
