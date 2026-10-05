# Veilshard (隐纱之核) 宣传片分镜脚本 · v3 镜头卡编排版

> **片名**：Veilshard — 让节点群自己照顾自己
> **时长**：112 秒（1'52"，落在 1–2min 区间）
> **画幅**：16:9 · 1920×1080 · 30fps（可另出 9:16 竖版）
> **投放场景**：GitHub README 置顶 / B 站 / X / Linux.do
> **目标观众**：自建 VPS、抗审查、自托管社群（有 Linux 基础，被一键脚本坑过的人）
> **v3 修订**：按 video-shotcraft 镜头卡库完成选卡——全片编排 **23 张镜头卡**（含点名 4 张），每张卡标注落点与适配命门；实现阶段必须读卡全文 + 对应 demo 源码后再动手。

---

## 〇、制作前提

Veilshard 是纯 CLI 项目，本片不出现任何网页截图。视觉语言三层，全部代码确定性生成（Remotion）：

1. **终端即 UI**：命令与输出按品牌视觉重绘，逐字符打字机动画；终端内容逐字取自 README 真实命令与输出，不虚构功能。
2. **讲解演示优先**：每个概念翻译成一个"能看懂的实验"——跟随数据包主角、审查员放大镜视角、焦油坑/水位计/倒带等物理比喻。
3. **品牌隐喻贯穿**：Veil（纱幕）、Shard（水晶碎片）；水晶青 `#00F2FE` + 紫红渐变 `#7F00FF→#E100FF` 全片复用。

### 一、镜头卡映射总表（23 张）

★ = 用户点名必用。实现时逐卡读 `references/shots/<类别>/<卡名>.md` 全文，并按"参考实现"定位 demo 源码；卡上"已知坑/命门"参数不得降档。

