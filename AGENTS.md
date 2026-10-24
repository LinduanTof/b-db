# AGENTS.md

## 项目简介

`bdb` 是一款使用 Go + Bubble Tea v2 构建的终端数据库客户端(TUI)。单个二进制支持 PostgreSQL、MySQL、SQLite、Redis 和 MongoDB:连接浏览、SQL 编辑执行、事务化单元格编辑、表结构查看,以及 CSV/JSON/Markdown 导出。面向用户的 README 和大部分 UI 文案为中英双语。

## 常用命令

```bash
make demo-db      # 根据 testdata/schema.sql 生成 testdata/demo.db(需要 sqlite3 命令)
make build        # 生成 bin/bdb,启用 trimpath 与 -s -w
make run          # go run ./cmd/bdb
make test         # 先执行 demo-db,再执行 go test ./...
make vet
go test ./...     # 需要 testdata/demo.db 已经存在
./bin/bdb --config config.example.yaml
./bin/bdb --config config.yaml ./migrations/001_init.sql   # 可选脚本,连接后自动执行
```

- 需要 Go 1.26+(`go.mod` 中为 `go 1.26.4`)。CI 使用 `go.mod` 中的版本,在 `main` 与 `dev` 上执行 build、vet、生成 demo 数据库以及 `-race` 测试。
- 全新检出后请用 `make test`。`internal/app` 的测试会直接读取 `testdata/demo.db`,文件不存在时会 `t.Fatal`。
- 基准测试:`go test -bench . -benchmem ./internal/ui/results ./internal/ui/editor`
- 发布:`make release`(linux amd64/arm64,关闭 CGO)或 `goreleaser release --clean`。
- 集成数据库:`docker compose -f deploy/docker-compose.yml up -d`;可用 `BDB_PG_PORT`、`BDB_MYSQL_PORT`、`BDB_REDIS_PORT`、`BDB_MONGO_PORT` 覆盖端口。常规测试不依赖外部数据库(SQL 测试使用临时 SQLite)。

## 架构说明

### 入口与启动流程

`cmd/bdb/main.go`:

1. 解析 `--config`、`--version` 以及一个可选的位置参数 SQL 脚本。
2. 若 `$XDG_CONFIG_HOME/bdb/config.yaml` 不存在,创建带注释的默认配置(权限 `0600`),绝不覆盖已有文件。
3. 加载并校验配置,输出非致命警告,创建 i18n 翻译器。
4. 构造根 `app.Model`,加载查询历史,启动 Bubble Tea。

### TUI 模型(`internal/app/model.go`)

根 `Model` 位于 `internal/app/model.go`(约 1600 行),持有所有子模型、焦点状态、浮层、会话、连接与查询编排;渲染/布局在 `render.go`,编辑与变更流程在 `edit.go`。

