package controllers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"sms/models"
)

type TransferController struct{}

// TransferCtl 样品异动控制器实例
var TransferCtl = &TransferController{}

// ownerTransferForm 人员转移表单
type ownerTransferForm struct {
	OldOwnerID uint     `form:"oldOwnerId" binding:"required"`
	NewOwnerID uint     `form:"newOwnerId" binding:"required"`
	ProductIds []string `form:"productIds"`
	Reason     string   `form:"reason"`
	Remark     string   `form:"remark"`
}

// warehouseTransferForm 移仓表单
type warehouseTransferForm struct {
	SourceWarehouseID uint     `form:"sourceWarehouseId" binding:"required"`
	SourceLocationID  uint     `form:"sourceLocationId"`
	TargetWarehouseID uint     `form:"targetWarehouseId" binding:"required"`
	TargetLocationID  uint     `form:"targetLocationId"`
	ProductIds        []string `form:"productIds"`
	Reason            string   `form:"reason"`
	Remark            string   `form:"remark"`
}

// Page 样品异动页（人员转移 / 移仓 双 Tab）
func (c *TransferController) Page(ctx *gin.Context) {
	c.renderPage(ctx, ctx.Query("error"), parseDone(ctx))
}

// Samples 异动候选样品接口（JSON）：按归属人或仓库（可叠加仓位）过滤
func (c *TransferController) Samples(ctx *gin.Context) {
	ownerID, _ := strconv.ParseUint(ctx.Query("ownerId"), 10, 64)
	warehouseID, _ := strconv.ParseUint(ctx.Query("warehouseId"), 10, 64)
	locationID, _ := strconv.ParseUint(ctx.Query("locationId"), 10, 64)
	if ownerID == 0 && warehouseID == 0 {
		ctx.JSON(http.StatusOK, gin.H{"samples": []models.TransferSample{}})
		return
	}
	samples, err := models.ListTransferSamples(uint(ownerID), uint(warehouseID), uint(locationID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "查询样品列表失败"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"samples": samples})
}

// DoOwnerTransfer 人员转移提交：批量变更所选样品的归属人
func (c *TransferController) DoOwnerTransfer(ctx *gin.Context) {
	var form ownerTransferForm
	if err := ctx.ShouldBind(&form); err != nil {
		c.renderPage(ctx, "请选择原归属人、新归属人并勾选要转移的样品", [2]string{})
		return
	}
	fail := func(msg string) {
		c.renderPage(ctx, msg, [2]string{})
	}
	if form.OldOwnerID == form.NewOwnerID {
		fail("新归属人不能与原归属人相同")
		return
	}
	oldOwner, err := models.GetUserByID(form.OldOwnerID)
	if err != nil || oldOwner.Status != 1 {
		fail("原归属人不存在或已禁用")
		return
	}
	newOwner, err := models.GetUserByID(form.NewOwnerID)
	if err != nil || newOwner.Status != 1 {
		fail("新归属人不存在或已禁用")
		return
	}
	productIDs, perr := parseTransferProductIDs(form.ProductIds)
	if perr != "" {
		fail(perr)
		return
	}
	// 校验所选样品确实归属原归属人（防止勾选过期列表）
	samples, err := models.ListTransferSamples(form.OldOwnerID, 0, 0)
	if err != nil {
		fail("查询样品列表失败，请重试")
		return
	}
	inList := make(map[uint]bool, len(samples))
	for _, s := range samples {
		inList[s.ID] = true
	}
	for _, pid := range productIDs {
		if !inList[pid] {
			fail("所选样品不归属于「" + oldOwner.DisplayName() + "」，请刷新后重新选择")
			return
		}
	}

	records := make([]models.TransferRecord, 0, len(productIDs))
	for _, pid := range productIDs {
		records = append(records, models.TransferRecord{
			Type:           models.TransferTypeOwner,
			ProductID:      pid,
			FromOwnerID:    oldOwner.ID,
			FromOwnerName:  oldOwner.DisplayName(),
			FromDepartment: oldOwner.DepartmentName,
			ToOwnerID:      newOwner.ID,
			ToOwnerName:    newOwner.DisplayName(),
			ToDepartment:   newOwner.DepartmentName,
			Reason:         strings.TrimSpace(form.Reason),
			Remark:         strings.TrimSpace(form.Remark),
		})
	}
	if err := models.CreateTransferRecords(records, nil); err != nil {
		fail("转移失败：" + err.Error())
		return
	}

	detail := "人员转移：" + strconv.Itoa(len(productIDs)) + " 个样品归属人由「" +
		oldOwner.DisplayName() + "（" + oldOwner.DepartmentName + "）」变更为「" +
		newOwner.DisplayName() + "（" + newOwner.DepartmentName + "）」"
	if form.Reason != "" {
		detail += "，原因：" + form.Reason
	}
	recordOperation(ctx, "样品异动", "样品异动", detail, productNamesSummary(transferSampleNames(samples, productIDs)))
	ctx.Redirect(http.StatusFound, "/transfers?done="+strconv.Itoa(len(productIDs))+
		"&done_msg="+url.QueryEscape("已将 "+strconv.Itoa(len(productIDs))+" 个样品的归属人变更为 "+newOwner.DisplayName()))
}