| # | 镜头卡 | 类别 | 落点 | 用法与适配要点 |
|---|---|---|---|---|
| 1 | ★ bottom-push-stack-wipe | transition | 换章骨架 →S05→S06→S08→S09 | 四次整屏推章，方向统一从底边；章底色用品牌饱和深档（青 #0E7490 / 紫 #6B21A8 / 靛 #312E81 / 青灰 #155E63），终端窗口卡钉死在章内坐标系；推入 30f 重 ease-out，上缘 40px 接缝阴影保留 |
| 2 | ★ aurora-bloom-bg-flip | effects | S03 痛点→品牌的转折重音 | blob 三层光色**整组**换成品牌紫红（主 #7F00FF / 核 #E100FF / 白融边），浅底 #ECECEC 压暗到 #0A0E14；反转压在 0.36s 命门内；A 句 blur-out 与 B 段之间留 0.6s 无字空档 |
| 3 | ★ bezier-source-converge-merge | ui-entrance | S03 多源汇聚成徽标 | 四条贝塞尔来源 = bash 脚本 / x-ui / 手动 iptables / 明文订阅（灰色占位节点，不上色），沿曲线加速吸入中心 Veilshard 水晶徽标；数据包强调色保留；曲线反向擦除只留徽标；`getPointAtLength` 需在挂载后计算（已知坑） |
| 4 | ★ chip-lift-to-user-pill | interaction | S05 从候选域名中选中并展开 | 5 个大厂域名 chip 网格，probe 命中目标 chip **3 帧台阶化反色**（不给缓动），其余按曼哈顿距离交错淡出；chip 左缘锚定生长成药丸 `www.microsoft.com`，绿点在生长收尾才亮，连接线接到节点水晶徽标 |
| 5 | typing-code-block | typography | S01 错误行 / S04 YAML / S11 安装命令 | 全片终端打字的基础词汇：逐字符打字、字符保持 token 高亮色、当前字符垫方块光标；demo 左右并置两式只取右式 |
| 6 | terminal-3d | camera | S02① 三台机器人肉 SSH | 三个终端窗散布 3D 空间，相机窗间飞行、每到一窗敲一段 iptables——"三步命令三站旅程"换成"三台机器三站疲命" |
| 7 | fracture | opening | S01 末节点碎裂 & S07 引爆呼应 | 取卡的后半（碎片背离中心加速旋转飞出）作硬转场；S01 被动碎裂、S07 主动引爆，首尾同卡不同机位与速度，构成母题呼应 |
| 8 | montage-rhythm-moves | rhythm | S02 节奏骨架 & S06→S07 接缝 | S02 用 B 式 wright-triple-cut 三连咔哒特写管碎切呼吸；S07 入场用 A 式 drop-blackout-slam 黑场蓄爆 1.5s——push 骨架刻意断开，让高潮吃能量落差 |
| 9 | quad-split-parallel-scenes | rhythm | S02 末 2s 四痛点四宫格 | 四象限并行跑各自崩坏微场景，关键节拍错开 3–6 帧，信息轰炸压到乐句尾重拍 |
| 10 | crash-zoom-punch | camera | S02④ 急推进度条 / S07 急推终端输出 | 6–11f 一拍急推；强调级用撞停震屏；前后 hold 足帧建立全景与特写 |
| 11 | letterspace-materialize | opening | S03 VEILSHARD 字标结晶 | 大字距全大写、全字符并行连续描画、同帧齐收；静谧仪式感，一次呼吸完成，终态静置 ≥30f |
| 12 | glow-flyline-moves | effects | S04 拓扑连线 + 数据包飞行 | C 式 orb-flyline-relay：光斑（节点水晶）与飞线（数据流）同帧共振；暗场数据叙事，Linear 味；数据包主角即沿 flyline 滑行的 orb |
| 13 | cycle-glass-node-morph | data | S04 落地机机制图（REALITY 三段机制） | 胶囊缩入美西节点玻璃体 → 三段循环标签沿弧线建立（伪装握手 / 量子填充 / 合规回落）→ 三枚玻璃节点托起接管 → 诊断标记 ✓ 钉住系统状态；8.6s 长镜压缩适配到 5s，保住"标签沿弧依次建立"主段 |
| 14 | scan-bracket-sweep | effects | S05 证书合规校验 | 骨架域名弹入，四角 L 形取景括号落下，扫描线往复扫 5 趟逐项点亮 `TLS 1.3 / H2 / 证书链`；文档静止、只有光在读它 |
| 15 | odometer-digit-roll | data | S07 `90s` 全屏巨号（兼 S05 42ms 小用） | 每个数位老虎机式独立滚带带残影，逐位过冲停稳，全部锁定瞬间整体加深脉冲；王牌指标全屏亮相，hold ≥45f |
| 16 | card-flip-reveal | transition | S06 密钥棱片翻面 | 三枚 ShortID 水晶棱片沿 Y 轴翻 180°，侧棱最薄处闪高光带，背面揭出新字符，逐张错峰 10f；数据粒子流全程不断（零停机语义） |
| 17 | dashboard-glow-highlight-pill | effects | S06 healthcheck 表格高亮 | 适配：青色光斑巡游至 `TCP_RST/BLOCKED` 行拉成胶囊高亮（红），再起笔描出诊断标签辉光轮廓；表格 3D 微漂移保留 |
| 18 | speed-ramp-freeze | rhythm | S07 炸裂 0.2x 凝视 | 碎裂瞬间 remap 到 0.2x 慢速窗 ≥40f，定格圈注裂纹起点 `GFW RST`，再解冻加速拉回 |
| 19 | scramble | typography | S08 明文→密文 / 锚点密钥锁定 | **倒用**：从真字高速跳乱码（加密方向），`#AES_KEY_████████` 段再正向逐字锁定；种子驱动可复现（确定性渲染红线） |
| 20 | gauge-readout-moves | data | S09 水位计配额熔断 | B 式 tape-scroll-fixed-pointer：针不动、刻度带滚过，冲刺刹车停在 `90%`（告警）→ 再冲一格 `98%`（刀闸拉下）；单指标大跳变的机械仪式感 |
| 21 | impact-feedback | effects | S09 刀闸落闸 & S10 图章砸落 | 元素级命中反馈：图章用顿帧+微震屏，刀闸用负片帧+集中线一闪；寄生在落位动作上，不单独成镜 |
| 22 | timeline-travel | data | S10 事务性回滚倒带 | **倒放适配**：部署步骤刻度轴 `✓binary → ✓config → ✗healthcheck`，失败后镜头沿轴**倒向**掠回，已完成的步骤卡片逐张倒扣消失，`rollback ✓ · system restored` 收尾 |
| 23 | logo-shrink-wordmark-lockup | outro | S11 品牌定妆 | 满屏能量收束到"图标 + 字标 + 标语"标准 lockup：图形快速收束带过冲刹车 → 图标左移让位 → 字母逐个滑入 → `github.com/ghosttT0/veilshard` 强调色收尾 |

