package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// ImportTemplateHandler serves the device batch-import xlsx (backend alternative to frontend Blob download).
type ImportTemplateHandler struct{}

func NewImportTemplateHandler() *ImportTemplateHandler { return &ImportTemplateHandler{} }

var importHeaders = []string{
	"设备类型编码（必填）", "设备编码（可选）", "设备名称（必填）", "生命周期状态（可选）",
	"设备高度U（可选）", "资产编号（可选）", "序列号（可选）", "厂商（可选）",
	"型号（可选）", "规格描述（可选）", "固件版本（可选）", "采购批次（可选）",
	"保修到期日（可选）", "所属组织（可选）", "负责人（可选）", "联系方式（可选）",
	"业务系统（可选）", "应用名称（可选）", "宽度mm（可选）", "深度mm（可选）",
	"高度mm（可选）", "重量kg（可选）", "额定功率W（可选）", "峰值功率W（可选）",
	"输入电压V（可选）", "需要双路电源（可选）", "管理IP（可选）", "业务IP（可选）",
	"MAC地址（可选）", "管理协议（可选）", "监控状态（可选）", "外部二维码（可选）",
	"二维码链接（可选）", "标签（可选）", "备注（可选）",
}

var importExample = []interface{}{
	"SERVER", "SRV-APP-001", "应用服务器001", "WAITING_RACK",
	2, "ASSET-2026-001", "SN20260831001", "示例厂商",
	"MODEL-X2", "2U/双路CPU/256GB", "V1.2.0", "2026-Q3",
	"2029-08-31", "技术中心", "李四", "13800000000",
	"统一门户", "门户应用", 440, 800,
	88, 25, 800, 1200,
	220, "是", "192.168.10.21", "10.10.20.21",
	"00:11:22:33:44:55", "SNMP", "已纳管", "QR-2026-001",
	"https://example.com/qr/001", "生产,核心", "批量导入示例",
}

type fieldDoc struct {
	Name, Required, Type, Def, Rule, Example string
}

var importFieldDocs = []fieldDoc{
	{"设备类型编码", "必填", "文本", "", "必须与系统中启用的设备类型编码或名称匹配；导出的设备信息可直接再次导入", "SERVER"},
	{"设备编码", "可选", "文本", "", "可选；填写已有编码会更新已有设备，填写新编码会新增设备，留空时根据序列号/资产编号匹配，否则由系统自动生成", "SRV-APP-001"},
	{"设备名称", "必填", "文本", "", "最多 150 个字符；更新时留空保持原值", "应用服务器001"},
	{"生命周期状态", "可选", "枚举值", "WAITING_RACK", "WAITING_RACK / RUNNING / MAINTENANCE / PENDING_REMOVAL / OFF_RACK / SCRAPPED", "WAITING_RACK"},
	{"设备高度U", "可选", "整数", "", "1-100 的整数；新增时留空使用设备类型默认高度，更新时留空保持原值", "2"},
	{"资产编号", "可选", "文本", "", "建议设置为文本格式；可用于识别已有设备", "ASSET-2026-001"},
	{"序列号", "可选", "文本", "", "建议设置为文本格式；可用于识别已有设备", "SN20260831001"},
	{"厂商", "可选", "文本", "", "设备厂商", "示例厂商"},
	{"型号", "可选", "文本", "", "设备型号", "MODEL-X2"},
	{"规格描述", "可选", "文本", "", "CPU、内存、端口等规格", "2U/双路CPU/256GB"},
	{"固件版本", "可选", "文本", "", "固件或系统版本", "V1.2.0"},
	{"采购批次", "可选", "文本", "", "采购批次号", "2026-Q3"},
	{"保修到期日", "可选", "日期 YYYY-MM-DD", "", "YYYY-MM-DD", "2029-08-31"},
	{"所属组织", "可选", "文本", "", "资产归属组织", "技术中心"},
	{"负责人", "可选", "文本", "", "设备负责人", "李四"},
	{"联系方式", "可选", "文本", "", "电话或邮箱", "13800000000"},
	{"业务系统", "可选", "文本", "", "所属业务系统", "统一门户"},
	{"应用名称", "可选", "文本", "", "承载应用名称", "门户应用"},
	{"宽度mm", "可选", "数字", "", "填写时必须大于 0", "440"},
	{"深度mm", "可选", "数字", "", "填写时必须大于 0", "800"},
	{"高度mm", "可选", "数字", "", "填写时必须大于 0", "88"},
	{"重量kg", "可选", "数字", "", "不能为负数；新增时留空可使用类型默认值，更新时留空保持原值", "25"},
	{"额定功率W", "可选", "数字", "", "不能为负数；新增时留空可使用类型默认值，更新时留空保持原值", "800"},
	{"峰值功率W", "可选", "数字", "", "不能为负数；更新时留空保持原值", "1200"},
	{"输入电压V", "可选", "数字", "", "不能为负数；更新时留空保持原值", "220"},
	{"需要双路电源", "可选", "布尔值", "", "是/否、TRUE/FALSE、1/0；新增时留空使用类型默认值，更新时留空保持原值", "是"},
	{"管理IP", "可选", "文本", "", "合法 IPv4 或 IPv6 地址", "192.168.10.21"},
	{"业务IP", "可选", "文本", "", "合法 IPv4 或 IPv6 地址", "10.10.20.21"},
	{"MAC地址", "可选", "文本", "", "建议格式 AA:BB:CC:DD:EE:FF", "00:11:22:33:44:55"},
	{"管理协议", "可选", "文本", "", "如 SNMP、Redfish、SSH", "SNMP"},
	{"监控状态", "可选", "文本", "", "自定义监控状态", "已纳管"},
	{"外部二维码", "可选", "文本", "", "外部系统二维码编码", "QR-2026-001"},
	{"二维码链接", "可选", "文本", "", "外部二维码 URL", "https://example.com/qr/001"},
	{"标签", "可选", "文本", "", "多个标签可使用逗号分隔", "生产,核心"},
	{"备注", "可选", "文本", "", "补充说明", "批量导入示例"},
}

