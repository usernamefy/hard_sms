package controllers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"sms/models"
)

type ProductController struct{}

// ProductCtl 商品控制器实例
var ProductCtl = &ProductController{}

// snPattern SN 码格式：SN + yyyyMMdd + 3 位序号
var snPattern = regexp.MustCompile(`^SN\d{11}$`)

// 商品图片限制：jpg/jpeg/png，≤ 5MB
const productImageMaxSize = 5 << 20

var productImageExts = map[string]bool{"jpg": true, "jpeg": true, "png": true}

type productForm struct {
	Name        string `form:"name" binding:"required"`
	SN          string `form:"sn" binding:"required"`
	SKU         string `form:"sku"`
	SPU         string `form:"spu"`
	Price       string `form:"price"`
	Category    string `form:"category" binding:"required"`
	SubCategory string `form:"subCategory"`
	OwnerID     uint   `form:"ownerId"`
	WarehouseID uint   `form:"warehouseId" binding:"required"`
	LocationID  uint   `form:"locationId"`
	Quantity    int    `form:"quantity" binding:"required"`
	Remark      string `form:"remark"`
}

// locationOption 仓库仓位二级联动的下拉数据
type locationOption struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func productDetailPath(id string) string { return "/products/" + id }

// List 商品列表页（page / keyword / category）
func (c *ProductController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	products, total, err := models.ListProducts(models.ProductQuery{
		Keyword:  ctx.Query("keyword"),
		Category: ctx.Query("category"),
		Page:     page,
		PageSize: 10,
	})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询商品列表失败"})
		return
	}

	pageSize := 10
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	ctx.HTML(http.StatusOK, "product_list.html", userPageData(ctx, gin.H{
		"title":      "商品列表 - 库存管理系统",
		"products":   products,
		"keyword":    ctx.Query("keyword"),
		"category":   ctx.Query("category"),
		"categories": models.ProductCategories,
		"page":       page,
		"total":      total,
		"totalPages": totalPages,
	}))
}

// inventoryFilter 库存查询筛选条件（用于回显表单选中态）
type inventoryFilter struct {
	Name            string
	SKU             string
	SN              string
	Category        string
	OwnerID         uint
	OwnerDepartment string
	WarehouseID     uint
	LocationID      uint
}

// Inventory 库存查询页（name/sku/sn/category/owner/department/warehouse/location），结果不分页
func (c *ProductController) Inventory(ctx *gin.Context) {
	warehouseID, _ := strconv.ParseUint(ctx.Query("warehouse"), 10, 64)
	locationID, _ := strconv.ParseUint(ctx.Query("location"), 10, 64)
	ownerID, _ := strconv.ParseUint(ctx.Query("owner"), 10, 64)
	filter := inventoryFilter{
		Name:            ctx.Query("name"),
		SKU:             ctx.Query("sku"),
		SN:              ctx.Query("sn"),
		Category:        ctx.Query("category"),
		OwnerID:         uint(ownerID),
		OwnerDepartment: ctx.Query("department"),
		WarehouseID:     uint(warehouseID),
		LocationID:      uint(locationID),
	}
	products, err := models.ListInventory(models.InventoryQuery{
		Name:            filter.Name,
		SKU:             filter.SKU,
		SN:              filter.SN,
		Category:        filter.Category,
		OwnerID:         filter.OwnerID,
		OwnerDepartment: filter.OwnerDepartment,
		WarehouseID:     filter.WarehouseID,
		LocationID:      filter.LocationID,
	})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "库存查询失败"})
		return
	}

	warehouses, err := models.ListEnabledWarehouses()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "加载仓库列表失败"})
		return
	}
	locations, err := models.ListEnabledLocations()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "加载仓位列表失败"})
		return
	}
	users, err := models.ListEnabledUsers()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "加载归属人列表失败"})
		return
	}
	departments, err := models.ListEnabledDepartments()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "加载部门列表失败"})
		return
	}
	type locationJSON struct {
		ID          uint   `json:"id"`
		WarehouseID uint   `json:"wh"`
		Name        string `json:"name"`
	}
	locationData := make([]locationJSON, 0, len(locations))
	for _, l := range locations {
		locationData = append(locationData, locationJSON{ID: l.ID, WarehouseID: l.WarehouseID, Name: l.Name})
	}
	locationBytes, _ := json.Marshal(locationData)

	ctx.HTML(http.StatusOK, "inventory.html", userPageData(ctx, gin.H{
		"title":        "库存查询 - 库存管理系统",
		"products":     products,
		"total":        len(products),
		"filter":       filter,
		"categories":   models.ProductCategories,
		"owners":       users,
		"departments":  departments,
		"warehouses":   warehouses,
		"locationJSON": string(locationBytes),
	}))
}