> 每张卡的"已知坑"（如 bezier 的 `getPointAtLength` 挂载时机、chip-lift 的反色不给缓动、aurora 的 0.36s 反转命门、bottom-push 的方向统一）在实现阶段照抄 demo 源码的调校参数，只做素材与配色的蒙皮替换。

### 二、视觉隐喻词汇表（讲解演示的"演员表"，全片统一造型）

| 角色/比喻 | 造型 | 出现镜头 | 含义 |
|---|---|---|---|
| **数据包（主角）** | 发光青色水晶胶囊，拖彗星尾迹（glow-flyline 的 orb） | S03 S04 S11 | 用户的流量 |
| **审查员（DPI/墙）** | 灰色独眼监视塔 + 放大镜，扫描时射红色扇形光束 | S01 S02 S04 S07 | 审查/探测方 |
| **节点** | 立方水晶体（Shard），点亮时内部棱面反光 | 全片 | 一台 VPS |
| **纱幕（Veil）** | 半透明白色波动绸缎 | S03 S04 S11 | 混淆层 |
| **密钥碎片** | 三枚小水晶棱片刻 ShortID 字符 | S06 | 凭据 |
| **焦油坑** | 画面底部黑色粘稠液面，冒泡，粘住伸入物 | S09 | Tarpit 陷阱 |
| **水位计 + 刀闸** | 竖直玻璃管 + 电工刀闸 | S09 | 流量配额/熔断 |
| **倒带刻度轴** | 水平步骤刻度轴 + 胶片齿孔 | S10 | 事务性回滚 |
| **双车道** | 上下两条发光车道（IPv4/IPv6），中间 50ms 转换灯 | S04 S06 | Happy Eyeballs |

### 三、设计 Tokens（从 assets/logo.svg、banner.svg 提取）

| Token | 值 | 用途 |
|---|---|---|
| 背景深 | `#0A0E14` → `#151821` | 全片底色，细微网格/噪点 |
| 主强调 | `#00F2FE` 水晶青 | 命令高亮、成功状态、数据包、关键数据 |
| 能量渐变 | `#7F00FF → #E100FF` | 加密流、REALITY 包裹、极光 blob、高潮段 |
| 章节底色 | `#0E7490` / `#6B21A8` / `#312E81` / `#155E63` | bottom-push 四章饱和深档 |
| 危险 | `#FF4757` / 琥珀 `#FFB020` | 审查员光束、BLOCKED、痛点段 |
| 正文字 | `#FFFFFF` / `#8B93A7` | 标题白、注释灰 |
| 字体 | JetBrains Mono（代码/数据）+ 思源黑体/HarmonyOS Sans（中文文案） | |

### 四、音乐与节奏

- BGM：100–118 BPM 暗黑电子；四拍乐句分镜，转场卡拍；交付**带 BGM + 无 BGM** 两版。
- 能量曲线：`黑场压抑(S01) → 焦躁碎切(S02) → 极光转折·凝聚揭示(S03) → 好奇讲解(S04) → 推章推进(S05–06) → 黑场蓄爆→高潮爆发(S07) → 幽灵骤静(S08) → 阴冷反杀(S09) → 干脆图章(S10) → 落定留白(S11)`。
- 换章语法分工：**bottom-push**（S05/S06/S08/S09 章节骨架）× **blackout-slam**（S07 高潮入口）× 黑场淡出（S11 收尾）——三种接缝各司其职，不混用。
- 时间码为目标时长，正式制作前做 BGM 节拍分析后吸附网格。

---

## 二、分镜脚本

### S01 · 冷开场 Hook —— 「又没了」
**0:00.0 – 0:08.0（8.0s）** · 能量：★☆☆☆☆ · 卡：`typing-code-block` + `fracture`

