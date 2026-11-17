# mihoro-go

[mihoro](https://github.com/spencerwooo/mihoro) 的 Go 重构版，Mihomo 代理内核的 Linux CLI 管理客户端。

多订阅管理、内核/组件更新、systemd timer 自动刷新、一键切换。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/aceak/mihoro-go/main/install.sh | sh

# 使用镜像
curl -fsSL https://raw.githubusercontent.com/aceak/mihoro-go/main/install.sh | sh -s -- --mirror https://ghfast.top
```

或从 [Releases](https://github.com/aceak/mihoro-go/releases) 下载二进制。

## 快速开始

```bash
mihoro sub add                   # 添加订阅源
mihoro init                      # 一键安装（内核 + 数据 + UI + 服务）
```

首次 `init` 会生成面板/API 密码（**只显示一次**，请及时保存），并询问是否启用组件自动更新；启用后 systemd timer 会在每天 02:00 刷新订阅、每周一 01:00 更新组件。面板默认仅本机（`127.0.0.1`）可访问。

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
mihoro init --allow-lan          # 允许局域网访问代理端口
mihoro init --dashboard-lan      # 面板/API 开放到局域网（默认仅本机 127.0.0.1）
mihoro init --no-secret          # 不生成 API 密码（默认自动生成）
mihoro init --mirror <url>       # GitHub 镜像（写入配置后 update/upgrade 自动使用）
mihoro init --proxy <url>        # 下载代理（写入配置后 update/upgrade 自动使用）
mihoro init --force              # 重做组件安装
mihoro update                    # 检查并更新 core + geodata + ui（无新版本则跳过）
mihoro update --all              # 全量检查并更新，全部成功后统一重启
mihoro update --proxy http://127.0.0.1:7890  # 本次经本机 mihomo 代理下载
mihoro apply                     # 应用 overrides 到 config.yaml
```

### 服务

```bash
mihoro status                    # 综合面板
mihoro status core               # mihomo 详情
mihoro start | stop | restart
mihoro log
```

### 安全

```bash
mihoro secret reset              # 重置面板/API 密码（回车自动生成强密码，仅显示一次）
mihoro secret reset --disable    # 删除密码，关闭认证
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
proxy = "http://127.0.0.1:7890"   # 可选：组件下载/自身升级走的代理

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

**镜像地址格式**：必须带 `http://` 或 `https://` 前缀（如 `https://ghfast.top`）；结尾斜杠和首尾引号会自动去除，命令行加不加引号均可。写入配置后，`update` / `upgrade` / systemd 自动更新都会使用该镜像。镜像下载失败时会自动回退到 GitHub 直连。

**代理**：`--proxy` 支持 `http://`、`https://`、`socks5://`、`socks5h://`（省略协议时按 `http://` 处理）。`init --proxy` 会保存到 `config.toml` 的 `proxy` 字段，后续 `update` / `upgrade` 自动使用；也可以直接编辑配置。未显式配置时，`HTTPS_PROXY` / `HTTP_PROXY` 环境变量仍然生效（Go 标准行为）。

**组件更新机制**：

- **core**：先读取 `version.txt` 与本地 `mihomo -v` 比较，版本相同直接跳过；`--arch` 会强制重装。
- **geodata**：通过 GitHub API 获取 release asset 的 SHA-256，与本地 `country.mmdb` / `geoip.dat` / `geosite.dat` 比对，相同则跳过下载。仅对默认的 `MetaCubeX/meta-rules-dat` 源生效。
- **ui**：通过 GitHub release tag 比对（metacubexd、zashboard）；`yacd-meta` 与 `custom:<url>` 没有可比较的 tag，仍然每次下载。
- 版本检查直接访问 `api.github.com`（不经过镜像，可走代理）；检查失败时打印警告并回退为正常下载，不阻断更新。设置 `GITHUB_TOKEN` 可提高 GitHub API 的限流额度。
- 所有组件先下载并校验（core 还会执行 `-v` 自检），**全部成功后才替换文件**；任一组件下载失败则不做任何改动。替换完成后统一重启一次，仅更新 UI 时不重启。

## 安全

- 面板/API（`external-controller`）默认仅监听 `127.0.0.1:9090`，局域网无法直接访问；需要局域网使用时执行 `mihoro init --force --dashboard-lan`，或修改 `config.toml` 的 `mihomo_config.external_controller` 后 `mihoro apply`
- 首次 `init` 自动生成随机 API 密码（也可手动输入自定义密码），**只在初始化结束时显示一次**，忘记后用 `mihoro secret reset` 重置
- 含密码的 `config.toml`、`config.yaml` 权限为 `0600`
- 已有配置不会自动改动：升级后如需启用密码或收紧监听，执行 `mihoro secret reset`、`mihoro init --force`
- 注意 `sudo` 下 `HOME` 默认为 `/root`，配置位于 `/root/.config/mihoro`；重置密码请在相同环境下执行

## 注意事项

**从 0.2.x 升级**：0.3.x 配置格式与 0.2.x 不兼容。升级后运行任意命令会有黄色警告提示，按提示执行 `mihoro sub add` 重新添加订阅，再 `mihoro init --force` 重装组件即可。旧配置 `~/.config/mihoro.toml` 不会自动删除，可自行处理。

## 致谢

基于 [@spencerwooo](https://github.com/spencerwooo) 的 [mihoro](https://github.com/spencerwooo/mihoro) 项目设计理念重构。

## License

MIT