// Add 渲染创建商品（入库登记）页，服务端预生成一个候选 SN
func (c *ProductController) Add(ctx *gin.Context) {
	sn, err := models.GenerateProductSN()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "生成 SN 码失败"})
		return
	}
	product := &models.Product{SN: sn, Status: models.ProductStatusInStock, Quantity: 1}
	c.renderForm(ctx, "/products/add", product, false, "")
}

// DoAdd 处理入库登记提交（multipart/form-data 含图片）
func (c *ProductController) DoAdd(ctx *gin.Context) {
	var form productForm
	// 校验失败时带着已填内容重渲染表单，避免用户重新录入
	fail := func(msg string) {
		c.renderForm(ctx, "/products/add", productFromForm(&form), false, msg)
	}
	if err := ctx.ShouldBind(&form); err != nil {
		fail("请填写商品名称、SN 码、一级分类、仓库和入库数量等必填项")
		return
	}
	form.SN = strings.TrimSpace(form.SN)
	if !snPattern.MatchString(form.SN) {
		fail("SN 码格式不正确，请点击「生成 SN 码」重新生成")
		return
	}
	if form.Quantity < 1 {
		fail("入库数量至少为 1")
		return
	}
	if !models.IsValidProductCategory(form.Category) {
		fail("请选择有效的一级分类")
		return
	}
	if form.OwnerID == 0 {
		fail("请选择样品归属人")
		return
	}
	price, priceErr := parsePriceInput(form.Price)
	if priceErr != "" {
		fail(priceErr)
		return
	}
	warehouse, err := models.GetWarehouseByID(form.WarehouseID)
	if err != nil || warehouse.Status != 1 {
		fail("所选仓库不存在或已禁用")
		return
	}
	if msg := validateLocation(warehouse, form.LocationID); msg != "" {
		fail(msg)
		return
	}
	imgData, imgExt, imgErr := readProductImage(ctx)
	if imgErr != "" {
		fail(imgErr)
		return
	}

	product := productFromForm(&form)
	product.Price = price
	product.InboundDate = time.Now()
	product.Status = models.ProductStatusInStock
	if uid, ok := sessions.Default(ctx).Get("user_id").(uint); ok && uid > 0 {
		product.CreatedBy = &uid
	}
	if err := models.CreateProduct(product); err != nil {
		fail("入库失败：" + err.Error())
		return
	}
	recordOperation(ctx, "商品入库", "商品入库", "商品「"+product.Name+"」入库（SN："+product.SN+"）", product.Name)
	saveProductImageFile(product, imgData, imgExt, "")
	ctx.Redirect(http.StatusFound, productDetailPath(strconv.FormatUint(uint64(product.ID), 10)))
}

// GenerateSN 生成候选 SN 码（JSON）
func (c *ProductController) GenerateSN(ctx *gin.Context) {
	sn, err := models.GenerateProductSN()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "生成 SN 码失败"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"sn": sn})
}

// Detail 商品详情页
func (c *ProductController) Detail(ctx *gin.Context) {
	product, err := findProduct(ctx)
	if err != nil {
		return
	}
	ctx.HTML(http.StatusOK, "product_detail.html", userPageData(ctx, gin.H{
		"title":   "商品详情 - 库存管理系统",
		"product": product,
	}))
}

// Edit 渲染编辑商品页（SN 码、入库日期、库龄只读）
func (c *ProductController) Edit(ctx *gin.Context) {
	product, err := findProduct(ctx)
	if err != nil {
		return
	}
	c.renderForm(ctx, "/products/edit/"+ctx.Param("id"), product, true, "")
}

