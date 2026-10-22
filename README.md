# bdb

一款面向 Linux 终端的现代化数据库管理工具，使用 Go + [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) 构建。目标是在一个终端窗口内完成连接管理、库表浏览、SQL 编辑执行、数据编辑与导出。

当前完成 **P1**：在 P0（PostgreSQL/SQLite 完整闭环）基础上，新增 MySQL/MariaDB、表结构详情（列/索引/外键）、Markdown 导出、SQL 文件执行、查询历史持久化，以及结果网格的列宽调整、排序与单元格复制。

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
| 全局 | `Esc` | 打开命令面板（弹窗）；查询执行中为取消查询 |
| 全局 | `q` | 退出（编辑器焦点下输入字符） |
| 全局 | `?` | 打开快捷键与命令帮助 |
| 全局 | `:` | 打开命令面板（侧栏/结果焦点） |
| 全局 | `Tab` / `Shift+Tab` | 切换面板 |
| 全局 | `Ctrl+C` | 退出 |
| 连接 | `↑` / `↓` | 移动光标 |
| 连接 | `Enter` | 连接或展开节点 |
| 连接 | `→` / `←` | 展开 / 折叠 |
| 连接 | `s` | 选中表并生成 `SELECT` |
| 连接 | `d` | 查看表结构（列/索引/外键） |
| 连接 | `r` | 重新加载当前节点 |
| 编辑器 | `Ctrl+Enter` / `F5` | 执行 SQL |
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

## 命令面板

按 `Esc` 打开弹窗式命令面板（主界面保留在背景中），`:` 也可用。输入时实时过滤，`↑`/`↓` 选择，`Tab` 补全命令名，`Enter` 执行，`Esc` 关闭。底部菜单栏常驻显示 `Esc 菜单`、`q 退出`、`? 帮助`，与 btop 的操作方式一致。`?` 帮助中同时列出快捷键与命令。

| 命令 | 作用 |
|---|---|
| `:connect <name>` | 连接到配置中的连接 |
| `:disconnect` | 断开当前连接并清空结果 |
| `:describe` | 查看侧栏中选中表的结构 |
| `:export csv\|json\|md` | 导出当前结果 |
| `:focus sidebar\|editor\|results` | 切换焦点 |
| `:lang en\|zh` | 切换界面语言 |
| `:run <path>` | 执行 SQL 文件 |
| `:help` | 显示快捷键与命令帮助 |
| `:reload` | 重新连接当前连接 |
| `:quit` | 退出 bdb |

命令名支持部分输入（如 `:exp` 后按 `Enter` 会先补全）；带参数的命令需要完整写出，如 `:lang zh`。

## SQL 文件执行

启动时传入脚本会在连接默认连接后自动执行，最后一条查询的结果显示在结果面板：

```bash
./bin/bdb --config config.yaml ./migrations/001_init.sql
```

也可以在应用内执行：

```
:run ./migrations/001_init.sql
```

脚本按分号切分，支持单引号/双引号/反引号字符串、`--`、`#`、`/* */` 注释；任一语句失败会停止并报告语句序号。执行期间可按 `Esc` 取消。

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
- **P4**：主题、性能基准、静态二进制发布。
