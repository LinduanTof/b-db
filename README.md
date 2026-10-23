# bdb

一款面向 Linux 终端的现代化数据库管理工具，使用 Go + [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) 构建。目标是在一个终端窗口内完成连接管理、库表浏览、SQL 编辑执行、数据编辑与导出。

已完成：PostgreSQL/MySQL/SQLite 浏览与查询、事务化数据编辑、Redis/MongoDB、多会话、表结构/DDL/索引、CSV/JSON/Markdown 导出、SQL 文件执行、查询历史、6 套主题、SQL 语法高亮与自动补全、专注布局与鼠标操作。

## 快速开始

要求 Go 1.26+。

```bash
go install github.com/LinduanTof/b-db/cmd/bdb@latest
# 或本地构建
make demo-db     # 生成 SQLite 示例库 testdata/demo.db
make build
./bin/bdb --config config.example.yaml
```

默认配置文件路径为 `$XDG_CONFIG_HOME/bdb/config.yaml`（通常是 `~/.config/bdb/config.yaml`），可用 `--config` 覆盖。

配置文件最小示例：

```yaml
default_connection: local
ui:
  language: auto          # auto | en | zh
  theme: default          # default | dracula | gruvbox | nord | catppuccin | solarized-light
  focus_mode: manual      # manual | auto
query:
  timeout: 30s
  max_rows: 1000
connections:
  - name: local
    type: sqlite
    path: ./testdata/demo.db
  - name: pg
    type: postgres
    host: 127.0.0.1
    port: 5432
    user: postgres
    password: ${PG_PASSWORD}
    database: postgres
    sslmode: disable
  - name: mysql
    type: mysql
    host: 127.0.0.1
    port: 3306
    user: bdb
    password: ${MYSQL_PASSWORD}
    database: bdb
```

`password` 等字段支持 `${ENV_VAR}` 引用；变量未设置时启动会报错并指出连接名与字段，不会静默使用空字符串。建议 `chmod 600` 配置文件。

## 快捷键

| 范围 | 按键 | 作用 |
|---|---|---|
| 全局 | `Esc` | 打开选项菜单；查询执行中为取消查询 |
| 全局 | `F2` | 切换专注布局（当前面板全屏） |
| 全局 | `q` | 退出（编辑器焦点下输入字符） |
| 全局 | `?` | 打开快捷键帮助 |
| 全局 | `Tab` / `Shift+Tab` | 切换面板 |
| 全局 | `Ctrl+C` | 退出 |
| 连接 | `↑` / `↓` | 移动光标 |
| 连接 | `Enter` | 连接、展开节点或打开表操作菜单 |
| 连接 | `→` / `←` | 展开 / 折叠 |
| 连接 | `s` | 选中表并生成 `SELECT` |
| 连接 | `d` | 查看表结构（列/索引/外键） |
| 连接 | `a` | 添加数据源（侧栏/结果焦点） |
| 连接 | `r` | 重新加载当前节点 |
| 编辑器 | `Ctrl+Enter` / `F5` | 执行 SQL |
| 编辑器 | `Ctrl+Space` | 手动触发 SQL 补全 |
| 编辑器 | `Ctrl+P` / `Ctrl+N` | 查询历史 |
| 编辑器 | `Esc` | 取消正在执行的查询 |
| 结果 | 方向键 / `hjkl` | 移动单元格 |
| 结果 | `PgUp` / `PgDn` | 翻页 |
| 结果 | `g` / `G` | 首行 / 末行 |
| 结果 | `H` / `L` | 向左 / 向右翻一屏列 |
| 结果 | `Enter` / `v` | 查看当前单元格的完整内容 |
| 结果 | `e` | 编辑当前单元格 |
| 结果 | `x` | 删除当前行 |
| 结果 | `[` / `]` | 减小 / 增大当前列宽 |
| 结果 | `o` / `O` | 按当前列排序（升/降）/ 清除排序 |
| 结果 | `c` | 复制当前单元格（OSC 52） |
| 结果 | `Ctrl+E` / `Ctrl+J` / `Ctrl+M` | 导出 CSV / JSON / Markdown |

