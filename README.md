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
mihoro init                      # 交互式初始化
mihoro init -s "<url>" -y        # 指定订阅，非交互式
mihoro init --allow-lan          # 允许局域网访问代理
mihoro init --mirror "<url>"     # GitHub 镜像加速下载
mihoro init --force              # 强制重新下载所有组件
sudo mihoro init --system -y     # 系统级服务（需 root）

mihoro update                    # 更新 mihomo 组件（core + geodata + ui）
mihoro update --config           # 仅更新远程订阅配置
mihoro update --all              # 更新全部

mihoro apply                     # 应用 mihoro.toml 覆盖到 config.yaml 并重启

mihoro status                    # 查看服务状态（版本、运行态、开机启动）
mihoro start | stop | restart | log

eval $(mihoro proxy export)      # 设置代理环境变量（localhost）
eval $(mihoro proxy export-lan)  # 设置代理环境变量（局域网 IP）
eval $(mihoro proxy unset)       # 取消代理环境变量

mihoro cron enable | disable | status   # 自动更新定时任务

mihoro upgrade                   # 更新 mihoro 自身
mihoro upgrade --check           # 仅检查更新

mihoro uninstall                 # 交互式卸载
mihoro uninstall -y              # 跳过确认
```

配置文件 `~/.config/mihoro.toml`：

```toml
remote_config_url = "https://example.com/sub.yaml"

[mihomo_config]
port = 7891
socks_port = 7892
mixed_port = 7890
mode = "rule"
log_level = "info"
external_controller = "0.0.0.0:9090"
```

## 致谢

基于 [@spencerwooo](https://github.com/spencerwooo) 的 [mihoro](https://github.com/spencerwooo/mihoro) 项目设计理念重构。

## License

MIT