| 层 | 内容 |
|---|---|
| 画面 | 全黑。光标闪 3 次。`typing-code-block` 逐字符打出红字：`curl: (28) Connection timed out`；停 1s，灰字 `ssh: connect to host 198.51.100.2 port 22: Connection refused`。<br>审查员监视塔升起，红光扫过——一颗节点水晶按 `fracture` **后半段**碎裂，碎片背离中心加速旋转飞出画面 |
| 运镜 | 固定机位；碎裂瞬间轻微 2.5D 推近 |
| 字幕 | 左下角：`凌晨 02:14` |
| VO | 「凌晨两点，你的节点，又没了。」 |
| SFX | 光标滴 → 打字 → 玻璃碎裂 + 低频 sub-drop + 光束电流嗡鸣 |
| 卡点 | 碎裂落第 1 强拍 |

### S02 · 痛点蒙太奇 —— 老办法的混沌
**0:08.0 – 0:19.0（11.0s）** · 能量：★★★☆☆ · 卡：`terminal-3d` + `montage-rhythm-moves`(B式) + `crash-zoom-punch` + `quad-split-parallel-scenes`

| 层 | 内容 |
|---|---|
| ①（3s） | `terminal-3d`：三个终端窗散布 3D 空间，相机逐窗飞行，每窗敲一段 `iptables -t nat -A PREROUTING...`，越敲越乱 |
| ②（2s） | 碎切对（wright-triple-cut 三连咔哒）：SNI 盲猜问号乱飞 → 证书日历 `expires in 3 days` 红闪节点熄灭 |
| ③（2s） | 云控制台 `Rebuilding… 20:00`，`crash-zoom-punch` 一拍急推进度条（撞停震屏） |
| ④（2s） | 订阅 URL 泄露，审查员红色触手扑向 IP 池，节点逐个变灰 |
| ⑤（2s） | `quad-split-parallel-scenes`：2×2 四宫格同时崩坏（断连/超时/被墙/泄露），节拍错开 3–6 帧，压到乐句尾 |
| 运镜 | 手持感抖动（克制）；四宫格段切固定 |
| 字幕 | 右下角红标签依次：`手动拼接` `SNI 盲猜` `证书漂移` `20min 重来` `订阅裸奔` |
| VO | 「登三台机器、手写 iptables、盲猜伪装域名——重来一次二十分钟；订阅一泄露，IP 池一夜裸奔。」 |
| SFX | 窗间飞行 whoosh、三连咔哒、急推撞停、虫鸣、四宫格同步电流噪声 |
| 卡点 | 每段切换落拍；四宫格四线错拍冲击压在乐句尾重拍 |

### S03 · 品牌揭示 —— 极光转折，多源归一
**0:19.0 – 0:27.0（8.0s）** · 能量：★★☆☆☆ · 卡：★`aurora-bloom-bg-flip` + ★`bezier-source-converge-merge` + `letterspace-materialize`

| 层 | 内容 |
|---|---|
| A 拍（0:19–0:23） | 全片唯一浅底段：`aurora-bloom-bg-flip`——紫红渐变 blob（品牌光色整组替换）从底部缓缓升起，字 A「旧的打法，到此为止。」逐词 blur-out；**0.36s 命门反转**压暗到 #0A0E14，blob 压成余晖 |
| B 拍（0:23–0:25.5） | `bezier-source-converge-merge`：暗场上四条曲线错峰 draw-on（来源节点 = bash 脚本 / x-ui / 手动 iptables / 明文订阅，中性灰），数据包沿路径滑行，四个旧来源沿曲线三段加速**吸入**中心，反向擦除通路——只剩 Veilshard 水晶徽标，接收脉冲 +12% outBack |
| C 拍（0:25.5–0:27） | `letterspace-materialize`：VEILSHARD 大字距字标全字符并行描画结晶；`隐纱之核` 副标 + tagline `极简 · 高隐匿 · 抗审查 · 零依赖` 静态浮现。**hold ≥1.5s 呼吸** |
| VO | 「旧的打法，到此为止。Veilshard，隐纱之核。」 |
| SFX | 极光升起的空气白噪 → 压暗 sub-drop → 曲线接通音×4 → 吸入 impact → 字标结晶的水晶 shimmer |
| 卡点 | 压暗反转、吸入、字标齐收三个高点各占一拍 |

