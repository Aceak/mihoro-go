# Changelog

## [0.3.1]

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
- `--allow-lan`、`--mirror`、`--system`、`--yes`、`--ua`、`--subscribe` 标志移除
- Port/SocksPort 改为 `*uint16` 防零值覆写

---

## [0.2.2]

- `upgrade --mirror` 支持
- upgrade 版本已最新时跳过更新
- upgrade 超时提示

---

## [0.2.1]

- `--allow-lan` 标志
- `--mirror` GitHub 镜像支持
- 统一信号处理
- `uninstall` 重写
- 支持系统级 systemd

---

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