> 普通终端通常无法区分 `Ctrl+Enter` 与 `Enter`。bdb 会请求终端的键盘增强协议（Kitty protocol）；若终端不支持，请使用 `F5` 执行。

## 选项菜单

按 `Esc` 打开 btop 风格的选项菜单（主界面保留在背景中）：

- `↑`/`↓` 移动选项，`←`/`→` 调整设置值，`Enter` 执行操作，`Esc` 关闭
- 操作：添加数据源、断开连接、导出 CSV/JSON/Markdown、帮助、退出
- 设置：主题、界面语言、查询超时、结果最大行数，修改即时生效
- 底部菜单栏常驻显示 `Esc 菜单`、`q 退出`、`? 帮助`

菜单打开时按 `q` 可直接退出。

## SQL 文件执行

启动时传入脚本会在连接默认连接后自动执行，最后一条查询的结果显示在结果面板：

```bash
./bin/bdb --config config.yaml ./migrations/001_init.sql
```

脚本按分号切分，支持单引号/双引号/反引号字符串、`--`、`#`、`/* */` 注释；任一语句失败会停止并报告语句序号。执行期间可按 `Esc` 取消。

## 添加数据源

在侧栏或结果面板按 `a`，或从选项菜单选择“添加数据源”，打开居中的大尺寸连接表单。用 `←`/`→` 选择 PostgreSQL、MySQL、SQLite、Redis 或 MongoDB，`↑`/`↓` 或 `Tab` 切换字段，`Enter` 在最后一个字段保存，`Esc` 取消。

- 密码字段以掩码显示；也可以填写 `${ENV_VAR}` 引用环境变量。
- 保存后会自动建立连接，新数据源立即出现在侧栏。
- 运行时添加的连接保存在 `$XDG_STATE_HOME/bdb/connections.yaml`（权限 `0600`），启动时与主配置合并；同名连接不会重复添加。
- 字段会根据类型动态变化：SQLite 只需要名称与路径；MySQL 与 Navicat 一样不需要填写数据库名；Redis 填写地址、密码与数据库编号；MongoDB 填写连接 URI。
- 表单内容居中展示，label 固定宽度左对齐；当前字段以 `▸` 标记并在底部显示操作提示。

## 专注布局

- 按 `F2` 让当前焦点面板占满主区域，再按一次恢复三栏
- 专注模式下 `Tab` 保持在三个全屏面板之间切换，不会退回三栏
- 侧栏同样支持全屏，方便浏览大量表和字段
- 配置 `ui.focus_mode: auto` 后，编辑器与结果面板会自动跟随焦点全屏，侧栏焦点时恢复三栏；此时 `F2` 用于暂停或恢复自动专注
- 选项菜单中也可选择 “Toggle focused layout”

## 鼠标

- 点击侧栏节点：连接、展开节点或打开表操作菜单
- 点击编辑器或结果面板：切换焦点
- 点击结果单元格：选中对应单元格
- 滚轮：在侧栏、结果网格、DDL/索引查看器中滚动

## 结果网格

- 字段较多时，`←`/`→` 移动单元格会自动横向滚动；`H`/`L` 直接向左/右翻一屏列，`Home`/`End` 跳到首列/末列
- 单元格内容超过列宽时仍会截断显示；选中后按 `Enter` 或 `v` 打开完整内容查看器
- 查看器支持 `↑`/`↓`、`PgUp`/`PgDn`、`g`/`G` 滚动，`Esc` 或 `q` 关闭
- `[`/`]` 调整当前列宽，`o`/`O` 排序，`c` 复制单元格

## 数据编辑

仅当结果通过「表操作菜单 → View data」打开、且表有主键时启用：

- `e` 编辑当前单元格：输入新值 → 预览 `UPDATE` SQL → `y` 确认执行
- `x` 删除当前行：预览 `DELETE` SQL → `y` 确认执行
- 写入在事务中执行，影响行数必须为 1
- 连接配置 `read_only: true` 时禁止编辑
- 表操作菜单的「Insert row template」会生成 `INSERT` 模板到编辑器