### S04 · ★讲解核心 —— 一份 YAML，一枚数据包的旅行
**0:27.0 – 0:44.0（17.0s）** · 能量：★★★☆☆ · 卡：`typing-code-block` + `glow-flyline-moves`(C式) + `cycle-glass-node-morph`

| 层 | 内容 |
|---|---|
| A 段（0:27–0:31） | `typing-code-block` 打出 `veilshard apply -f veilshard.yaml`，YAML 关键行（`role: relay` / `role: exit` / `sni_target: "auto"`）token 色高亮飞入；右侧 `glow-flyline` 飞线连线，两颗节点水晶随行点亮，拓扑骨架成型 |
| B 段（0:31–0:44） | 跟随数据包长镜头：<br>① **双车道**（2s）：IPv4 车道被审查员红光射断，`50ms` 转换灯闪，胶囊无感滑入 IPv6 车道<br>② **中继**（1.5s）：香港水晶节点 speed-line 直穿不停留<br>③ **落地机制**（5s）：`cycle-glass-node-morph`——胶囊缩入美西节点玻璃体，三段循环标签沿弧线依次建立：`伪装握手 → 量子填充 → 合规回落`，三枚玻璃节点托起接管，`✓` 诊断标记钉住<br>④ **偷天换日**（2.5s）：镜头拉开，审查员放大镜对准流量，圆形 mask 特写内：`www.apple.com` 真证书、真 TLS 1.3、Chrome 指纹——独眼眨了眨，放大镜放下（放行）<br>⑤ 胶囊射入 `自由互联网` 地平线（2s） |
| 运镜 | B 段横移长镜跟随胶囊；放大镜视图圆形 mask |
| 字幕 | A 段：`一份 YAML，声明整个网格`；放大镜时小字：`审查员看到的：一次普通的 HTTPS 访问` |
| VO | 「一份 YAML，声明中继与落地。跟着一枚数据包过墙：双栈竞速，五十毫秒切换活路；香港中继透传，美西落地套上 REALITY——审查员眼里，你只是在访问 apple.com：证书是真的，握手是真的，什么也查不出来。」 |
| SFX | 打字 → 变道 whoosh → 中继风声 → 玻璃节点托起的机械音 → 审查段抽空只留低频 → 放行轻叹 → 音乐涌起 |
| 卡点 | 变道/透传/标签建立/放行落拍；放大镜段刻意反节奏留白 |

### S05 · `veilshard probe` —— 从候选里选中"这一个"
**0:44.0 – 0:54.0（10.0s）** · 能量：★★★☆☆ · 卡：★`bottom-push-stack-wipe`(章1·青) + `scan-bracket-sweep` + ★`chip-lift-to-user-pill` + `odometer-digit-roll`(小用)

| 层 | 内容 |
|---|---|
| 推章（1s） | `bottom-push` 整屏从底边推入：青色章 #0E7490，终端窗口卡钉死章内 |
| 校验（2s） | `scan-bracket-sweep`：候选域名骨架弹入，L 形括号落下，扫描线往复扫 5 趟，逐项点亮 `TLS 1.3 / ALPN H2 / 证书链合规` |
| 选中（3.5s） | `chip-lift-to-user-pill`：五个大厂域名 chip 排开，目标 chip **3 帧硬切反色**选中，其余按曼哈顿距离交错淡出；chip 左缘锚定生长成药丸 `www.microsoft.com`，`odometer` 小号滚出 `42ms`，绿点收尾点亮，1px 连接线接到节点水晶徽标 |
| 彩蛋（2s） | 证书日历快进 `14 days` → 漂移箭头滑向新目标 → `hot-reloaded`，粒子不断 |
| hold（1.5s） | 静置 |
| 字幕 | 标题：`自动探测 · 择优伪装 · 临期自愈` |
| VO | 「伪装目标不用猜——本地并发实测，在证书合规的大厂里选中最快的一个；临期，还会自己漂移。」 |
| SFX | 推章哐 → 扫描线机械呼吸 → 反色硬切咔 → 生长+打字 → 绿点叮 → 漂移滑音 |
| 卡点 | 推章落拍；反色、绿点、漂移三高点落拍 |