// DoWarehouseTransfer 移仓提交：批量变更所选样品的目标仓库/仓位
func (c *TransferController) DoWarehouseTransfer(ctx *gin.Context) {
	var form warehouseTransferForm
	if err := ctx.ShouldBind(&form); err != nil {
		c.renderPage(ctx, "请选择源仓库、目标仓库并勾选要移仓的样品", [2]string{})
		return
	}
	fail := func(msg string) {
		c.renderPage(ctx, msg, [2]string{})
	}
	if form.SourceWarehouseID == form.TargetWarehouseID {
		fail("目标仓库不能与源仓库相同")
		return
	}
	sourceWarehouse, err := models.GetWarehouseByID(form.SourceWarehouseID)
	if err != nil || sourceWarehouse.Status != 1 {
		fail("源仓库不存在或已禁用")
		return
	}
	targetWarehouse, err := models.GetWarehouseByID(form.TargetWarehouseID)
	if err != nil || targetWarehouse.Status != 1 {
		fail("目标仓库不存在或已禁用")
		return
	}
	targetLocationName := ""
	if form.TargetLocationID > 0 {
		location, lerr := models.GetLocationByID(form.TargetLocationID)
		if lerr != nil || location.WarehouseID != targetWarehouse.ID || location.Status != 1 {
			fail("目标仓位不存在、已禁用或不属于目标仓库")
			return
		}
		targetLocationName = location.Name
	}
	productIDs, perr := parseTransferProductIDs(form.ProductIds)
	if perr != "" {
		fail(perr)
		return
	}
	samples, err := models.ListTransferSamples(0, form.SourceWarehouseID, form.SourceLocationID)
	if err != nil {
		fail("查询样品列表失败，请重试")
		return
	}
	inList := make(map[uint]bool, len(samples))
	for _, s := range samples {
		inList[s.ID] = true
	}
	for _, pid := range productIDs {
		if !inList[pid] {
			fail("所选样品已不在源仓库（仓位）中，请刷新后重新选择")
			return
		}
	}

	// 源位置名称（记录用）
	sourceLocationName := ""
	if form.SourceLocationID > 0 {
		if location, lerr := models.GetLocationByID(form.SourceLocationID); lerr == nil {
			sourceLocationName = location.Name
		}
	}

	records := make([]models.TransferRecord, 0, len(productIDs))
	updates := make([]models.TransferProductUpdate, 0, len(productIDs))
	for _, pid := range productIDs {
		records = append(records, models.TransferRecord{
			Type:              models.TransferTypeWarehouse,
			ProductID:         pid,
			FromWarehouseID:   sourceWarehouse.ID,
			FromWarehouseName: sourceWarehouse.Name,
			FromLocationID:    form.SourceLocationID,
			FromLocationName:  sourceLocationName,
			ToWarehouseID:     targetWarehouse.ID,
			ToWarehouseName:   targetWarehouse.Name,
			ToLocationID:      form.TargetLocationID,
			ToLocationName:    targetLocationName,
			Reason:            strings.TrimSpace(form.Reason),
			Remark:            strings.TrimSpace(form.Remark),
		})
		updates = append(updates, models.TransferProductUpdate{
			ProductID:       pid,
			ToWarehouseID:   targetWarehouse.ID,
			ToWarehouseName: targetWarehouse.Name,
			ToLocationID:    form.TargetLocationID,
			ToLocationName:  targetLocationName,
		})
	}
	if err := models.CreateTransferRecords(records, updates); err != nil {
		fail("移仓失败：" + err.Error())
		return
	}

	sourceDesc := sourceWarehouse.Name
	if sourceLocationName != "" {
		sourceDesc += " / " + sourceLocationName
	}
	targetDesc := targetWarehouse.Name
	if targetLocationName != "" {
		targetDesc += " / " + targetLocationName
	}
	detail := "移仓：" + strconv.Itoa(len(productIDs)) + " 个样品由「" + sourceDesc + "」移至「" + targetDesc + "」"
	if form.Reason != "" {
		detail += "，原因：" + form.Reason
	}
	recordOperation(ctx, "样品异动", "样品异动", detail, productNamesSummary(transferSampleNames(samples, productIDs)))
	ctx.Redirect(http.StatusFound, "/transfers?done="+strconv.Itoa(len(productIDs))+
		"&done_msg="+url.QueryEscape("已将 "+strconv.Itoa(len(productIDs))+" 个样品移至 "+targetDesc))
}

