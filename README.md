# mihoro-go

[mihoro](https://github.com/spencerwooo/mihoro) 的 Go 重构版，Mihomo 代理内核的 Linux CLI 管理客户端。

多订阅管理、内核/组件更新、systemd timer 自动刷新、一键切换。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/aceak/mihoro-go/main/install.sh | sh

# 使用镜像
curl -fsSL https://raw.githubusercontent.com/aceak/mihoro-go/main/install.sh | sh -s -- --mirror "https://ghfast.top"
```

或从 [Releases](https://github.com/aceak/mihoro-go/releases) 下载二进制。

## 快速开始

```bash
mihoro sub add                   # 添加订阅源
mihoro init                      # 一键安装（内核 + 数据 + UI + 服务）
```

首次 `init` 会询问是否启用组件自动更新，启用后 systemd timer 会在每天 02:00 刷新订阅、每周一 01:00 更新组件。

## 命令

### 订阅

```bash
mihoro sub add                   # 添加订阅
mihoro sub list                  # 列出所有（`*` 标记当前激活）
mihoro sub info [name]           # 详情
mihoro sub update <name|--all>   # 手动更新
mihoro sub use <name>            # 切换
mihoro sub remove <name>         # 删除
```

### 组件

```bash
mihoro init                      # 初始化安装
mihoro init --allow-lan          # 允许局域网访问
mihoro init --mirror "<url>"     # GitHub 镜像（写入配置后后续自动使用）
mihoro init --force              # 重做组件安装
mihoro update                    # 更新 core + geodata + ui
mihoro update --all              # 全量更新 + 重启
mihoro apply                     # 应用 overrides 到 config.yaml
```

### 服务

```bash
mihoro status                    # 综合面板
mihoro status core               # mihomo 详情
mihoro start | stop | restart
mihoro log
```

### 代理 & 其他

```bash
eval $(mihoro proxy export)      # 设置代理
eval $(mihoro proxy unset)       # 取消
mihoro upgrade                   # 更新自身
mihoro uninstall                 # 卸载
```

## 配置

`~/.config/mihoro/config.toml` + `subscriptions.toml`：

```toml
# config.toml
mihomo_binary_path = "~/.local/bin/mihomo"
mihomo_config_root = "~/.config/mihomo"
github_mirror = "https://ghfast.top"

[mihomo_config]
port = 7891
socks_port = 7892
mode = "rule"
```

```toml
# subscriptions.toml
active_subscription = "my-vps"

[[subscriptions]]
name = "my-vps"
url = "https://example.com/sub.yaml"
user_agent = "clash/mihoro-go"
auto_update = true
```

## 注意事项

**从 0.2.x 升级**：0.3.x 配置格式与 0.2.x 不兼容。升级后运行任意命令会有黄色警告提示，按提示执行 `mihoro sub add` 重新添加订阅，再 `mihoro init --force` 重装组件即可。旧配置 `~/.config/mihoro.toml` 不会自动删除，可自行处理。

## 致谢

基于 [@spencerwooo](https://github.com/spencerwooo) 的 [mihoro](https://github.com/spencerwooo/mihoro) 项目设计理念重构。

## License

MIT