// DoEdit 处理编辑商品提交（修改仓库/仓位即视为移库，不动入库日期）
func (c *ProductController) DoEdit(ctx *gin.Context) {
	product, err := findProduct(ctx)
	if err != nil {
		return
	}
	var form productForm
	fail := func(msg string) {
		draft := productFromForm(&form)
		draft.SN = product.SN
		draft.InboundDate = product.InboundDate
		draft.AgeDays = product.AgeDays
		c.renderForm(ctx, "/products/edit/"+ctx.Param("id"), draft, true, msg)
	}
	if err := ctx.ShouldBind(&form); err != nil {
		fail("请填写商品名称、一级分类、仓库和入库数量等必填项")
		return
	}
	if form.Quantity < 1 {
		fail("入库数量至少为 1")
		return
	}
	if !models.IsValidProductCategory(form.Category) {
		fail("请选择有效的一级分类")
		return
	}
	if form.OwnerID == 0 {
		fail("请选择样品归属人")
		return
	}
	price, priceErr := parsePriceInput(form.Price)
	if priceErr != "" {
		fail(priceErr)
		return
	}
	warehouse, err := models.GetWarehouseByID(form.WarehouseID)
	if err != nil || warehouse.Status != 1 {
		fail("所选仓库不存在或已禁用")
		return
	}
	if msg := validateLocation(warehouse, form.LocationID); msg != "" {
		fail(msg)
		return
	}
	imgData, imgExt, imgErr := readProductImage(ctx)
	if imgErr != "" {
		fail(imgErr)
		return
	}

	var locationID *uint
	if form.LocationID > 0 {
		locationID = &form.LocationID
	}
	//使用map，不会过滤0值
	fields := map[string]interface{}{
		"name":         form.Name,
		"sku":          form.SKU,
		"spu":          form.SPU,
		"category":     form.Category,
		"sub_category": form.SubCategory,
		"price":        price,
		"owner_id":     form.OwnerID,
		"warehouse_id": form.WarehouseID,
		"location_id":  locationID,
		"quantity":     form.Quantity,
		"remark":       form.Remark,
	}
	if err := models.UpdateProduct(product, fields); err != nil {
		fail("保存失败：" + err.Error())
		return
	}
	saveProductImageFile(product, imgData, imgExt, product.Image)
	ctx.Redirect(http.StatusFound, productDetailPath(ctx.Param("id")))
}

// renderForm 渲染商品新增/编辑表单（含仓库仓位二级联动数据）
func (c *ProductController) renderForm(ctx *gin.Context, action string, product *models.Product, isEdit bool, errMsg string) {
	warehouses, err := models.ListEnabledWarehouses()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询仓库列表失败"})
		return
	}
	locations, err := models.ListEnabledLocations()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询仓位列表失败"})
		return
	}
	users, err := models.ListEnabledUsers()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询用户列表失败"})
		return
	}

	// 样品归属人下拉：启用用户（带所属部门供表单自动带出）
	type ownerOption struct {
		ID   uint
		Name string
		Dept string
	}
	ownerOptions := make([]ownerOption, 0, len(users))
	for i := range users {
		ownerOptions = append(ownerOptions, ownerOption{ID: users[i].ID, Name: users[i].DisplayName(), Dept: users[i].DepartmentName})
	}
	selectedOwnerID := uint(0)
	if product.OwnerID != nil {
		selectedOwnerID = *product.OwnerID
	}

	// 编辑时当前仓库/仓位可能已被停用，补充进下拉数据，保证回显不丢失
	warehouseIDs := make(map[uint]bool, len(warehouses))
	for _, w := range warehouses {
		warehouseIDs[w.ID] = true
	}
	if isEdit && product.WarehouseID > 0 && !warehouseIDs[product.WarehouseID] {
		if w, err := models.GetWarehouseByID(product.WarehouseID); err == nil {
			warehouses = append(warehouses, *w)
		}
	}
	locationsByWarehouse := make(map[string][]locationOption)
	locationIDs := make(map[uint]bool)
	for _, l := range locations {
		key := strconv.FormatUint(uint64(l.WarehouseID), 10)
		locationsByWarehouse[key] = append(locationsByWarehouse[key], locationOption{ID: l.ID, Name: l.Name})
		locationIDs[l.ID] = true
	}
	if isEdit && product.LocationID != nil && !locationIDs[*product.LocationID] {
		if l, err := models.GetLocationByID(*product.LocationID); err == nil {
			key := strconv.FormatUint(uint64(l.WarehouseID), 10)
			locationsByWarehouse[key] = append(locationsByWarehouse[key], locationOption{ID: l.ID, Name: l.Name})
		}
	}

	selectedLocationID := ""
	if product.LocationID != nil {
		selectedLocationID = strconv.FormatUint(uint64(*product.LocationID), 10)
	}
	inboundDate := time.Now().Format("2006-01-02")
	if isEdit && !product.InboundDate.IsZero() {
		inboundDate = product.InboundDate.Format("2006-01-02")
	}

	title := "创建商品（入库登记） - 库存管理系统"
	if isEdit {
		title = "编辑商品 - 库存管理系统"
	}
	ctx.HTML(http.StatusOK, "product_form.html", userPageData(ctx, gin.H{
		"title":                title,
		"action":               action,
		"product":              product,
		"isEdit":               isEdit,
		"error":                errMsg,
		"categories":           models.ProductCategories,
		"warehouses":           warehouses,
		"locationsByWarehouse": locationsByWarehouse,
		"selectedLocationID":   selectedLocationID,
		"ownerOptions":         ownerOptions,
		"selectedOwnerID":      selectedOwnerID,
		"inboundDate":          inboundDate,
		"ageDays":              product.AgeDays,
	}))
}

