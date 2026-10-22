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

	KeyHintSidebar: "enter: open  s: select  d: structure",
	KeyHintEditor:  "ctrl+enter/f5: run  ctrl+p/n: history  esc: cancel",
	KeyHintResults: "[/]: width  o: sort  c: copy",
	KeyMenuHint:    "Esc menu   q quit   ? help",

	KeyHelpTitle:       "Keyboard shortcuts",
	KeyHelpGlobal:      "Global",
	KeyHelpSidebar:     "Connections",
	KeyHelpEditor:      "SQL editor",
	KeyHelpResults:     "Results",
	KeyHelpGlobalKeys:  "esc: command palette   q: quit   ?: help   tab/shift+tab: panes   ctrl+c: quit",
	KeyHelpSidebarKeys: "↑/↓: move   enter: connect or expand   s: select table   d: table structure   r: reload",
	KeyHelpEditorKeys:  "ctrl+enter or f5: execute   ctrl+p: previous query   ctrl+n: next query   esc: cancel",
	KeyHelpResultsKeys: "arrows/hjkl: move cell   pgup/pgdn: page   [/]: column width   o: sort   c: copy   ctrl+e/j/m: export",
	KeyHelpClose:       "press ? or esc to close",
	KeyHelpCommands:    "Commands",
	KeyHelpCommandsTip: "press ':' to open the command palette",

	KeyCommandTitle: "Command palette",
	KeyCommandHint:  "enter: run   tab: complete   esc: close",

	KeyCmdConnect:    "connect to a configured connection",
	KeyCmdDisconnect: "close the current connection",
	KeyCmdExport:     "export current results (csv or json)",
	KeyCmdFocus:      "move focus to sidebar, editor or results",
	KeyCmdLang:       "switch UI language (en or zh)",
	KeyCmdHelp:       "show keyboard shortcuts and commands",
	KeyCmdReload:     "reconnect the current connection",
	KeyCmdQuit:       "quit bdb",
	KeyCmdDescribe:   "show structure of the selected table",
	KeyCmdRun:        "execute a SQL file",

	KeyDetailTitle:       "Table structure",
	KeyDetailColumns:     "Columns",
	KeyDetailIndexes:     "Indexes",
	KeyDetailForeignKeys: "Foreign keys",
	KeyDetailNone:        "(none)",
	KeyDetailHint:        "esc: close   ↑/↓: scroll   g/G: top/bottom",

	KeyErrorDetail:  "failed to load table structure: %v",
	KeyErrorScript:  "script failed at statement %d: %v",
	KeyStatusCopied: "Copied to clipboard",
	KeyStatusScript: "%d/%d statement(s) in %s",

	KeyErrorUsage: "usage: %s",
	KeyStatusLang: "Language: %s",

	KeyErrorConnect:      "connection failed: %v",
	KeyErrorQuery:        "query failed: %v",
	KeyErrorExport:       "export failed: %v",
	KeyErrorNotSupported: "database type %q is not implemented yet",
	KeyErrorCatalog:      "failed to load metadata: %v",

	KeyResultsPrompt:    "Run a query to see results",
	KeyTerminalTooSmall: "Terminal too small — resize to at least 40x10",
	KeyNoConnections:    "No connections configured; edit ~/.config/bdb/config.yaml",

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

	KeyHintSidebar: "enter: 打开  s: 选中  d: 结构",
	KeyHintEditor:  "ctrl+enter/f5: 执行  ctrl+p/n: 历史  esc: 取消",
	KeyHintResults: "[/]: 列宽  o: 排序  c: 复制",
	KeyMenuHint:    "Esc 菜单   q 退出   ? 帮助",

	KeyHelpTitle:       "键盘快捷键",
	KeyHelpGlobal:      "全局",
	KeyHelpSidebar:     "连接",
	KeyHelpEditor:      "SQL 编辑器",
	KeyHelpResults:     "结果",
	KeyHelpGlobalKeys:  "esc: 命令面板   q: 退出   ?: 帮助   tab/shift+tab: 面板   ctrl+c: 退出",
	KeyHelpSidebarKeys: "↑/↓: 移动   enter: 连接或展开   s: 选中表   d: 表结构   r: 重载",
	KeyHelpEditorKeys:  "ctrl+enter 或 f5: 执行   ctrl+p: 上一条   ctrl+n: 下一条   esc: 取消",
	KeyHelpResultsKeys: "方向键/hjkl: 移动单元格   pgup/pgdn: 翻页   [/]: 列宽   o: 排序   c: 复制   ctrl+e/j/m: 导出",
	KeyHelpClose:       "按 ? 或 esc 关闭",
	KeyHelpCommands:    "命令",
	KeyHelpCommandsTip: "按 ':' 打开命令面板",

	KeyCommandTitle: "命令面板",
	KeyCommandHint:  "enter: 执行   tab: 补全   esc: 关闭",

	KeyCmdConnect:    "连接到指定连接",
	KeyCmdDisconnect: "断开当前连接",
	KeyCmdExport:     "导出当前结果（csv 或 json）",
	KeyCmdFocus:      "切换焦点到侧栏、编辑器或结果",
	KeyCmdLang:       "切换界面语言（en 或 zh）",
	KeyCmdHelp:       "显示快捷键与命令帮助",
	KeyCmdReload:     "重新连接当前连接",
	KeyCmdQuit:       "退出 bdb",
	KeyCmdDescribe:   "查看选中表的结构",
	KeyCmdRun:        "执行 SQL 文件",

	KeyDetailTitle:       "表结构",
	KeyDetailColumns:     "列",
	KeyDetailIndexes:     "索引",
	KeyDetailForeignKeys: "外键",
	KeyDetailNone:        "（无）",
	KeyDetailHint:        "esc: 关闭   ↑/↓: 滚动   g/G: 顶部/底部",

	KeyErrorDetail:  "加载表结构失败：%v",
	KeyErrorScript:  "脚本第 %d 条语句失败：%v",
	KeyStatusCopied: "已复制到剪贴板",
	KeyStatusScript: "%d/%d 条语句，耗时 %s",

	KeyErrorUsage: "用法：%s",
	KeyStatusLang: "界面语言：%s",

	KeyErrorConnect:      "连接失败：%v",
	KeyErrorQuery:        "查询失败：%v",
	KeyErrorExport:       "导出失败：%v",
	KeyErrorNotSupported: "数据库类型 %q 尚未实现",
	KeyErrorCatalog:      "加载元数据失败：%v",

	KeyResultsPrompt:    "执行查询后在这里查看结果",
	KeyTerminalTooSmall: "终端窗口过小，请调整到至少 40x10",
	KeyNoConnections:    "尚未配置连接，请编辑 ~/.config/bdb/config.yaml",

	KeyObjectTable:  "表",
	KeyObjectView:   "视图",
	KeyObjectColumn: "列",

	KeyQuerySelectTable: "SELECT * FROM %s LIMIT 100;",

	KeyNoRows:      "没有数据",
	KeyResultsNull: "NULL",
}