### S06 · `rotate` + `healthcheck` —— 换血不停车 · 两个视角对视
**0:54.0 – 1:04.0（10.0s）** · 能量：★★★★☆ · 卡：★`bottom-push-stack-wipe`(章2·紫) + `card-flip-reveal` + `dashboard-glow-highlight-pill`

| 层 | 内容 |
|---|---|
| 推章（1s） | `bottom-push` 推入紫色章 #6B21A8 |
| A 段（3s） | `card-flip-reveal`：一条数据粒子束横穿画面，三枚 ShortID 水晶棱片沿 Y 轴翻 180°（侧棱高光带），背面揭出新字符，逐张错峰 10f——**粒子流一毫秒不断**。终端角标：`veilshard rotate · grace period ✓` |
| B 段（6s） | 中缝裂开双视角：<br>**左（VPS 自测）**：绿勾列 `✓ port listening ✓ service active ✓ TLS OK`——"在里面看永远全绿"<br>**右（客户端视角）**：`veilshard healthcheck` 表格 3D 升入微漂移，`dashboard-glow-highlight-pill` 光斑巡游至 `jp-node TCP_RST/BLOCKED` 行拉成红色胶囊高亮，`de-node HIGH_LATENCY/QOS` 琥珀、`us-west-01 ALIVE` 青；BLOCKED 行触发 0.5s 双车道闪回：客户端滑向 IPv6 |
| 字幕 | A：`零停机热轮换`；B：`VPS 自测永远全绿，用户视角才见真章` |
| VO | 「指纹密钥热轮换，旧连接不断。可自测永远全绿怎么办？探针从真实客户端视角发起——封锁、限速，提前知道。」 |
| SFX | 推章哐 → 棱片翻转机械咔哒（流声垫底）→ 裂缝低频 sweep → 光斑巡游的 shimmer → 行盖章 |
| 卡点 | 翻面、裂缝、高亮胶囊、三行落拍 |

### S07 · `veilshard recycle` —— 被墙？90 秒 IP 换血（高潮）
**1:04.0 – 1:16.0（12.0s）** · 能量：★★★★★ · 卡：`montage-rhythm-moves`(A式) + `fracture` + `speed-ramp-freeze` + `odometer-digit-roll` + `crash-zoom-punch`

| 层 | 内容 |
|---|---|
| 蓄爆（1.5s） | `drop-blackout-slam`：黑场，VO 悬半句「节点被墙？」——蓄力 |
| 引爆（2.5s） | 红光扫中节点，裂纹蔓延 → 主动引爆，`fracture` 后半碎裂飞散（呼应 S01，这次是我们按的按钮）；`speed-ramp-freeze` 碎裂瞬间 0.2x 凝视 ≥40f，定格圈注 `GFW RST`，解冻急速拉回 |
| 换血（4.5s） | 世界地图亮起：法兰克福熄灭 → 东京区青色锁定 → 新水晶降落成型，SSH 注入/加固/配置回写三道光线依次射入；`odometer-digit-roll` 全屏巨号 `20:00 → 0:90` 逐位疾转锁定 **90s**，整体脉冲 |
| 收尾（3.5s） | `crash-zoom-punch` 急推终端：`✓ destroyed → ✓ new IP 45.32.x.x → ✓ hardened → ✓ fleet updated`；客户端图标全程绿灯 |
| 字幕 | 大字：`90 秒，IP 换血` |
| VO | 「节点被墙？这次不用你动手——销毁、重建、换区、回写，九十秒换血完成，用户无感。」 |
| SFX | 黑场静默 → 引爆大 impact（全片 ≤3 处大 slam 之一）→ 慢动作抽气 → 里程表滚带急刹 → 定格脉冲 slam → 终端 ui 音 |
| 卡点 | 引爆、锁定、脉冲、急推全钉重拍 |

### S08 · `veilshard publish` —— 零知识订阅分发
**1:16.0 – 1:26.0（10.0s）** · 能量：★★☆☆☆ · 卡：★`bottom-push-stack-wipe`(章3·靛) + `scramble`