- `Update` 将 `tea.WindowSizeMsg` 分发给所有浮层并调用 `layout()`,按键交给 `handleKey`,鼠标事件单独处理,自定义消息交给对应处理器。未识别的消息最终落到 `editor.Update`。
- 浮层优先级在三个地方重复维护:`handleKey`、`render()`、`anyOverlayOpen()`。顺序为:选项菜单、表操作菜单、文本查看器、结构详情、连接表单、单元格编辑、确认框、帮助。新增模态框时必须同时接入这三处,以及 `layout()`/`SetSize` 和 `closeOverlays()`。
- `View()` 返回 `tea.NewView(m.render())`,启用备用屏幕、单元格级鼠标事件和 Kitty 键盘增强协议(用于区分 `Ctrl+Enter` 与 `Enter`)。
- 异步操作放在返回类型化消息的 `tea.Cmd` 闭包中(`connOpenedMsg`、`queryDoneMsg`、`tablePageDoneMsg`、`mutationDoneMsg` 等),处理器回到根模型执行。查询带 `querySeq` 序号,过期结果会被丢弃。
- 表数据浏览与编辑器查询是两条独立路径:前者走 `pagination.go` 的 `runPageQuery`/`tablePageDoneMsg`,维护 `pagination` 并设置可编辑来源,不读写编辑器;后者走 `runQuery`/`queryDoneMsg`,只处理编辑器内容。两者共享结果面板,编辑器查询成功后会清除分页状态。
- 侧栏 `↑`/`↓` 只移动光标,`Enter` 才激活节点。表节点激活(`sidebar.TableActivateMsg`)会新建或复用同名标签(`editorTab.source`)并预填 SQL,同时通过分页路径加载数据;`s` 只预览数据,`m` 打开当前节点(连接/数据库/表)的操作菜单(`sidebar.NodeMenuMsg`),`o` 只写入编辑器而不执行。连接根激活显示连接信息,数据库节点激活显示表概览,二者通过 `Model.context`(`contextInfo`/`contextTables`/`contextData`/`contextQuery`)决定结果面板标题、提示和 `r` 的行为。
- 表概览原始数据保存在 `overviewAll`,客户端过滤词在 `overviewFilter`;`rebuildOverview` 子串匹配前三列后重新注入结果面板。切换会话或断开时必须调用 `resetOverviewState`,否则旧概览会串到新连接;`r` 重新加载后会自动重新应用过滤词。服务端版本异步加载,`versionSeq` 防止旧连接的版本覆盖当前面板。
- 布局是手动计算的:`layout()` 计算 `sideW`、`rightW`、`editorH`、`resultsH`;终端小于 20x6 时直接返回,小于 40x10 时渲染 "terminal too small" 提示。焦点模式有 `manual`(F2 切换缩放)和 `auto`(缩放跟随焦点,F2 暂停)。`ui.layout` 支持 `stacked`(默认)、`columns`(三栏,右侧宽度不足 60 列时回退 stacked,结果记在 `columnsActive`)和 `results-top`(结果在上);菜单切换会写入状态文件,启动时状态文件优先于配置。
- 会话保存在 `map[string]*session` 中。切换连接时保存当前编辑器/结果/补全器/编辑缓存/标签/分页/结果面板上下文/概览数据与过滤词/侧栏光标,并恢复目标会话。任何新的按连接保存的 UI 状态都必须加入 `saveCurrentSession` 和 `activateSession`,否则会在连接之间串数据。
- 侧栏光标用 `sidebar.CursorPath()`/`RestoreCursor(path)` 按节点路径恢复;`pendingCursor` 会保留到树中出现匹配节点,以覆盖表/列节点的异步加载。切换会话时仅当目标会话上次停留在 `contextTables` 才重新加载概览,否则保留上次的表数据或查询结果。关闭或断开后必须调用 `sidebar.CollapseSession` 收起对应节点:断开连接(`disconnectConnection`)会关闭同 base 的全部派生会话并折叠连接根,关闭单个会话只折叠其数据库节点。
- 侧栏的活动会话标记由 `syncSidebarSessions` 注入(`sessions` 的 key 加当前连接)。新增连接、断开、切换或重置会话后必须调用它,否则侧栏的 `●`/`◉` 状态和活动数量会过期。
- PostgreSQL、MySQL、MongoDB 连接成功后会在连接节点下列出所有数据库,并在内存中克隆出 `<连接名>/<数据库名>` 派生会话;每个数据库是独立会话。`SupportsDatabases` 决定哪些类型使用数据库列表,`databaseIsSchema` 决定数据库节点下直接加载表(MySQL/MongoDB)还是 schema(PostgreSQL)。数据库列表在连接切换后保留,点击连接根只展开列表,点击数据库节点才切换会话。派生配置只存在于 `cfg.Connections` 中,不写入运行时连接文件。数据库发现由 `db.DatabaseLister` 可选接口完成。
- SQL 编辑器支持按连接隔离的多标签(`tabs.go`);标签内容通过 `tab_state.go` 自动持久化到 `$XDG_STATE_HOME/bdb/editor_tabs.yaml`(0600,临时文件 + rename)。新增需要持久化的标签字段时同步修改 `savedTab`/`savedTabs` 与 `collectTabStates`;仅内存使用的 `editorTab.source` 故意不持久化,重启后表标签去重不会命中。
- 编辑器获得焦点时 `q` 不退出;`Ctrl+C` 始终退出。浮层打开时 `q` 退出,其他按键先交给浮层处理。

### UI 组件(`internal/ui/*`)

每个组件是独立包,模型为值类型:

```go
func New(t *i18n.Translator) Model
func (m *Model) SetSize(width, height int)
func (m *Model) Update(...) (Model, tea.Cmd)
func (m *Model) View() string
```

模态组件额外提供 `Open`、`Close`、`IsOpen`。根模型使用 `*Model` 并原地修改;子模型通常返回值拷贝。浮层通过 `textutil.Overlay` 组合,支持 ANSI 样式和 CJK 宽度。