## Redis 与 MongoDB

Redis：

- 侧栏按 db → key 浏览（SCAN，最多 500 个键）
- 表操作菜单 View data 会按 key 类型生成 `GET` / `HGETALL` / `LRANGE` / `SMEMBERS` / `ZRANGE` / `XRANGE`
- 编辑器可直接输入 Redis 命令，如 `SCAN 0 COUNT 50`、`GET "my key"`

MongoDB：

- 侧栏按 database → collection 浏览
- 编辑器查询语法：
  - `find <db>.<collection> [json]`
  - `findOne <db>.<collection> [json]`
  - `aggregate <db>.<collection> [json array]`
  - `count <db>.<collection> [json]`
- 表操作菜单 View data 会生成 `find db.collection {}`

## 多会话

每个连接作为独立会话保留编辑器内容、结果网格、补全与编辑缓存。在侧栏切换到另一个配置连接时自动保存当前状态，切回时恢复；断开当前连接不会影响其他会话。

## 表操作菜单

在侧栏选中表后按 `Enter` 打开：

- **View data**：生成 `SELECT *` 并立即执行
- **View DDL**：查看建表语句（MySQL 使用 `SHOW CREATE TABLE`，SQLite 读取 `sqlite_master`，PostgreSQL 根据 catalog 生成）
- **View structure**：列、索引、外键详情
- **View indexes**：索引列表
- **Generate SELECT**：只生成查询填入编辑器，不执行

DDL 与索引查看器支持 `↑`/`↓`、`PgUp`/`PgDn`、`g`/`G` 滚动，`Esc` 或 `q` 关闭。

## SQL 编辑器

编辑器会按当前主题为 SQL 着色：关键字、函数、字符串、数字与注释分别使用不同颜色。支持多行编辑、行号、`Ctrl+Space` 自动补全与查询历史。

## SQL 自动补全

在编辑器中输入标识符时会自动弹出候选，也可按 `Ctrl+Space` 手动触发。候选包括 SQL 关键字、常用函数、当前连接的所有表名，以及已加载的列名；输入 `users.` 时会按需加载该表的列。`↑`/`↓` 选择，`Tab` 或 `Enter` 接受，`Esc` 关闭。

## 主题

内置 6 套主题：`default`、`dracula`、`gruvbox`、`nord`、`catppuccin`、`solarized-light`。可在配置文件中设置 `ui.theme`，或按 `Esc` 打开选项菜单后用 `←`/`→` 切换。运行时选择会写入 `$XDG_STATE_HOME/bdb/theme`，下次启动自动恢复。

## 查询历史

成功执行的语句会持久化到 `$XDG_STATE_HOME/bdb/history`（默认 `~/.local/state/bdb/history`），文件权限为 `0600`，最多保留 500 条。编辑器内用 `Ctrl+P` / `Ctrl+N` 浏览。

## 开发

分支策略：

- `main` 只接受合并，不直接提交
- 日常开发在 `dev` 分支进行

```bash
make test        # 生成 demo.db 并运行全部测试
make vet
go test ./...
```

PostgreSQL / MySQL 集成验证：

```bash
docker compose -f deploy/docker-compose.yml up -d
PGPASSWORD=bdb psql -h 127.0.0.1 -U bdb -d bdb
mysql -h 127.0.0.1 -ubdb -pbdb bdb
```

若本地端口已被占用，可用环境变量覆盖：`BDB_PG_PORT=5433 BDB_MYSQL_PORT=3307 docker compose ... up -d`。

性能基准：

```bash
go test -bench . -benchmem ./internal/ui/results ./internal/ui/editor
```

静态发布：

```bash
make release          # 生成 dist/bdb-linux-amd64、arm64 与 checksums.txt
goreleaser release --clean
```

## 路线图

P2/P3/P4 已完成：事务化数据编辑、SQL 语法高亮、Redis/MongoDB、多会话、性能基准与静态发布。

后续可扩展：SSH 隧道、TLS、Redis/MongoDB 写入、插入行表单、更多导出格式。
