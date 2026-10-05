# vpnctl Architecture & Design Guide

## 1. Safety Principles & Boundaries

### 1.1 Data Plane vs Control Plane
- **Data Plane (成熟核心)**: Xray Core (VLESS + REALITY + TCP). 代理流量由经过大规模生产检验的成熟 Core 处理，`vpnctl` 本身绝不直接转发或拦截代理流量。
- **Control Plane (vpnctl)**: 负责安装引导、预检、凭据安全生成、Systemd 服务托管、UFW 防火墙事务、Mihomo 配置导出以及全方位 Doctor 诊断。

### 1.2 SSH 防锁死与防火墙事务
- **自动检测实际 SSH 端口**: 不硬编码 22 端口，深度解析 `/etc/ssh/sshd_config`、`/etc/ssh/sshd_config.d/*` 以及 `/proc/net/tcp` 活跃监听端口（如 62505）。
- **防火墙规则幂等与隔离**: 在操作任何防火墙规则前，先确保 detected SSH 端口已放行；绝不调用 `ufw reset`，仅管理自身标记为 `vpnctl:proxy` 的规则。
- **状态追踪 (`/etc/vpnctl/state.json`)**: 记录由 `vpnctl` 实际创建的文件与防火墙规则，在 `uninstall` 或 `rollback` 时按账本精准清理，绝不破坏用户原有的 Docker、Web 或 SSH 规则。

### 1.3 事务性部署与自动回滚
每个安装步骤都在 `rollback.Engine` 注册逆向补偿动作：
1. 校验环境与 SSH
2. 生成密码学凭据 (`crypto/rand`, X25519)
3. 校验 SHA-256 并安装二进制
4. 创建 `vpnctl-proxy` 非 root 系统用户
5. 生成配置与 systemd 单元
6. 配置 UFW
7. 启动服务并执行 **Health Check** (TCP 监听 + SNI 握手)
8. **若任一环节或健康检查失败，自动逆向全量回滚并恢复原始状态**。

### 1.4 凭据安全与隐私
- 私钥在 VPS 本地由 `crypto/ecdh.X25519()` 强随机源生成，绝不使用伪随机数。
- 配置文件权限 `0600`/`0640`，禁止全局可读。
- `vpnctl status` 永远对私钥脱敏 (`********`)，客户端导出 (`vpnctl export`) 仅输出公钥与 Short ID，私钥绝不出服务端。

---

## 2. 目录规范

| 路径 | 权限 | 用途 |
| :--- | :--- | :--- |
| `/etc/vpnctl/vpnctl.yaml` | 0600 (root:root) | 用户意图配置 |
| `/etc/vpnctl/state.json` | 0600 (root:root) | 实际资源账本状态 |
| `/etc/vpnctl/credentials.json` | 0600 (root:root) | 本地密钥凭据 |
| `/etc/vpnctl/core/config.json` | 0640 (root:vpnctl-proxy) | Core 运行时配置 |
| `/etc/vpnctl/users/users.json` | 0600 (root:root) | 多用户数据存储 |
| `/var/lib/vpnctl/versions/` | 0700 (root:root) | 版本备份与历史 |
| `/var/log/vpnctl/` | 0755 (vpnctl-proxy) | 服务访问与错误日志 |
| `/usr/local/bin/vpnctl` | 0755 (root:root) | vpnctl 主程序 |
| `/usr/local/bin/xray` | 0755 (root:root) | Proxy Core 主程序 |

---

## 3. CLI 命令清单

```bash
# 一键安装（自动事务与健康检查）
sudo vpnctl install

# 查看运行状态与连接指标
vpnctl status

# 全面体检与异常排查
sudo vpnctl doctor
vpnctl doctor --explain <RULE_ID>

# 导出客户端配置
vpnctl export mihomo [username]
vpnctl export uri    [username]
vpnctl export qr     [username]

# 用户管理
vpnctl user list
vpnctl user add <username>
vpnctl user remove <username>
vpnctl user enable <username>
vpnctl user disable <username>

# 核心热升级（带自动回滚）
sudo vpnctl update

# 查看日志与配置
vpnctl logs -f
vpnctl config show

# 安全卸载（仅清理 vpnctl 创建的规则与服务）
sudo vpnctl uninstall
sudo vpnctl uninstall --keep-config
```
