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
- 异步操作放在返回类型化消息的 `tea.Cmd` 闭包中(`connOpenedMsg`、`queryDoneMsg`、`mutationDoneMsg` 等),处理器回到根模型执行。查询带 `querySeq` 序号,过期结果会被丢弃。
- 布局是手动计算的:`layout()` 计算 `sideW`、`rightW`、`editorH`、`resultsH`;终端小于 20x6 时直接返回,小于 40x10 时渲染 "terminal too small" 提示。焦点模式有 `manual`(F2 切换缩放)和 `auto`(缩放跟随焦点,F2 暂停)。
- 会话保存在 `map[string]*session` 中。切换连接时保存当前编辑器/结果/补全器/编辑缓存,并恢复目标会话。任何新的按连接保存的 UI 状态都必须加入 `saveCurrentSession` 和 `activateSession`,否则会在连接之间串数据。
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
- Redis 支持 `SCAN 0 COUNT 50`、`GET "key"` 等命令;Mongo 支持 `find db.collection [json]`、`findOne`、`aggregate`、`count`。二者都不支持写入(`Mutator()` 返回 `nil`)。

### 配置与持久化状态

| 用途 | 位置 |
|---|---|
| 主配置 | `$XDG_CONFIG_HOME/bdb/config.yaml`(默认 `~/.config/bdb/config.yaml`) |
| 运行时添加的连接 | `$XDG_STATE_HOME/bdb/connections.yaml` |
| 查询历史 | `$XDG_STATE_HOME/bdb/history` |
| 当前主题 | `$XDG_STATE_HOME/bdb/theme` |

- `${ENV_VAR}` 展开适用于大多数连接字符串字段;变量缺失时会报错并指出连接名与字段,不会静默使用空字符串。
- 运行时连接仅在主配置中不存在同名连接时才合并;新增数据源会保存原始 `${VAR}` 引用,但当前会话使用展开后的副本连接。
- 状态文件使用 `0600` 权限和临时文件 + rename 的原子写入方式。新增持久化数据时沿用该模式。
- 校验按连接类型检查必填项(host/user/database/path/addr/uri),拒绝重名,并要求 `default_connection` 存在。
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
