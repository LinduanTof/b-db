# bdb

一款面向 Linux 终端的现代化数据库管理工具，使用 Go + [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) 构建。目标是在一个终端窗口内完成连接管理、库表浏览、SQL 编辑执行、数据编辑与导出。

当前完成 **P1**：PostgreSQL/SQLite/MySQL 浏览与查询、表结构详情、CSV/JSON/Markdown 导出、SQL 文件执行、查询历史、结果网格增强、6 套可持久化主题、SQL 自动补全，以及界面内添加数据源。

## 快速开始

要求 Go 1.26+。

```bash
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
| 全局 | `q` | 退出（编辑器焦点下输入字符） |
| 全局 | `?` | 打开快捷键帮助 |
| 全局 | `Tab` / `Shift+Tab` | 切换面板 |
| 全局 | `Ctrl+C` | 退出 |
| 连接 | `↑` / `↓` | 移动光标 |
| 连接 | `Enter` | 连接或展开节点 |
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

在侧栏或结果面板按 `a`，或从选项菜单选择“添加数据源”，打开居中的大尺寸连接表单。用 `←`/`→` 选择 PostgreSQL、MySQL 或 SQLite，`↑`/`↓` 或 `Tab` 切换字段，`Enter` 在最后一个字段保存，`Esc` 取消。

- 密码字段以掩码显示；也可以填写 `${ENV_VAR}` 引用环境变量。
- 保存后会自动建立连接，新数据源立即出现在侧栏。
- 运行时添加的连接保存在 `$XDG_STATE_HOME/bdb/connections.yaml`（权限 `0600`），启动时与主配置合并；同名连接不会重复添加。
- 字段会根据类型动态变化：SQLite 只需要名称与路径；MySQL 与 Navicat 一样不需要填写数据库名，连接后可浏览所有数据库，选中表时会生成 `db.table` 形式的查询。
- 表单内容居中展示，label 固定宽度左对齐；当前字段以 `▸` 标记并在底部显示操作提示。

## SQL 自动补全

在编辑器中输入标识符时会自动弹出候选，也可按 `Ctrl+Space` 手动触发。候选包括 SQL 关键字、常用函数、当前连接的所有表名，以及已加载的列名；输入 `users.` 时会按需加载该表的列。`↑`/`↓` 选择，`Tab` 或 `Enter` 接受，`Esc` 关闭。

## 主题

内置 6 套主题：`default`、`dracula`、`gruvbox`、`nord`、`catppuccin`、`solarized-light`。可在配置文件中设置 `ui.theme`，或按 `Esc` 打开选项菜单后用 `←`/`→` 切换。运行时选择会写入 `$XDG_STATE_HOME/bdb/theme`，下次启动自动恢复。

## 查询历史

成功执行的语句会持久化到 `$XDG_STATE_HOME/bdb/history`（默认 `~/.local/state/bdb/history`），文件权限为 `0600`，最多保留 500 条。编辑器内用 `Ctrl+P` / `Ctrl+N` 浏览。

## 开发

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

## 路线图

- **P2**：数据编辑（主键定位、事务、SQL 预览与确认）、语法高亮编辑器。
- **P3**：Redis、MongoDB、多会话。
- **P4**：性能基准、静态二进制发布。

已完成：主题系统、SQL 自动补全。