渲染规则:绝不能用 `len`/`utf8.RuneCountInString` 计算终端宽度。使用 `textutil.Width/Truncate/FitLine/FitLines`(底层为 `charmbracelet/x/ansi`),结果表格使用 `go-runewidth`。UI 测试通过 `ansi.StringWidth` 校验精确行数和单元格宽度。

### 数据库层(`internal/db`)

- `types.go` 定义 `Driver`、`Conn`、`Catalog`、`Querier`、`Mutator` 接口和能力位。
- `registry.go` 是全局注册表。驱动在 `init()` 中自注册:SQL 驱动位于 `internal/db/sql/conn.go`,Redis/Mongo 在各自包内。它们通过 `internal/app/model.go` 中的匿名导入激活。没有集中的类型 switch;新增驱动意味着实现接口、在 `init()` 注册、并添加匿名导入。
- SQL(PostgreSQL/MySQL/SQLite)共用 `internal/db/sql`;各数据库差异通过 `catalog.go` 中的分支实现。标识符引用/占位符在 `internal/db/dialect`。SQLite 使用纯 Go 的 `modernc.org/sqlite`,因此发布时可以 `CGO_ENABLED=0`;`:memory:` 数据库会强制单连接,避免连接池中每个连接看到独立的内存库。
- `internal/query/executor.go` 根据顶层语句类型选择 `Query` 或 `Exec`:识别 `WITH` 的 CTE 体、`RETURNING`、括号包裹的查询以及 PostgreSQL dollar-quoting。`query.IsWrite` 用于只读连接的保守写入拦截。Redis/Mongo 设置 `AlwaysQuery`,因为它们所谓的 "SQL" 实际是命令前端。
- 只有从表视图打开、且表有主键的结果才允许编辑。SQL 写入在事务中执行,影响行数必须恰好为 1;预览 SQL 通过 dialect 生成。`read_only: true` 会在 app 层阻止单元格编辑和 SQL 写入语句,但这是保守的客户端检查,不能替代数据库侧的只读账号。
- 可选能力通过接口断言实现,不要把它们加进 `Conn`:服务端版本用 `db.Versioner`,表统计用 `db.TableStatProvider`(PG/MySQL 实现,SQLite 返回 nil)。统计包含行数估算、大小、列数与表注释;失败只降级为 `-`,不影响表列表。
- 大结果集导出必须用 `db.Streamer`/`db.RowStream`(SQL 驱动实现),不要用 `Querier.Query` 把整表载入内存。`RowStream` 是前向游标,调用方负责 `Close`;导出总超时 30 分钟,连接池上限为 4,库级导出逐表打开并在写完后立即关闭游标。XLSX 单 sheet 上限 1,048,575 行,超出返回错误。
- 表/库节点菜单的导出由 `app/export_data.go` 编排:导出在 `tea.Cmd` 中后台执行,逐表读取 DDL 与流式数据。库级导出优先复用当前会话连接,其次复用 `sessions` 中的连接,否则临时 `db.Open` 并在结束后关闭。CSV 输出目录,SQL/JSON/Excel 单文件。XLSX 由 `internal/export/xlsx.go` 手写 OOXML,不引入第三方依赖。进度通过 `Model.SetSender` 注入的 `program.Send` 从后台推送 `exportProgressMsg`(表计数与每 10 万行行数),状态栏显示当前表与计数;测试中可直接注入回调。`exporting` 标志让状态栏显示进行中图标,`exportedMsg` 负责清除。
- 表概览的原始行结构固定为 `schema/表名/类型/行数/大小/列数/注释`,`overviewAll` 与过滤都基于这个结构;渲染时 `typeHasSchema` 为 false 的连接(MySQL/MongoDB)去掉首列,避免重复展示数据库名。`contextTables` 下结果面板处于行选择模式(`results.SetRowMode`):左右键不移动列,`Enter`/`v` 通过 `openOverviewTable` 打开当前行表的数据预览。
- Redis 支持 `SCAN 0 COUNT 50`、`GET "key"` 等命令;Mongo 支持 `find db.collection [json]`、`findOne`、`aggregate`、`count`。二者都不支持写入(`Mutator()` 返回 `nil`)。

### 配置与持久化状态

| 用途 | 位置 |
|---|---|
| 主配置 | `$XDG_CONFIG_HOME/bdb/config.yaml`(默认 `~/.config/bdb/config.yaml`) |
| 运行时添加的连接 | `$XDG_STATE_HOME/bdb/connections.yaml` |
| 查询历史 | `$XDG_STATE_HOME/bdb/history` |
| 当前主题 | `$XDG_STATE_HOME/bdb/theme` |
| 当前布局 | `$XDG_STATE_HOME/bdb/layout` |

