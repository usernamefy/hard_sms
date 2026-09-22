package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"sms/models"
)

type BorrowController struct{}

// BorrowCtl 借用控制器实例
var BorrowCtl = &BorrowController{}

// borrowForm 新建借用表单（商品清单为 productIds[] / quantities[] 平行数组）
type borrowForm struct {
	BorrowNo   string   `form:"borrowNo"` // 仅作预览展示，服务端提交时重新生成
	Department string   `form:"department"`
	Days       int      `form:"days" binding:"required"`
	Remark     string   `form:"remark"`
	ProductIds []string `form:"productIds"`
	Quantities []string `form:"quantities"`
}

// borrowItemDraft 校验失败回显表单时的单行草稿
type borrowItemDraft struct {
	ProductID uint
	Quantity  int
}

// borrowRowView 校验失败回显表单时，商品清单行的商品快照数据
type borrowRowView struct {
	ProductID uint
	Name      string
	SN        string
	Warehouse string
	Location  string
	Available int
	Quantity  int
}

// borrowStatusText 借用单状态文案
func borrowStatusText(status int) string {
	if status == models.BorrowOrderStatusActive {
		return "借用中"
	}
	if status == models.BorrowOrderStatusReturned {
		return "已归还"
	}
	return "已取消"
}

// List 借用列表页（page / keyword / status）
func (c *BorrowController) List(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	status, _ := strconv.Atoi(ctx.Query("status"))
	orders, total, err := models.ListBorrowOrders(models.BorrowQuery{
		Keyword:  ctx.Query("keyword"),
		Status:   status,
		Page:     page,
		PageSize: 10,
	})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "查询借用列表失败"})
		return
	}

	pageSize := 10
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	ctx.HTML(http.StatusOK, "borrow_list.html", userPageData(ctx, gin.H{
		"title":      "借用记录 - 库存管理系统",
		"orders":     orders,
		"keyword":    ctx.Query("keyword"),
		"status":     status,
		"page":       page,
		"total":      total,
		"totalPages": totalPages,
	}))
}

// Add 渲染新建借用页：预生成候选单号，加载在库商品（含可借数量）
func (c *BorrowController) Add(ctx *gin.Context) {
	c.renderForm(ctx, "/borrows/add", borrowForm{Days: 7}, nil, "")
}

// GenerateNo 生成候选借用单号（JSON）
func (c *BorrowController) GenerateNo(ctx *gin.Context) {
	no, err := models.GenerateBorrowNo()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "生成借用单号失败"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"no": no})
}

// SearchProducts 商品搜索接口（JSON）：按商品名称/SKU 模糊搜索在库商品，
// 供新建借用页搜索预览并添加到借用清单
func (c *BorrowController) SearchProducts(ctx *gin.Context) {
	products, err := models.SearchBorrowableProducts(ctx.Query("q"), 10)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "搜索商品失败"})
		return
	}
	type productHit struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		SN        string `json:"sn"`
		SKU       string `json:"sku"`
		Warehouse string `json:"warehouse"`
		Location  string `json:"location"`
		Available int    `json:"available"`
		Image     string `json:"image"`
	}
	hits := make([]productHit, 0, len(products))
	for i := range products {
		p := &products[i]
		image := ""
		if p.Image != "" {
			image = "/static/" + p.Image
		}
		hits = append(hits, productHit{
			ID:        p.ID,
			Name:      p.Name,
			SN:        p.SN,
			SKU:       p.SKU,
			Warehouse: p.WarehouseName,
			Location:  p.LocationName,
			Available: p.InStockQty,
			Image:     image,
		})
	}
	ctx.JSON(http.StatusOK, gin.H{"products": hits})
}

// DoAdd 处理新建借用提交
func (c *BorrowController) DoAdd(ctx *gin.Context) {
	var form borrowForm
	fail := func(msg string) {
		c.renderForm(ctx, "/borrows/add", form, parseBorrowDraft(&form), msg)
	}
	if err := ctx.ShouldBind(&form); err != nil {
		fail("请填写借用天数并至少选择一件商品")
		return
	}
	if form.Days < 1 || form.Days > 365 {
		fail("借用天数需在 1 ~ 365 之间")
		return
	}
	inputs, parseErr := parseBorrowInputs(&form)
	if parseErr != "" {
		fail(parseErr)
		return
	}
	if len(inputs) == 0 {
		fail("请至少选择一件要借用的商品")
		return
	}

	// 借用人固定取当前登录用户，不信任表单数据
	session := sessions.Default(ctx)
	borrowerID, _ := session.Get("user_id").(uint)
	if borrowerID == 0 {
		fail("登录状态已失效，请重新登录")
		return
	}
	borrower, err := models.GetUserByID(borrowerID)
	if err != nil {
		fail("登录用户不存在，请重新登录")
		return
	}

	now := time.Now()
	order := &models.BorrowOrder{
		BorrowerID:       borrower.ID,
		BorrowerName:     borrower.DisplayName(),
		Department:       strings.TrimSpace(form.Department),
		BorrowDate:       now,
		Days:             form.Days,
		ExpectReturnDate: now.AddDate(0, 0, form.Days),
		Remark:           strings.TrimSpace(form.Remark),
	}
	if err := models.CreateBorrowOrder(order, inputs); err != nil {
		fail("借用失败：" + err.Error())
		return
	}
	totalQty := 0
	for _, in := range inputs {
		totalQty += in.Quantity
	}
	recordOperation(ctx, "借用管理", "借用", "新建借用单「"+order.BorrowNo+"」，共 "+strconv.Itoa(len(inputs))+" 种商品 "+strconv.Itoa(totalQty)+" 件",
		productNamesSummary(models.BorrowProductNames(order.ID)))
	ctx.Redirect(http.StatusFound, "/borrows/"+strconv.FormatUint(uint64(order.ID), 10))
}