var importUsage = [][2]string{
	{"1", "请只在“导入数据”工作表中填写待导入数据，“填写示例”工作表不会被导入。"},
	{"2", "表头包含“必填/可选”标识，请勿修改或删除必填字段表头。"},
	{"3", "机柜/设备编码为可选字段：留空时由系统在导入提交时自动生成；设备导出信息再次导入时，已有设备编码会更新原设备，不会重复创建。序列号、资产编号、IP、MAC 等字段建议在 Excel 中设置为文本格式，避免前导 0 丢失。"},
	{"4", "枚举字段必须按“字段说明”工作表中的英文值填写；布尔值可填写：是/否、TRUE/FALSE、1/0。"},
	{"5", "导入前系统会校验重复标识与冲突；更新时空白可选字段默认保持原值。"},
	{"6", "本接口由 rebuild 后端生成，与前端浏览器内 Blob 下载内容等价，可按网络环境二选一。"},
}

func (h *ImportTemplateHandler) Download(c *gin.Context) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	dataSheet := "导入数据"
	_ = f.SetSheetName("Sheet1", dataSheet)
	exampleSheet := "填写示例"
	docSheet := "字段说明"
	usageSheet := "使用说明"
	_, _ = f.NewSheet(exampleSheet)
	_, _ = f.NewSheet(docSheet)
	_, _ = f.NewSheet(usageSheet)

	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#D9EAD3"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	for i, hcol := range importHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(dataSheet, cell, hcol)
		if style != 0 {
			_ = f.SetCellStyle(dataSheet, cell, cell, style)
		}
		_ = f.SetColWidth(dataSheet, cell[:1], cell[:1], 16)
	}
	// broader col widths by index
	for i := 1; i <= len(importHeaders); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		_ = f.SetColWidth(dataSheet, col, col, 18)
	}

	for i, hcol := range importHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(exampleSheet, cell, hcol)
		if style != 0 {
			_ = f.SetCellStyle(exampleSheet, cell, cell, style)
		}
	}
	for i, v := range importExample {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(exampleSheet, cell, v)
	}
	for i := 1; i <= len(importHeaders); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		_ = f.SetColWidth(exampleSheet, col, col, 18)
	}

	docHeaders := []string{"字段名称", "是否必填", "类型格式", "默认值", "可选值或填写说明", "示例"}
	for i, hcol := range docHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(docSheet, cell, hcol)
		if style != 0 {
			_ = f.SetCellStyle(docSheet, cell, cell, style)
		}
	}
	for r, d := range importFieldDocs {
		vals := []string{d.Name, d.Required, d.Type, d.Def, d.Rule, d.Example}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			_ = f.SetCellValue(docSheet, cell, v)
		}
	}
	_ = f.SetColWidth(docSheet, "A", "A", 16)
	_ = f.SetColWidth(docSheet, "B", "B", 10)
	_ = f.SetColWidth(docSheet, "C", "C", 14)
	_ = f.SetColWidth(docSheet, "D", "D", 14)
	_ = f.SetColWidth(docSheet, "E", "E", 48)
	_ = f.SetColWidth(docSheet, "F", "F", 20)

	_ = f.SetCellValue(usageSheet, "A1", "设备批量导入模板使用说明")
	if style != 0 {
		_ = f.SetCellStyle(usageSheet, "A1", "A1", style)
	}
	for i, row := range importUsage {
		_ = f.SetCellValue(usageSheet, fmt.Sprintf("A%d", i+2), row[0])
		_ = f.SetCellValue(usageSheet, fmt.Sprintf("B%d", i+2), row[1])
	}
	_ = f.SetColWidth(usageSheet, "A", "A", 8)
	_ = f.SetColWidth(usageSheet, "B", "B", 100)

	name := fmt.Sprintf("设备批量导入模板-%s.xlsx", time.Now().Format("20060102"))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, urlEncodeFilename(name)))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Cache-Control", "no-store")
	if err := f.Write(c.Writer); err != nil {
		// headers may already be sent
		return
	}
	c.Status(http.StatusOK)
}

func urlEncodeFilename(s string) string {
	const hex = "0123456789ABCDEF"
	b := []byte(s)
	out := make([]byte, 0, len(b)*3)
	for _, c := range b {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			out = append(out, c)
			continue
		}
		out = append(out, '%', hex[c>>4], hex[c&0xF])
	}
	return string(out)
}
