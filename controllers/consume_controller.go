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

type ConsumeController struct{}

// ConsumeCtl 消耗控制器实例
var ConsumeCtl = &ConsumeController{}

// consumeForm 消耗表单
type consumeForm struct {
	ProductID uint   `form:"productId" binding:"required"`
	Quantity  int    `form:"quantity" binding:"required"`
	Reason    string `form:"reason"`
	Remark    string `form:"remark"`
}

// Add 渲染消耗页：预生成候选消耗单号、预填当前登录消耗人与部门
func (c *ConsumeController) Add(ctx *gin.Context) {
	consumeNo, err := models.GenerateConsumeNo()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "生成消耗单号失败"})
		return
	}
	doneQty, _ := strconv.Atoi(ctx.DefaultQuery("done", "0"))
	c.renderConsume(ctx, nil, consumeForm{}, consumeNo, ctx.Query("error"), doneQty, ctx.Query("no"))
}

// SearchProducts 可消耗商品搜索接口（JSON）：按 SN 码模糊搜索在库且有库存的商品
func (c *ConsumeController) SearchProducts(ctx *gin.Context) {
	products, err := models.SearchConsumableProducts(ctx.Query("q"), 10)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "搜索可消耗商品失败"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"products": products})
}

// DoAdd 处理消耗提交：创建消耗记录并同步扣减库存（不可撤销）
func (c *ConsumeController) DoAdd(ctx *gin.Context) {
	var form consumeForm
	if err := ctx.ShouldBind(&form); err != nil {
		c.renderFail(ctx, consumeForm{}, "请选择商品并填写消耗数量")
		return
	}
	if form.Quantity < 1 {
		c.renderFail(ctx, form, "消耗数量至少为 1")
		return
	}

	session := sessions.Default(ctx)
	consumerID, _ := session.Get("user_id").(uint)
	if consumerID == 0 {
		c.renderFail(ctx, form, "登录状态已失效，请重新登录")
		return
	}
	consumer, err := models.GetUserByID(consumerID)
	if err != nil {
		c.renderFail(ctx, form, "登录用户不存在，请重新登录")
		return
	}

	record := &models.ConsumeRecord{
		ProductID:          form.ProductID,
		Quantity:           form.Quantity,
		Reason:             strings.TrimSpace(form.Reason),
		Remark:             strings.TrimSpace(form.Remark),
		ConsumerID:         consumer.ID,
		ConsumerName:       consumer.DisplayName(),
		ConsumerDepartment: consumer.DepartmentName,
	}
	if err := models.CreateConsumeRecord(record); err != nil {
		c.renderFail(ctx, form, "消耗失败："+err.Error())
		return
	}

	detail := "新建消耗单「" + record.ConsumeNo + "」，商品「" + record.ProductName + "」消耗 " + strconv.Itoa(record.Quantity) + " 件"
	if record.Reason != "" {
		detail += "，原因：" + record.Reason
	}
	recordOperation(ctx, "库存管理", "消耗", detail, record.ProductName)
	ctx.Redirect(http.StatusFound, "/consumes/add?done="+strconv.Itoa(form.Quantity)+"&no="+record.ConsumeNo)
}

// renderFail 消耗提交校验失败：带已选商品与表单内容回显
func (c *ConsumeController) renderFail(ctx *gin.Context, form consumeForm, msg string) {
	var selected *models.ConsumableProduct
	if form.ProductID > 0 {
		selected, _ = models.GetConsumableProduct(form.ProductID)
	}
	consumeNo, err := models.GenerateConsumeNo()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "生成消耗单号失败"})
		return
	}
	c.renderConsume(ctx, selected, form, consumeNo, msg, 0, "")
}

// renderConsume 渲染消耗页；selected 为校验失败回显时已选中的商品，
// doneQty/doneNo 大于 0 时展示消耗成功提示
func (c *ConsumeController) renderConsume(ctx *gin.Context, selected *models.ConsumableProduct, form consumeForm, consumeNo, errMsg string, doneQty int, doneNo string) {
	session := sessions.Default(ctx)
	consumerName, _ := session.Get("realname").(string)
	if consumerName == "" {
		consumerName, _ = session.Get("username").(string)
	}
	department := ""
	if uid, ok := session.Get("user_id").(uint); ok && uid > 0 {
		if user, uerr := models.GetUserByID(uid); uerr == nil {
			department = user.DepartmentName
		}
	}

	ctx.HTML(http.StatusOK, "consume_form.html", userPageData(ctx, gin.H{
		"title":        "新增消耗 - 库存管理系统",
		"consumeNo":    consumeNo,
		"today":        time.Now().Format("2006-01-02T15:04"),
		"consumerName": consumerName,
		"department":   department,
		"reasons":      models.ConsumeReasons,
		"selected":     selected,
		"form":         form,
		"error":        errMsg,
		"doneQty":      doneQty,
		"doneNo":       doneNo,
	}))
}