// Detail 借用详情页
func (c *BorrowController) Detail(ctx *gin.Context) {
	order, err := findBorrowOrder(ctx)
	if err != nil {
		return
	}
	ctx.HTML(http.StatusOK, "borrow_detail.html", userPageData(ctx, gin.H{
		"title": "借用详情 - 库存管理系统",
		"order": order,
		"error": ctx.Query("error"),
	}))
}

// returnForm 归还表单：搜索商品选中一条借用明细后登记归还
type returnForm struct {
	ItemID             string `form:"itemId"`             // 选中的借用明细 ID
	ReturnQuantity     string `form:"returnQuantity"`     // 归还数量
	IsLost             string `form:"isLost"`             // 商品丢失：勾选时为 "1"
	CompensationAmount string `form:"compensationAmount"` // 赔偿金额（丢失时必填）
	Remark             string `form:"remark"`
}

// Return 渲染归还页：预生成候选归还单号，搜索商品选中借用明细后提交
func (c *BorrowController) Return(ctx *gin.Context) {
	returnNo, err := models.GenerateReturnNo()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "生成归还单号失败"})
		return
	}
	doneQty, _ := strconv.Atoi(ctx.DefaultQuery("done", "0"))
	c.renderReturn(ctx, nil, returnForm{}, "", returnNo, doneQty, ctx.Query("no"))
}

// SearchReturnables 归还商品搜索接口（JSON）：按借用单号/借用人/商品名称/SN
// 模糊搜索借用中且有未归还数量的明细，带回借用信息与超期天数
func (c *BorrowController) SearchReturnables(ctx *gin.Context) {
	items, err := models.SearchReturnableItems(ctx.Query("q"), 10)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "搜索可归还商品失败"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items})
}

// DoReturn 处理归还提交：创建归还单，累加借用明细已归还数量，整单还清自动转为已归还
func (c *BorrowController) DoReturn(ctx *gin.Context) {
	var form returnForm
	fail := func(msg string) {
		var selected *models.BorrowReturnItem
		if id, err := strconv.ParseUint(strings.TrimSpace(form.ItemID), 10, 64); err == nil && id > 0 {
			selected, _ = models.GetReturnItemByID(uint(id))
		}
		returnNo, _ := models.GenerateReturnNo()
		c.renderReturn(ctx, selected, form, msg, returnNo, 0, "")
	}
	if err := ctx.ShouldBind(&form); err != nil {
		fail("归还失败：表单数据不完整，请重新提交")
		return
	}
	itemID, err := strconv.ParseUint(strings.TrimSpace(form.ItemID), 10, 64)
	if err != nil || itemID == 0 {
		fail("归还失败：请先搜索并选择要归还的商品")
		return
	}
	quantity, err := strconv.Atoi(strings.TrimSpace(form.ReturnQuantity))
	if err != nil || quantity < 1 {
		fail("归还失败：归还数量需为不小于 1 的整数")
		return
	}
	isLost := strings.TrimSpace(form.IsLost) == "1"
	var compensation *float64
	if isLost {
		raw := strings.TrimSpace(form.CompensationAmount)
		amount, perr := strconv.ParseFloat(raw, 64)
		if raw == "" || perr != nil || amount < 0 {
			fail("归还失败：商品丢失时需填写不小于 0 的赔偿金额")
			return
		}
		compensation = &amount
	}

	// 归还操作人固定取当前登录用户，不信任表单数据
	session := sessions.Default(ctx)
	operatorID, _ := session.Get("user_id").(uint)
	operatorName := ""
	if realname, ok := session.Get("realname").(string); ok && realname != "" {
		operatorName = realname
	} else if username, ok := session.Get("username").(string); ok {
		operatorName = username
	}
	if operatorID == 0 {
		fail("登录状态已失效，请重新登录")
		return
	}

	order := &models.ReturnOrder{
		BorrowItemID:       uint(itemID),
		Quantity:           quantity,
		IsLost:             lostInt(isLost),
		CompensationAmount: compensation,
		Remark:             strings.TrimSpace(form.Remark),
		ReturnedByID:       operatorID,
		ReturnedByName:     operatorName,
	}
	if err := models.CreateReturnOrder(order); err != nil {
		fail("归还失败：" + err.Error())
		return
	}

	detail := "新建归还单「" + order.ReturnNo + "」，商品「" + order.ProductName + "」归还 " + strconv.Itoa(order.Quantity) + " 件"
	if isLost {
		detail += "，商品丢失，赔偿金额 " + strconv.FormatFloat(*compensation, 'f', 2, 64) + " 元"
	}
	recordOperation(ctx, "借用管理", "归还", detail, order.ProductName)
	ctx.Redirect(http.StatusFound, "/borrows/return?done="+strconv.Itoa(order.Quantity)+"&no="+order.ReturnNo)
}

