# mihoro-go

[mihoro](https://github.com/spencerwooo/mihoro) 的 Go 重构版，Mihomo 代理内核的 Linux CLI 管理客户端。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/aceak/mihoro-go/main/install.sh | sh

# 使用镜像
curl -fsSL https://raw.githubusercontent.com/aceak/mihoro-go/main/install.sh | sh -s -- --mirror "https://ghfast.top"
```

或从 [Releases](https://github.com/aceak/mihoro-go/releases) 下载二进制放到 `$PATH`。

## 使用

```bash
# 初始化
mihoro init                      # 交互式初始化（需先 mihoro sub add 添加订阅）
mihoro init --allow-lan          # 允许局域网访问
mihoro init --mirror "<url>"     # GitHub 镜像加速下载
mihoro init --force              # 强制重做组件安装

# 订阅管理
mihoro sub add                   # 交互式添加订阅
mihoro sub list                  # 列出所有订阅
mihoro sub info [name]           # 查看订阅详情
mihoro sub update <name>         # 手动更新订阅
mihoro sub update --all          # 更新所有订阅
mihoro sub use <name>            # 切换激活订阅
mihoro sub current               # 显示当前激活订阅
mihoro sub remove <name>         # 删除订阅

# 组件更新
mihoro update                    # 更新 mihomo 组件（core + geodata + ui）
mihoro update --all              # 更新全部组件并重启
mihoro update --mirror "<url>"   # 指定镜像

# 配置
mihoro apply                     # 应用 overrides 到 mihomo config.yaml 并重启

# 服务管理
mihoro status                    # 综合状态面板
mihoro status core               # mihomo 服务详情
mihoro status sub                # 当前订阅详情
mihoro start | stop | restart
mihoro log

# 代理
eval $(mihoro proxy export)      # 设置代理（localhost）
eval $(mihoro proxy export-lan)  # 设置代理（局域网 IP）
eval $(mihoro proxy unset)       # 取消代理

# 其他
mihoro upgrade                   # 更新 mihoro 自身
mihoro uninstall                 # 交互式卸载
mihoro uninstall -y              # 跳过确认
```

配置文件 `~/.config/mihoro/`：

```toml
# config.toml
mihomo_binary_path = "~/.local/bin/mihomo"
mihomo_config_root = "~/.config/mihomo"

[mihomo_config]
port = 7891
socks_port = 7892
mixed_port = 7890
mode = "rule"
log_level = "info"
external_controller = "0.0.0.0:9090"
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

## 致谢

基于 [@spencerwooo](https://github.com/spencerwooo) 的 [mihoro](https://github.com/spencerwooo/mihoro) 项目设计理念重构。

## License

MIT