- `${ENV_VAR}` 展开适用于大多数连接字符串字段;变量缺失时会报错并指出连接名与字段,不会静默使用空字符串。
- 运行时连接仅在主配置中不存在同名连接时才合并;新增数据源会保存原始 `${VAR}` 引用,但当前会话使用展开后的副本连接。
- 状态文件使用 `0600` 权限和临时文件 + rename 的原子写入方式。新增持久化数据时沿用该模式。
- 校验按连接类型检查必填项(PostgreSQL/MySQL 的 host、user,SQLite 的 path,Redis 的 addr,Mongo 的 uri/host;PostgreSQL 的 database 可省略,默认连 `postgres` 库),拒绝重名;`default_connection` 字段仅为兼容旧配置保留,启动时不再自动连接。
- 新增配置字段需要同步修改 `config.Config`/相关结构体、`applyDefaults`、必要的校验、`config.example.yaml`,以及 `config.go` 中的默认配置模板。YAML 使用 `KnownFields(true)`,未知字段会直接报错。

### i18n 与主题

- 新增文案键需在 `internal/i18n/keys.go` 添加常量,并在 `internal/i18n/i18n.go` 的 **`en` 和 `zh` 两张表**中都添加条目。缺失键会静默返回键名(或回退英文),漏掉中文翻译很难被发现。
- `language: auto` 会从 `LC_ALL`/`LANG` 推断语言。
- 主题是 `internal/ui/theme` 中的全局 lipgloss 样式;`theme.Set(name)` 会修改包级变量。新增主题需要向 `palettes` 添加完整的 `Palette`(含语法高亮颜色)。运行时选择会写入状态文件,下次启动恢复;状态文件优先于 `ui.theme`。

## 约定

- 提交信息使用英文 conventional 前缀(`feat:`、`fix:`、`chore:`、`docs:`、`build:`、`style:`、`refactor:`)。
- README 中的分支策略:`main` 只接受合并;日常开发在 `dev` 分支进行。
- 测试仅使用标准库:表驱动/子测试、`t.TempDir()`、`t.Setenv()`(常用 `XDG_STATE_HOME`/`XDG_CONFIG_HOME` 隔离状态)。没有 testify、mock 生成器或自定义测试框架。
- 根模型测试通过 `tea.KeyPressMsg` 和 `internal/app/model_test.go` 中的辅助函数(`runCmd`、`sendKey`、`connect`)驱动 TUI,并断言渲染内容。新增交互测试时沿用这些辅助函数。
- SQL 驱动测试使用临时 SQLite 数据库,无需 Docker 即可运行。Redis/Mongo 测试只覆盖解析和辅助函数。
- 没有 golangci-lint 配置;声明的静态检查是 `go vet ./...`。
- 面向用户的文案应放入 i18n 表;新增快捷键或菜单项时要同时补充中英文帮助文本。用户可见功能还需同步更新 `README.md` 和 `config.example.yaml`。

## 注意事项

- 全新检出后仅执行 `go test ./...` 不够,需先 `make demo-db`。`testdata/demo.db` 已被 gitignore,而 `internal/app` 测试会复制该文件。
- `make demo-db` 会删除并通过 `sqlite3` 命令重建 `testdata/demo.db`。
- `internal/sqlscript` 的脚本切分是手写的方言感知实现:`SplitFor(script, dialect)` 在 PostgreSQL 下保留 `#` 运算符并支持 `$tag$...$tag$`;MySQL 下支持 `#` 注释和 `DELIMITER`;其他方言把 `#` 当作普通字符。它仍不处理 SQLite 触发器体中的分号,MySQL `--` 注释的空格要求也未校验。
- `make release` 不会注入版本号;goreleaser 通过 `-X main.version={{.Version}}` 注入。Makefile 构建的二进制版本显示为 `dev`。
- 根 `Model.Update` 对许多消息会提前返回;新增需要响应尺寸或焦点变化的状态时,还要同步修改 `layout()`/窗口尺寸分支。
- 异步处理器必须检查 `querySeq`/消息 id;取消和超时错误需先用 `errors.Is` 判断,再显示通用失败信息。
- 结果列宽基于展示字符串;排序和单元格编辑基于原始值,但未知数值类型会回退为字符串比较;导出会遵循当前客户端排序。