| 层 | 内容 |
|---|---|
| 推章（1s） | `bottom-push` 推入深靛章 #312E81，随后画面能量骤降 |
| 加密（4s） | `scramble` **倒用**：`clash.yaml` 明文整行字符高速跳乱码——加密方向的视觉直译；密文化作光束飘向右侧 Worker 云 |
| 全盲（3s） | Worker 云朵**蒙着眼**捧密文；红色爬虫触手试探性捏了捏密文，一无所获缩回；`#` 锚点从 URL 末端发光分离，留在用户本地，`#AES_KEY_████████` 段正向逐字锁定 |
| 静置（2s） | 近乎静止，dolly-in 缓推 |
| 字幕 | `零知识分发 · 服务端全盲`；小字：`AES-256-GCM · 密钥永不上传 · 爬虫只能拿到随机密文` |
| VO | 「订阅分发，零知识：配置端到端加密，密钥只活在链接的 # 号后面。爬虫摸到的，只是一串随机密文。」 |
| SFX | 推章哐（轻）→ 跳字沙沙 → 幽深 pad → 爬虫黏腻音 → 锚点"叮"落弱拍 |
| 卡点 | 刻意弱节奏，只有锚点锁定落一记轻拍 |

### S09 · ★防御演示 —— 焦油坑与水位计
**1:26.0 – 1:36.0（10.0s）** · 能量：★★★☆☆ · 卡：★`bottom-push-stack-wipe`(章4·青灰) + `gauge-readout-moves`(B式) + `impact-feedback`

| 层 | 内容 |
|---|---|
| 推章（1s） | `bottom-push` 推入青灰章 #155E63 |
| A 焦油坑（4s） | 底部黑色粘稠液面冒泡；审查员红色扫描触手一只只伸入——被**粘住**、挣扎、下沉（`impact-feedback` 顿帧闷击）；监视塔独眼句柄耗尽越眨越慢，给节点打上灰标 `unreachable`，掉头离开。终端角标：`sudo veilshard defense ✓` |
| B 水位计（5s） | `gauge-readout-moves` B 式滚带定针：玻璃水位计针不动、刻度带滚过，冲刺刹车停在 **90%**——琥珀告警灯亮，Telegram/Bark 弹通知，节点标 `draining` 引流；再冲一格 **98%**——电工**刀闸咔哒拉下**（负片帧+集中线一闪），代理入站红色断开，水位停。小字：`内核计数器 · 无数据库` |
| 字幕 | A：`扫描器？请君入瓮`；B：`天价账单，闸在 98%` |
| VO | 「主动探测的扫描器，全请进焦油坑——耗干句柄，标记死节点。流量涨到九成，先告警引流；九成八，直接拉闸。」 |
| SFX | 黏稠气泡、下沉闷响、告警提示 ×2、刀闸重咔哒（中 slam） |
| 卡点 | 下沉、告警、拉闸落拍 |

### S10 · 工程信任 —— 图章 + 回滚倒带
**1:36.0 – 1:42.0（6.0s）** · 能量：★★★☆☆ · 卡：`impact-feedback` + `timeline-travel`(倒放)

| 层 | 内容 |
|---|---|
| 图章（2.4s） | 两枚大图章 45° 侧斜砸落（`impact-feedback` 顿帧+微震屏）：① `单个静态二进制`（小字：`Go 1.22 纯标准库 · 0 第三方依赖`）② `SSH 永不锁定`（小字：`自动探测真实端口 · 绝不 ufw reset`） |
| 倒带（2.4s） | `timeline-travel` **倒放适配**：部署刻度轴 `✓ binary → ✓ config → ✗ health check failed`，失败瞬间镜头沿轴**倒向**掠回，已完成的步骤卡片逐张倒扣消失，画面四角浮现胶片齿孔——终端打出 `rollback ✓ · system restored` |
| hold（1.2s） | 静置 |
| VO | 「单个静态二进制，零依赖；失败自动回滚——绝不碰你的 SSH。」 |
| SFX | 两声厚重 stamp + 磁带绞盘倒带音 |
| 卡点 | 每图章一拍；倒带起止各一拍 |

### S11 · CTA —— 一条命令，GitHub 见
**1:42.0 – 1:52.0（10.0s）** · 能量：★☆☆☆☆ · 卡：`typing-code-block` + `logo-shrink-wordmark-lockup`