// productFromForm 用表单已填内容构造商品对象（校验失败回显用，SN/入库日期等由调用方补齐）
func productFromForm(form *productForm) *models.Product {
	product := &models.Product{
		Name:        form.Name,
		SN:          form.SN,
		SKU:         form.SKU,
		SPU:         form.SPU,
		Category:    form.Category,
		SubCategory: form.SubCategory,
		WarehouseID: form.WarehouseID,
		Quantity:    form.Quantity,
		Remark:      form.Remark,
	}
	if form.OwnerID > 0 {
		ownerID := form.OwnerID
		product.OwnerID = &ownerID
	}
	if form.LocationID > 0 {
		product.LocationID = &form.LocationID
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(form.Price), 64); err == nil {
		product.Price = &v
	}
	return product
}

// parsePriceInput 校验并解析价格（元），空串视为未填写；返回错误信息时价格为 nil
func parsePriceInput(s string) (*float64, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || v > 99999999.99 {
		return nil, "价格格式不正确（应为 0 ~ 99999999.99 的数字）"
	}
	return &v, ""
}

// validateLocation 校验仓位：必须属于所选仓库且为启用状态；未选择仓位时通过
func validateLocation(warehouse *models.Warehouse, locationID uint) string {
	if locationID == 0 {
		return "请选择仓位"
	}
	location, err := models.GetLocationByID(locationID)
	if err != nil || location.WarehouseID != warehouse.ID || location.Status != 1 {
		return "所选仓位不存在、已禁用或不属于所选仓库"
	}
	return ""
}

// findProduct 解析 URL 中的商品 ID 并查询，不存在时直接渲染 404
func findProduct(ctx *gin.Context) (*models.Product, error) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	product, err := models.GetProductByID(uint(id))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"msg": "商品不存在"})
		return nil, err
	}
	return product, nil
}

// readProductImage 读取上传的商品图片；未选择文件返回空数据；后缀/大小不合法时返回错误信息
func readProductImage(ctx *gin.Context) (data []byte, ext string, errMsg string) {
	fh, err := ctx.FormFile("image")
	if err != nil {
		return nil, "", ""
	}
	if fh.Size > productImageMaxSize {
		return nil, "", "商品图片大小不能超过 5MB"
	}
	ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(fh.Filename)), ".")
	if !productImageExts[ext] {
		return nil, "", "商品图片仅支持 jpg / png 格式"
	}
	file, err := fh.Open()
	if err != nil {
		return nil, "", "读取商品图片失败"
	}
	defer file.Close()
	data, err = io.ReadAll(file)
	if err != nil || len(data) > productImageMaxSize {
		return nil, "", "读取商品图片失败或图片大小超过 5MB"
	}
	return data, ext, ""
}

// saveProductImageFile 保存上传图片（以 SN 重命名）并更新 image 字段；oldPath 非空时清理被替换的旧文件
func saveProductImageFile(product *models.Product, data []byte, ext, oldPath string) {
	if len(data) == 0 {
		return
	}
	relPath, err := saveProductImage(product.SN, ext, data)
	if err != nil {
		log.Printf("保存商品图片失败 sn=%s: %v", product.SN, err)
		return
	}
	if err := models.UpdateProduct(product, map[string]interface{}{"image": relPath}); err != nil {
		log.Printf("更新商品图片路径失败 sn=%s: %v", product.SN, err)
		return
	}
	if oldPath != "" && oldPath != relPath {
		removeProductImage(oldPath)
	}
}

// saveProductImage 图片写入 static/uploads/products/yyyyMM/ 并以 SN 重命名
// （SN 唯一，天然防冲突、防原始文件名路径穿越），返回相对路径
func saveProductImage(sn, ext string, data []byte) (string, error) {
	month := time.Now().Format("200601")
	dir := filepath.Join("static", "uploads", "products", month)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	filename := sn + "." + ext
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0o644); err != nil {
		return "", err
	}
	return "uploads/products/" + month + "/" + filename, nil
}

// removeProductImage 清理旧图片文件（仅允许 uploads/products 目录内的路径）
func removeProductImage(relPath string) {
	if relPath == "" {
		return
	}
	full := filepath.ToSlash(filepath.Join("static", filepath.FromSlash(relPath)))
	if strings.HasPrefix(full, "static/uploads/products/") {
		_ = os.Remove(filepath.FromSlash(full))
	}
}
