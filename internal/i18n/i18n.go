package i18n

import (
	"fmt"
	"os"
	"strings"
)

type Lang string

const (
	En Lang = "en"
	Zh Lang = "zh"
)

type Translator struct {
	lang Lang
}

func New(setting string) *Translator {
	switch strings.ToLower(strings.TrimSpace(setting)) {
	case "zh", "zh-cn", "zh_cn", "chinese":
		return &Translator{lang: Zh}
	case "en", "english":
		return &Translator{lang: En}
	}
	locale := os.Getenv("LC_ALL")
	if locale == "" {
		locale = os.Getenv("LANG")
	}
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return &Translator{lang: Zh}
	}
	return &Translator{lang: En}
}

func (t *Translator) Lang() Lang { return t.lang }

func (t *Translator) SetLang(lang Lang) {
	if lang == En || lang == Zh {
		t.lang = lang
	}
}

func (t *Translator) T(key string, args ...any) string {
	dict := en
	if t.lang == Zh {
		dict = zh
	}
	s, ok := dict[key]
	if !ok {
		s, ok = en[key]
		if !ok {
			return key
		}
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

var en = map[string]string{
	KeyAppTitle: "bdb",

	KeyPaneConnections: "Connections",
	KeyPaneSQL:         "SQL",
	KeyPaneResults:     "Results",

	KeyStatusReady:         "Ready",
	KeyStatusConnecting:    "Connecting to %s…",
	KeyStatusConnected:     "Connected to %s",
	KeyStatusDisconnected:  "Disconnected",
	KeyStatusRunning:       "Running…",
	KeyStatusCancelled:     "Query cancelled",
	KeyStatusTimeout:       "Query timed out",
	KeyStatusNoConnection:  "No connection",
	KeyStatusEmptyQuery:    "Nothing to run",
	KeyStatusRows:          "%d row(s) in %s",
	KeyStatusRowsTruncated: "%d row(s) in %s (truncated)",
	KeyStatusAffected:      "%d row(s) affected in %s",
	KeyStatusExported:      "Exported to %s",
	KeyStatusNoResults:     "No results to export",
	KeyStatusLoading:       "Loading…",

	KeyHintSidebar:        "enter: menu  s: select  d: structure  a: add",
	KeyHintEditor:         "ctrl+enter/f5: run  ctrl+space: complete  esc: cancel",
	KeyHintResults:        "enter/v: cell  e: edit  x: del  H/L: scroll",
	KeyMenuBarHint:        "Esc menu   F2 layout   q quit   ? help",
	KeyMenuTitle:          "Options",
	KeyMenuHint:           "↑/↓: move   ←/→: change   enter: run   esc: close",
	KeyMenuActions:        "Actions",
	KeyMenuSettings:       "Settings",
	KeyMenuExportCSV:      "Export CSV",
	KeyMenuExportJSON:     "Export JSON",
	KeyMenuExportMarkdown: "Export Markdown",
	KeyMenuTheme:          "Theme",
	KeyMenuLanguage:       "Language",
	KeyMenuTimeout:        "Query timeout",
	KeyMenuMaxRows:        "Max rows",
	KeyMenuLayout:         "Toggle focused layout",

	KeyTableData:               "View data",
	KeyTableDDL:                "View DDL",
	KeyTableStructure:          "View structure",
	KeyTableIndexes:            "View indexes",
	KeyTableSelect:             "Generate SELECT",
	KeyTableDDLTitle:           "DDL: %s",
	KeyTableIndexesTitle:       "Indexes: %s",
	KeyTextViewHint:            "esc: close   ↑/↓: scroll   g/G: top/bottom",
	KeyErrorDDL:                "failed to load DDL: %v",
	KeyErrorIndexes:            "failed to load indexes: %v",
	KeyCellTitle:               "Cell: %s",
	KeyCellEditTitle:           "Edit %s",
	KeyCellEditHint:            "enter: preview   esc: cancel",
	KeyConfirmUpdate:           "Confirm update",
	KeyConfirmDelete:           "Confirm delete",
	KeyConfirmHint:             "y/enter: execute   n/esc: cancel",
	KeyErrorEditSource:         "editing is only available for table data views",
	KeyErrorNoPrimaryKey:       "this table has no primary key; editing is disabled",
	KeyErrorReadOnly:           "connection is read-only",
	KeyErrorMutation:           "write failed: %v",
	KeyErrorEdit:               "failed to load table info: %v",
	KeyErrorUnsupportedFeature: "not supported for %s",
	KeyStatusUpdated:           "1 row updated",
	KeyStatusDeleted:           "1 row deleted",
	KeyMenuInsert:              "Insert row template",
	KeyQueryInsertTemplate:     "INSERT INTO %s (%s) VALUES (%s);",

	KeyHelpTitle:       "Keyboard shortcuts",
	KeyHelpGlobal:      "Global",
	KeyHelpSidebar:     "Connections",
	KeyHelpEditor:      "SQL editor",
	KeyHelpResults:     "Results",
	KeyHelpGlobalKeys:  "esc: options menu   f2: focused layout   q: quit   ?: help   tab/shift+tab: panes   ctrl+c: quit",
	KeyHelpSidebarKeys: "↑/↓: move   enter: table actions   s: select table   d: structure   a: add source   r: reload",
	KeyHelpEditorKeys:  "ctrl+enter or f5: execute   ctrl+space: complete   ctrl+p: previous query   ctrl+n: next query   esc: cancel",
	KeyHelpResultsKeys: "arrows/hjkl: move cell   H/L: scroll columns   enter/v: view full cell   e: edit cell   x: delete row   [/]: width   o: sort   c: copy   ctrl+e/j/m: export",
	KeyHelpClose:       "press ? or esc to close",
	KeyHelpCommands:    "Commands",
	KeyHelpCommandsTip: "press ':' to open the command palette",

	KeyCommandTitle: "Command palette",
	KeyCommandHint:  "enter: run   tab: complete   esc: close",

	KeyCompletionTitle: "Completions",
	KeyCompletionHint:  "tab/enter: accept   esc: close",

	KeyCmdConnect:    "connect to a configured connection",
	KeyCmdDisconnect: "close the current connection",
	KeyCmdExport:     "export current results (csv or json)",
	KeyCmdFocus:      "move focus to sidebar, editor or results",
	KeyCmdLang:       "switch UI language (en or zh)",
	KeyCmdTheme:      "switch color theme (use the next theme or a name)",
	KeyCmdHelp:       "show keyboard shortcuts and commands",
	KeyCmdReload:     "reconnect the current connection",
	KeyCmdQuit:       "quit bdb",
	KeyCmdDescribe:   "show structure of the selected table",
	KeyCmdRun:        "execute a SQL file",
	KeyCmdAdd:        "add a new data source",

	KeyFormTitle:             "New connection",
	KeyFormType:              "Type",
	KeyFormName:              "Name",
	KeyFormHost:              "Host",
	KeyFormPort:              "Port",
	KeyFormUser:              "User",
	KeyFormPassword:          "Password",
	KeyFormDatabase:          "Database",
	KeyFormSSLMode:           "SSL mode",
	KeyFormPath:              "Path",
	KeyFormAddr:              "Addr",
	KeyFormDB:                "DB",
	KeyFormURI:               "URI",
	KeyFormHint:              "↑/↓ or tab: next   enter: next/save   esc: cancel",
	KeyFormRequired:          "%s is required",
	KeyFormInvalidPort:       "invalid port %q",
	KeyFormInvalidDB:         "invalid database number %q",
	KeyFormDuplicate:         "connection %q already exists",
	KeyStatusConnectionAdded: "Saved connection %s",
	KeyErrorSaveConnection:   "failed to save connection: %v",
	KeyErrorLoadConnections:  "failed to load saved connections: %v",

	KeyDetailTitle:       "Table structure",
	KeyDetailColumns:     "Columns",
	KeyDetailIndexes:     "Indexes",
	KeyDetailForeignKeys: "Foreign keys",
	KeyDetailNone:        "(none)",
	KeyDetailHint:        "esc: close   ↑/↓: scroll   g/G: top/bottom",

	KeyErrorDetail:             "failed to load table structure: %v",
	KeyErrorScript:             "script failed at statement %d: %v",
	KeyStatusCopied:            "Copied to clipboard",
	KeyStatusScript:            "%d/%d statement(s) in %s",
	KeyStatusCompletionLoading: "Loading columns…",

	KeyErrorUsage:       "usage: %s",
	KeyStatusLang:       "Language: %s",
	KeyStatusTheme:      "Theme: %s",
	KeyStatusTimeoutSet: "Query timeout: %s",
	KeyStatusMaxRowsSet: "Max rows: %d",
	KeyErrorTheme:       "unknown theme %q; available: %s",
	KeyHelpThemes:       "themes: %s (use :theme <name>)",

	KeyErrorConnect:      "connection failed: %v",
	KeyErrorQuery:        "query failed: %v",
	KeyErrorExport:       "export failed: %v",
	KeyErrorNotSupported: "database type %q is not implemented yet",
	KeyErrorCatalog:      "failed to load metadata: %v",

	KeyResultsPrompt:    "Run a query to see results",
	KeyTerminalTooSmall: "Terminal too small — resize to at least 40x10",
	KeyNoConnections:    "No connections configured; edit %s or press a to add one",

	KeyObjectTable:  "table",
	KeyObjectView:   "view",
	KeyObjectColumn: "column",

	KeyQuerySelectTable: "SELECT * FROM %s LIMIT 100;",

	KeyNoRows:      "No rows",
	KeyResultsNull: "NULL",
}

var zh = map[string]string{
	KeyAppTitle: "bdb",

	KeyPaneConnections: "连接",
	KeyPaneSQL:         "SQL",
	KeyPaneResults:     "结果",

	KeyStatusReady:         "就绪",
	KeyStatusConnecting:    "正在连接 %s…",
	KeyStatusConnected:     "已连接 %s",
	KeyStatusDisconnected:  "已断开",
	KeyStatusRunning:       "执行中…",
	KeyStatusCancelled:     "查询已取消",
	KeyStatusTimeout:       "查询超时",
	KeyStatusNoConnection:  "未连接",
	KeyStatusEmptyQuery:    "没有可执行的语句",
	KeyStatusRows:          "%d 行，耗时 %s",
	KeyStatusRowsTruncated: "%d 行，耗时 %s（已截断）",
	KeyStatusAffected:      "影响 %d 行，耗时 %s",
	KeyStatusExported:      "已导出到 %s",
	KeyStatusNoResults:     "没有可导出的结果",
	KeyStatusLoading:       "加载中…",

	KeyHintSidebar:        "enter: 菜单  s: 选中  d: 结构  a: 添加",
	KeyHintEditor:         "ctrl+enter/f5: 执行  ctrl+space: 补全  esc: 取消",
	KeyHintResults:        "enter/v: 查看  e: 编辑  x: 删除  H/L: 横滚",
	KeyMenuBarHint:        "Esc 菜单   F2 布局   q 退出   ? 帮助",
	KeyMenuTitle:          "选项",
	KeyMenuHint:           "↑/↓: 移动   ←/→: 调整   enter: 执行   esc: 关闭",
	KeyMenuActions:        "操作",
	KeyMenuSettings:       "设置",
	KeyMenuExportCSV:      "导出 CSV",
	KeyMenuExportJSON:     "导出 JSON",
	KeyMenuExportMarkdown: "导出 Markdown",
	KeyMenuTheme:          "主题",
	KeyMenuLanguage:       "语言",
	KeyMenuTimeout:        "查询超时",
	KeyMenuMaxRows:        "最大行数",
	KeyMenuLayout:         "切换专注布局",

	KeyTableData:               "查看数据",
	KeyTableDDL:                "查看 DDL",
	KeyTableStructure:          "查看结构",
	KeyTableIndexes:            "查看索引",
	KeyTableSelect:             "生成 SELECT",
	KeyTableDDLTitle:           "DDL：%s",
	KeyTableIndexesTitle:       "索引：%s",
	KeyTextViewHint:            "esc: 关闭   ↑/↓: 滚动   g/G: 顶部/底部",
	KeyErrorDDL:                "加载 DDL 失败：%v",
	KeyErrorIndexes:            "加载索引失败：%v",
	KeyCellTitle:               "单元格：%s",
	KeyCellEditTitle:           "编辑 %s",
	KeyCellEditHint:            "enter: 预览   esc: 取消",
	KeyConfirmUpdate:           "确认更新",
	KeyConfirmDelete:           "确认删除",
	KeyConfirmHint:             "y/enter: 执行   n/esc: 取消",
	KeyErrorEditSource:         "只有通过“查看数据”打开的结果可以编辑",
	KeyErrorNoPrimaryKey:       "该表没有主键，无法编辑",
	KeyErrorReadOnly:           "连接为只读模式",
	KeyErrorMutation:           "写入失败：%v",
	KeyErrorEdit:               "加载表信息失败：%v",
	KeyErrorUnsupportedFeature: "%s 不支持该操作",
	KeyStatusUpdated:           "已更新 1 行",
	KeyStatusDeleted:           "已删除 1 行",
	KeyMenuInsert:              "插入行模板",
	KeyQueryInsertTemplate:     "INSERT INTO %s (%s) VALUES (%s);",

	KeyHelpTitle:       "键盘快捷键",
	KeyHelpGlobal:      "全局",
	KeyHelpSidebar:     "连接",
	KeyHelpEditor:      "SQL 编辑器",
	KeyHelpResults:     "结果",
	KeyHelpGlobalKeys:  "esc: 选项菜单   f2: 专注布局   q: 退出   ?: 帮助   tab/shift+tab: 面板   ctrl+c: 退出",
	KeyHelpSidebarKeys: "↑/↓: 移动   enter: 表操作   s: 选中表   d: 表结构   a: 添加数据源   r: 重载",
	KeyHelpEditorKeys:  "ctrl+enter 或 f5: 执行   ctrl+space: 补全   ctrl+p: 上一条   ctrl+n: 下一条   esc: 取消",
	KeyHelpResultsKeys: "方向键/hjkl: 移动单元格   H/L: 横向滚动   enter/v: 查看完整内容   e: 编辑单元格   x: 删除行   [/]: 列宽   o: 排序   c: 复制   ctrl+e/j/m: 导出",
	KeyHelpClose:       "按 ? 或 esc 关闭",
	KeyHelpCommands:    "命令",
	KeyHelpCommandsTip: "按 ':' 打开命令面板",

	KeyCommandTitle: "命令面板",
	KeyCommandHint:  "enter: 执行   tab: 补全   esc: 关闭",

	KeyCompletionTitle: "补全",
	KeyCompletionHint:  "tab/enter: 接受   esc: 关闭",

	KeyCmdConnect:    "连接到指定连接",
	KeyCmdDisconnect: "断开当前连接",
	KeyCmdExport:     "导出当前结果（csv 或 json）",
	KeyCmdFocus:      "切换焦点到侧栏、编辑器或结果",
	KeyCmdLang:       "切换界面语言（en 或 zh）",
	KeyCmdTheme:      "切换配色主题（切换到下一个或指定名称）",
	KeyCmdHelp:       "显示快捷键与命令帮助",
	KeyCmdReload:     "重新连接当前连接",
	KeyCmdQuit:       "退出 bdb",
	KeyCmdDescribe:   "查看选中表的结构",
	KeyCmdRun:        "执行 SQL 文件",
	KeyCmdAdd:        "添加新的数据源",

	KeyFormTitle:             "新建连接",
	KeyFormType:              "类型",
	KeyFormName:              "名称",
	KeyFormHost:              "主机",
	KeyFormPort:              "端口",
	KeyFormUser:              "用户",
	KeyFormPassword:          "密码",
	KeyFormDatabase:          "数据库",
	KeyFormSSLMode:           "SSL 模式",
	KeyFormPath:              "路径",
	KeyFormAddr:              "地址",
	KeyFormDB:                "数据库编号",
	KeyFormURI:               "URI",
	KeyFormHint:              "↑/↓ 或 tab: 切换   enter: 下一个/保存   esc: 取消",
	KeyFormRequired:          "请填写%s",
	KeyFormInvalidPort:       "端口无效：%q",
	KeyFormInvalidDB:         "数据库编号无效：%q",
	KeyFormDuplicate:         "连接 %q 已存在",
	KeyStatusConnectionAdded: "已保存连接 %s",
	KeyErrorSaveConnection:   "保存连接失败：%v",
	KeyErrorLoadConnections:  "加载已保存连接失败：%v",

	KeyDetailTitle:       "表结构",
	KeyDetailColumns:     "列",
	KeyDetailIndexes:     "索引",
	KeyDetailForeignKeys: "外键",
	KeyDetailNone:        "（无）",
	KeyDetailHint:        "esc: 关闭   ↑/↓: 滚动   g/G: 顶部/底部",

	KeyErrorDetail:             "加载表结构失败：%v",
	KeyErrorScript:             "脚本第 %d 条语句失败：%v",
	KeyStatusCopied:            "已复制到剪贴板",
	KeyStatusScript:            "%d/%d 条语句，耗时 %s",
	KeyStatusCompletionLoading: "正在加载列…",

	KeyErrorUsage:       "用法：%s",
	KeyStatusLang:       "界面语言：%s",
	KeyStatusTheme:      "主题：%s",
	KeyStatusTimeoutSet: "查询超时：%s",
	KeyStatusMaxRowsSet: "最大行数：%d",
	KeyErrorTheme:       "未知主题 %q；可用：%s",
	KeyHelpThemes:       "主题：%s（使用 :theme <名称>）",

	KeyErrorConnect:      "连接失败：%v",
	KeyErrorQuery:        "查询失败：%v",
	KeyErrorExport:       "导出失败：%v",
	KeyErrorNotSupported: "数据库类型 %q 尚未实现",
	KeyErrorCatalog:      "加载元数据失败：%v",

	KeyResultsPrompt:    "执行查询后在这里查看结果",
	KeyTerminalTooSmall: "终端窗口过小，请调整到至少 40x10",
	KeyNoConnections:    "尚未配置连接，请编辑 %s 或按 a 添加",

	KeyObjectTable:  "表",
	KeyObjectView:   "视图",
	KeyObjectColumn: "列",

	KeyQuerySelectTable: "SELECT * FROM %s LIMIT 100;",

	KeyNoRows:      "没有数据",
	KeyResultsNull: "NULL",
}
