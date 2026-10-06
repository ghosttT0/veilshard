<p align="center">
  <img src="./assets/banner1.jpg" alt="veilshard" width="780">
</p>

<h1 align="center">veilshard</h1>

<p align="center">
  一条命令在 VPS 上装好抗审查代理节点（VLESS + Reality）<br>
  自带<b>多用户订阅分发</b>与<b>流量管理面板</b> · 零第三方依赖 · 单个静态二进制
</p>

<p align="center">
  <a href="#快速开始"><img src="https://img.shields.io/badge/安装-一条命令-238636?style=flat-square" alt="Install"></a>
  <img src="https://img.shields.io/badge/Go_1.22-纯标准库-00ADD8?style=flat-square" alt="Go">
  <img src="https://img.shields.io/badge/平台-Ubuntu_22.04%2F24.04-E95420?style=flat-square" alt="Ubuntu">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="MIT"></a>
</p>

---

## 30 秒看懂

- 装完终端直接吐出一条 **Clash 订阅链接**，导入 Clash Verge / Clash for Windows / Shadowrocket 即可上网
- 面板里点几下就能**给朋友开账号**：每人独立订阅链接、独立 UUID、独立流量配额和有效期
- Clash 顶部那条流量进度条显示的是**真实用量**（xray 按用户精确计量，不是写死的假数字）
- 节点用 Reality 伪装成访问 Apple 的正常 HTTPS 流量，主动探测会被原样转发到真的 `gateway.icloud.com`
- 管理面板有 256 位密钥登录门禁、防爆破延迟、订阅访问日志（谁在什么时候用什么客户端拉了链接）

## 快速开始

> 要求：Ubuntu 22.04 / 24.04（amd64），root 权限，443 未被占用

**方式一：源码安装（服务器上有 Go 1.22+）**

```bash
git clone https://github.com/ghosttT0/veilshard.git && cd veilshard
sudo bash scripts/install.sh
```

**方式二：已有编译好的二进制**

```bash
sudo ./veilshard install
```

安装过程自动完成：SNI 探测 → 下载 xray 内核 → 生成 Reality 密钥 → 配置 UFW/systemd → 启动订阅服务。
结束时终端会打印三样东西：

```
✓ 订阅链接   http://你的IP:8080/sub/xxxx
✓ 管理面板   http://你的IP:8080/panel
✓ 管理密钥   64位十六进制（登录面板用，也存在 /etc/vpnctl/admin_token）
```

把订阅链接粘进客户端，完事。

## 给朋友开账号

**面板（推荐）**：浏览器打开 `http://IP:8080/panel`，输入管理密钥 → 填用户名、配额（GB）、有效天数 → 创建 → 点"复制"把订阅链接发给对方。

**CLI**：

```bash
veilshard user add alice --expire 30d --quota 100GB
# ✓ Added user 'alice' (UUID: f0b3d0db-..., Expires: 2026-11-05, Quota: 50.00 GB)
# Subscription URL: http://你的IP:8080/sub/34581946...
```

对方拿到链接导入 Clash 就能用。她的流量、剩余配额在你面板里实时可见；`remove` 或过期后链接立即失效。

## 功能

| | |
|---|---|
| **一键部署** | preflight 检查 → Reality 密钥生成 → UFW/systemd 配置 → 回滚保障，全程无人工编辑配置文件 |
| **VLESS + Reality** | 伪装 SNI 可探测优选（默认 `gateway.icloud.com`），证书自动漂移免维护 |
| **多用户订阅** | 每用户独立 token / UUID / 配额 / 有效期；Clash 收到完整 YAML，其他客户端收到 Base64 |
| **真实流量统计** | xray StatsService 按用户计量，订阅头 `Subscription-Userinfo` 驱动客户端流量条 |
| **管理面板** | 用户/配额/二维码/访问日志/节点状态可视化；256 位密钥 + 服务端会话门禁，未登录只见登录框 |
| **订阅访问日志** | 记录每次拉取的真实 IP（Cloudflare 隧道感知）、客户端 UA、命中/无效令牌 |
| **抗探测** | 无效令牌统一 3 秒 tarpit，探测者无法区分"链接不存在"和"链接失效" |
| **Cloudflare Tunnel 友好** | 面板与订阅走 CF 域名隐藏源站 IP，自动识别 `X-Forwarded-Proto` 下发 https 链接 |
| **零依赖** | `go.mod` 0 外部依赖，纯标准库，`CGO_ENABLED=0` 静态编译 |

## CLI 速查

| 命令 | 作用 |
|---|---|
| `veilshard install` | 一键部署节点 |
| `veilshard export serve -port 8080` | 启动订阅 + 面板服务 |
| `veilshard user add <名> [--expire 30d] [--quota 100GB]` | 添加用户 |
| `veilshard user token <名>` | 查看用户订阅链接 |
| `veilshard user traffic` | 各用户实时上下行流量 |
| `veilshard user quota <名> <200GB\|0>` | 修改配额（0 = 不限） |
| `veilshard user reset-token <名>` | 换发订阅链接（旧的立即作废） |
| `veilshard user list / enable / disable / remove / revoke` | 用户管理 |
| `veilshard export qr <用户>` | 终端 ASCII 二维码 |
| `veilshard status / doctor / logs` | 状态 / 自检 / 日志 |
| `veilshard probe / rotate / healthcheck` | SNI 探测 / 零停机轮换 / 客户端视角探针 |
| `veilshard quota -limit 800GB` | 整机流量核算与熔断（iptables，零数据库） |
| `veilshard uninstall` | 卸载并回滚全部变更 |

> 旧命令名 `vpnctl` 完全兼容（同一二进制的别名）。

## 进阶：用 Cloudflare Tunnel 藏住面板

节点流量（Reality/443）保持直连；只把面板和订阅套上 CF 域名：

1. 域名 DNS 托管到 Cloudflare，服务器安装 `cloudflared`
2. 建隧道指向 `http://localhost:8080`，绑定域名（如 `sub.example.com`）
3. 之后面板就是 `https://sub.example.com/panel`，订阅链接自动变成 https 域名形式

面板内复制按钮输出的链接会自动跟随访问方式（域名 → https，IP → http）。

## 安全模型

- 管理面板：未登录访问只返回一个登录表单，页面其余字节不下发；密钥仅经 POST 表单传输（不进 URL/日志）
- 会话为服务端内存态（HttpOnly + SameSite=Strict，24h），重启即全失效；登出立即销毁
- 管理密钥 256 位，常数时间比较，失败响应带随机延迟对抗爆破
- 凭据文件 `/etc/vpnctl/{credentials,users,state}.json` 权限 0600
- 订阅令牌 128 位 CSPRNG，无效令牌统一 tarpit，无法枚举有效用户

## 文档

- [docs/architecture.md](docs/architecture.md) — 系统架构
- [docs/README-v0.2.md](docs/README-v0.2.md) — v0.2 详版手册（品牌/架构/对比表/安全保证）
- [开发文档.md](开发文档.md) — 开发笔记

## License

MIT