// renderPage 渲染样品异动页；errMsg 非空展示错误，done 非零展示成功提示
func (c *TransferController) renderPage(ctx *gin.Context, errMsg string, done [2]string) {
	users, err := models.ListEnabledUsers()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "error.html", gin.H{"msg": "加载用户列表失败"})
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
	// template.JS 标记为安全 JS，避免模板把 JSON 输出成带引号的字符串导致前端无法遍历
	safeLocationJSON := template.JS(locationBytes)

	doneQty, _ := strconv.Atoi(done[0])
	ctx.HTML(http.StatusOK, "transfer.html", userPageData(ctx, gin.H{
		"title":        "样品异动 - 库存管理系统",
		"users":        users,
		"warehouses":   warehouses,
		"locationJSON": safeLocationJSON,
		"ownerReasons": models.TransferOwnerReasons,
		"whReasons":    models.TransferWarehouseReasons,
		"error":        errMsg,
		"doneQty":      doneQty,
		"doneMsg":      done[1],
	}))
}

// parseTransferProductIDs 解析勾选的样品 ID（去重，非法值跳过）
func parseTransferProductIDs(raw []string) ([]uint, string) {
	if len(raw) == 0 {
		return nil, "请勾选要异动的样品"
	}
	ids := make([]uint, 0, len(raw))
	seen := make(map[uint]bool, len(raw))
	for _, s := range raw {
		id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
		if err != nil || id == 0 {
			continue
		}
		if !seen[uint(id)] {
			seen[uint(id)] = true
			ids = append(ids, uint(id))
		}
	}
	if len(ids) == 0 {
		return nil, "请勾选要异动的样品"
	}
	return ids, ""
}

// transferSampleNames 取勾选样品的名称（日志快照）
func transferSampleNames(samples []models.TransferSample, ids []uint) []string {
	want := make(map[uint]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	names := make([]string, 0, len(ids))
	for _, s := range samples {
		if want[s.ID] {
			names = append(names, s.Name)
		}
	}
	return names
}

// parseDone 解析成功提示参数（done 数量 + done_msg 文案）
func parseDone(ctx *gin.Context) [2]string {
	doneQty, _ := strconv.Atoi(ctx.DefaultQuery("done", "0"))
	if doneQty <= 0 {
		return [2]string{}
	}
	return [2]string{strconv.Itoa(doneQty), ctx.Query("done_msg")}
}
