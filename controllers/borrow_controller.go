package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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

// Return 整单归还
func (c *BorrowController) Return(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	err := models.ReturnBorrowOrder(uint(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.HTML(http.StatusNotFound, "error.html", gin.H{"msg": "借用单不存在"})
		return
	}
	detailPath := "/borrows/" + ctx.Param("id")
	if err != nil {
		ctx.Redirect(http.StatusFound, detailPath+"?error="+err.Error())
		return
	}
	ctx.Redirect(http.StatusFound, detailPath)
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