// lostInt 布尔转归还单丢失标记
func lostInt(isLost bool) int {
	if isLost {
		return 1
	}
	return 0
}

// renderReturn 渲染归还页；selected 为校验失败回显时已选中的借用明细，
// form 为回显的表单值；doneQty/doneNo 大于 0 时展示归还成功提示
func (c *BorrowController) renderReturn(ctx *gin.Context, selected *models.BorrowReturnItem, form returnForm, errMsg, returnNo string, doneQty int, doneNo string) {
	ctx.HTML(http.StatusOK, "borrow_return.html", userPageData(ctx, gin.H{
		"title":     "新增归还 - 库存管理系统",
		"returnNo":  returnNo,
		"today":     time.Now().Format("2006-01-02T15:04"),
		"selected":  selected,
		"form":      form,
		"error":     errMsg,
		"doneQty":   doneQty,
		"doneNo":    doneNo,
	}))
}

// parseBorrowInputs 解析并校验商品清单平行数组，返回数值化输入
func parseBorrowInputs(form *borrowForm) ([]models.BorrowItemInput, string) {
	if len(form.ProductIds) != len(form.Quantities) {
		return nil, "商品清单数据不完整，请重新编辑"
	}
	inputs := make([]models.BorrowItemInput, 0, len(form.ProductIds))
	for i := range form.ProductIds {
		productID, err := strconv.ParseUint(strings.TrimSpace(form.ProductIds[i]), 10, 64)
		if err != nil {
			continue // 未选择商品的空行跳过
		}
		quantity, err := strconv.Atoi(strings.TrimSpace(form.Quantities[i]))
		if err != nil || quantity < 1 {
			return nil, "借用数量至少为 1"
		}
		inputs = append(inputs, models.BorrowItemInput{ProductID: uint(productID), Quantity: quantity})
	}
	return inputs, ""
}

// parseBorrowDraft 宽松解析表单数组为回显草稿（仅用于渲染，不做校验）
func parseBorrowDraft(form *borrowForm) []borrowItemDraft {
	if len(form.ProductIds) != len(form.Quantities) {
		return nil
	}
	draft := make([]borrowItemDraft, 0, len(form.ProductIds))
	for i := range form.ProductIds {
		productID, err := strconv.ParseUint(strings.TrimSpace(form.ProductIds[i]), 10, 64)
		if err != nil {
			continue
		}
		quantity, err := strconv.Atoi(strings.TrimSpace(form.Quantities[i]))
		if err != nil || quantity < 1 {
			quantity = 1
		}
		draft = append(draft, borrowItemDraft{ProductID: uint(productID), Quantity: quantity})
	}
	return draft
}

// renderForm 渲染新建借用表单；draft 为空时清单无行（通过搜索添加商品），
// 校验失败回显时按草稿行带出商品快照信息
func (c *BorrowController) renderForm(ctx *gin.Context, action string, form borrowForm, draft []borrowItemDraft, errMsg string) {
	rows := make([]borrowRowView, 0, len(draft))
	for _, d := range draft {
		if d.ProductID == 0 {
			continue
		}
		p, err := models.GetProductByID(d.ProductID)
		if err != nil {
			continue
		}
		rows = append(rows, borrowRowView{
			ProductID: p.ID,
			Name:      p.Name,
			SN:        p.SN,
			Warehouse: p.WarehouseName,
			Location:  p.LocationName,
			Available: p.InStockQty,
			Quantity:  d.Quantity,
		})
	}

	days := form.Days
	if days < 1 {
		days = 7
	}
	borrowNo := form.BorrowNo
	if borrowNo == "" {
		no, err := models.GenerateBorrowNo()
		if err != nil {
			ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "生成借用单号失败"})
			return
		}
		borrowNo = no
	}

	ctx.HTML(http.StatusOK, "borrow_form.html", userPageData(ctx, gin.H{
		"title":    "新建借用 - 库存管理系统",
		"action":   action,
		"form":     form,
		"days":     days,
		"borrowNo": borrowNo,
		"rows":     rows,
		"error":    errMsg,
		"today":    time.Now().Format("2006-01-02 15:04"),
	}))
}

// findBorrowOrder 解析 URL 中的借用单 ID 并查询，不存在时直接渲染 404
func findBorrowOrder(ctx *gin.Context) (*models.BorrowOrder, error) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	order, err := models.GetBorrowByID(uint(id))
	if err != nil {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"msg": "借用单不存在"})
		return nil, err
	}
	return order, nil
}