| 层 | 内容 |
|---|---|
| 打字（3.5s） | 黑场。`typing-code-block` 逐字打出青色安装命令（与 S01 红字冷暖呼应）：<br>`curl -fsSL https://raw.githubusercontent.com/veilshard/veilshard/main/scripts/install.sh | sudo bash` |
| 定妆（3.5s） | `logo-shrink-wordmark-lockup`：满屏图形能量收束（过冲刹车）→ 水晶图标左移让位 → VEILSHARD 字母逐个滑入 lockup → `github.com/ghosttT0/veilshard` 与 `MIT License · v0.2.0` 强调色收尾 |
| 谢幕（3s） | S04 的数据包胶囊最后一次划过画面，穿过一道纱幕消失在地平线。**hold ≥2s 完全静止**。角标：`Star ⭐ 如果它帮你省下了凌晨两点` |
| VO | 「一条命令，装好一切。MIT 开源——GitHub 搜索 Veilshard，让你的节点群，自己照顾自己。」 |
| SFX | 打字渐弱 → 收束刹车 → 字母滑入轻响 → 音乐自然收尾 |
| 卡点 | 命令打完落最后一拍，之后交给留白 |

---

## 三、旁白全文（录音稿，约 62s）

> 凌晨两点，你的节点，又没了。
>
> 登三台机器、手写 iptables、盲猜伪装域名——重来一次二十分钟；订阅一泄露，IP 池一夜裸奔。
>
> 旧的打法，到此为止。Veilshard，隐纱之核。
>
> 一份 YAML，声明中继与落地。跟着一枚数据包过墙：双栈竞速，五十毫秒切换活路；香港中继透传，美西落地套上 REALITY——审查员眼里，你只是在访问 apple.com：证书是真的，握手是真的，什么也查不出来。
>
> 伪装目标不用猜——本地并发实测，在证书合规的大厂里选中最快的一个；临期，还会自己漂移。
>
> 指纹密钥热轮换，旧连接不断。可自测永远全绿怎么办？探针从真实客户端视角发起——封锁、限速，提前知道。
>
> 节点被墙？这次不用你动手——销毁、重建、换区、回写，九十秒换血完成，用户无感。
>
> 订阅分发，零知识：配置端到端加密，密钥只活在链接的 # 号后面。爬虫摸到的，只是一串随机密文。
>
> 主动探测的扫描器，全请进焦油坑——耗干句柄，标记死节点。流量涨到九成，先告警引流；九成八，直接拉闸。
>
> 单个静态二进制，零依赖；失败自动回滚——绝不碰你的 SSH。
>
> 一条命令，装好一切。MIT 开源——GitHub 搜索 Veilshard，让你的节点群，自己照顾自己。

---

## 四、制作路线与红线

| 路线 | 说明 | 适配度 |
|---|---|---|
| **A. Remotion 镜头卡管线（推荐）** | 按 video-shotcraft「自主自由创作」管线：23 张卡逐张读全文 + demo 源码，copy `assets/lib/` 组件进工程，按本片 tokens 蒙皮；BGM 选定后先节拍分析再吸附时间线 | ★★★★★ 卡库词汇与分镜一一对应 |
| B. AE 手 K | 23 张卡的粒子/翻面/里程表效果手 K 成本极高 | ★★☆☆☆ |
| C. 真实录屏 + 包装 | 仅个别终端窗格用真实输出文本重绘 | 补充手段 |

**真实性红线**：所有命令、输出、表格与 README（v0.2.0）逐字一致；`90s 换血`、`AES-256-GCM`、`50ms`、`90%/98%`、`<15 天漂移` 等数字不得夸大。发布前对照 `internal/` 各包复核功能口径。

**确定性渲染红线**：scramble、碎片、粒子一律固定种子伪随机（mulberry32，seed 从 index 派生），禁 `Math.random()`/`Date.now()`。

**交付物清单**：1920×1080 成片（带 BGM 版 + 无 BGM 版）、封面帧（S03 定格或 S07 脉冲帧）、9:16 竖版可选、README 置顶 GIF（S04 数据包旅行 10s 循环版）。
