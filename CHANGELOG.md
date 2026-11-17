# Changelog

## [0.3.4]

### 更新

- `init` / `update` / `upgrade` 新增 `--proxy`（http/https/socks5/socks5h），`config.toml` 新增 `proxy` 字段；`init --proxy` 会自动持久化
- 组件下载优先走镜像，镜像失败自动回退 GitHub 直连（直连同样走配置的代理）
- `update` 改为两阶段：所有组件先下载并校验，全部成功后才替换文件；任一失败不改动现有安装
- 服务在全部替换完成后只重启一次；仅更新 UI 时不重启
- core 已是最新版本时跳过下载；`--arch` 会强制重装
- geodata 使用 GitHub API 中 release asset 的 SHA-256 与本地文件比对，未变化时跳过下载
- UI 使用 GitHub release tag 比对（metacubexd / zashboard），未变化时跳过下载；yacd-meta 与 custom UI 仍每次下载
- 版本检查直连 GitHub API（可走代理，不经过镜像）；检查失败时警告并回退为正常下载
- 下载内容校验（core gzip、UI 归档、geodata 文件），镜像返回 HTML 错误页时自动回退直连
- 镜像 URL 拒绝空白字符，避免写入 systemd unit 时被拆成多个参数
- 修复传入共享 HTTP client 时下载超时设置被忽略的问题
- 修复镜像地址被写进 `geox_url` 时无法识别为默认 geodata 源的问题
- 修复进度条在重试/回退时可能因进度超过总量而崩溃的问题

## [0.3.3]

### 安全

- 默认生成随机 API 密码（secret）写入 mihomo 配置，面板/API 需认证；密码仅在生成时显示一次
- `init` 中途失败时仍会显示已生成的密码，避免被锁在面板外
- 手动输入的密码含空格时明确报错，不再静默裁剪首尾空白
- 面板/API 默认监听 `127.0.0.1:9090`（原 `0.0.0.0:9090`），`init --dashboard-lan` 开放到局域网
- 新增 `mihoro secret reset`（别名 `rotate`）重置密码，`--disable` 关闭认证
- 新增 `init --dashboard-lan` / `init --no-secret` 标志
- `config.toml`、`config.yaml` 权限收紧为 `0600`
- 修复管道多行输入时交互提示被吞的问题（init 与 `sub add` 均覆盖）

### 修复

- `--mirror` 现在会在 `update` / `upgrade` 时读取配置文件中保存的镜像（此前只有命令行显式传参才生效）
- 镜像参数统一校验与规范化：引号、结尾斜杠自动去除，缺 `http(s)://` 前缀时给出明确报错；`install.sh` 同步处理
- 订阅 URL、代理地址输入做相同规范化：引号自动去除，缺 `http(s)://` 前缀时明确报错并重新询问

### 其他

- `status` 面板显示密码状态（仅状态，不显示值）
- 已有配置不受影响，不自动迁移

## [0.3.2]

- `sub update` 移除文件存在跳过和 `--force`，手动更新每次都下载


## [0.3.1]

### 升级检测

- 自动检测 0.2.2 旧配置并提示升级步骤，旧配置不删除
- init 后写 `.version` 供未来版本判断兼容性

### 多订阅管理

- `mihoro sub` 命令组：`add` / `list` / `info` / `remove` / `update` / `use` / `current`
- 每订阅独立 User-Agent、Proxy、Headers，支持多订阅并存切换
- `sub add` 交互式添加，URL 立即下载验证，失败提示代理/Header 重试
- `sub use <name>` 一键切换，自动下载→override→写入 config.yaml→重启 mihomo
- `sub update --all` 更新活跃订阅时自动 apply 并重启

### 命令变更

- `init` 不再创建订阅，提示使用 `sub add`；已有订阅时显示激活订阅状态
- `update` 移除 `--config`，订阅更新走 `sub update`；`--all` 只含 core+geodata+UI
- `status` 显示综合面板 + timer 下次触发时间；`status core` / `status sub` 子命令
- `uninstall` 新流程：timer→配置→二进制→mihomo，每步可选
- 删除 `cron`、`setup` 命令，移除 `--system` / `--yes` / `--ua` / `--subscribe` 标志
- 移除用户级 systemd 支持，统一系统级

### systemd timer 替代 crontab

- 订阅刷新：每天 02:00
- 组件更新：每周一 01:00
- timer service 自动带 `--mirror` 参数

### 配置

- `~/.config/mihoro/config.toml`（组件）+ `subscriptions.toml`（订阅）
- 移除 `remote_config_url` / `mihoro_user_agent` / `auto_update_interval` / `user_systemd_root`
- 新增 `github_mirror`
- 组件配置（allow-lan/mirror/auto-update）init 末尾一次写入

### 其他

- 下载器统一 `Download(ctx, DownloadOptions)`，支持代理、Headers、超时、原子写入
- 全局 Ctrl+C 退出
- Port/SocksPort 改为 `*uint16` 防零值覆写
- `--allow-lan`、`--mirror`、`--system`、`--yes`、`--ua`、`--subscribe` 标志移除


## [0.2.2]

- `upgrade --mirror` 支持
- upgrade 版本已最新时跳过更新
- upgrade 超时提示


## [0.2.1]

- `--allow-lan` 标志
- `--mirror` GitHub 镜像支持
- 统一信号处理
- `uninstall` 重写
- 支持系统级 systemd


## [0.1.1] 内部开发版本，未公开发布。
初始版本，[mihoro](https://github.com/spencerwooo/mihoro) Rust 项目的 Go 语言重构版本。

- `init` 一键初始化（内核 + geodata + UI + systemd 服务）
- `update` 组件更新（core/geodata/UI/config）
- `apply` TOML overrides
- `start` / `stop` / `restart` / `status` / `log` systemd 管理
- `proxy export` / `export-lan` / `unset`
- `cron enable` / `disable` / `status` crontab 自动更新
- `upgrade` 自更新
- `uninstall` 卸载
- `completion` shell 补全
