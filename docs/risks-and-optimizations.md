# 风险与优化建议

> 基于对 50 个包、约 5.1 万行非测试代码的通读与实测。每条结论标注**证据等级**：
> - **【已复现】**：在本机跑出了错误输出/基准数据；
> - **【代码确证】**：逐行追踪确认，未构造运行时复现；
> - **【静态推断】**：依赖架构/平台知识，需真机或 profiler 确认。
>
> 严重度：**S1** 崩溃/数据竞争/资源泄漏 · **S2** 正确性 · **S3** 性能 · **S4** 可维护性。

---

## 0. 结论摘要

### 最该先做的 9 件事

| # | 事项 | 类型 | 工作量 | 收益 |
|---|---|---|---|---|
| 1 | 修 `layoutKey` 漏 `DisableSpaceTrim` | S2 正确性 | 0.5 天 | 消除编辑器/标签共享 Shaper 时的光标与选区偏移（**已复现**） |
| 2 | 修语义树 `valid` 被覆盖 | S2 无障碍 | 0.5 天 | hover-only 区域的 label 恢复可见 |
| 3 | 修 GPU 全屏清除导致 layer 区间越界 | S1 崩溃 | 1 天 | 消除一个可达 panic |
| 4 | 文本行偏移增量计算 + `textView` 去掉 viewport 失效 | S3 性能 | 2–3 天 | 长文档输入延迟从 O(N) 降到 O(段落) |
| 5 | 路径缓存 key 去掉 `Ops.version` | S3 性能 | 3–5 天 | 圆角/描边 clip 只 tessellate 一次 |
| 6 | 输入命中索引（预计算逆变换 + 边界） | S3 性能 | 1 周 | 重叠区域从 O(n²)/110µs 降到微秒级 |
| 7 | `driver` 生命周期收口（锁 driver、可取消 Run、复位 timer） | S1 稳定性 | 2–3 天 | 关闭/重开窗口不再挂起或竞态 |
| 8 | 修 `Affine2D.Shear` 的 Y 平移项 | S2 正确性 | 0.5 天 | 公开 API 的错切变换不再偏移（**已复现**） |
| 9 | `ops.Reset` 清 `multipOp`；未完成 Record 报错 | S1 可用性 | 1 天 | 一次 `clip.Path` 未 `End` 不会永久毒化 `Ops`（**已复现**） |

### 总体判断

- **内核设计优秀**：op 流 + 双消费者 + 零分配热路径，是长期打磨的架构，**不应重写**。
- **问题集中在"边界"**：平台后端（8 份复制粘贴）、模块边界（app→material、gpu→layout）、缓存契约（key 漏字段/含易变字段）、错误边界（panic）。
- **主要性能债在 GPU 提交粒度与文本动态路径**，而不是布局或绘制原语。
- **可维护性债在超大文件与重复实现**，可用"提取 + 组合"渐进偿还，不需要大爆炸重构。

---

## 1. 架构风险

### A1. `app` 是上帝包，且反向依赖 `widget/material`（S4，高）

**证据**

- `app` 15,105 行，承担窗口、事件循环、IME、剪贴板、装饰、语义、8 个平台后端。
- `app/window.go` 直接 import `widget` 与 `widget/material`（[app/window.go:29-30](../app/window.go#L29-L30)），`init()` 无条件构造 `material.Theme` + `text.Shaper` + `material.Decorations`（[app/window.go:761-793](../app/window.go#L761-L793)），即使应用使用 `CustomRenderer` 或平台自带装饰。
- 依赖闭包：`app` 173 个包，其中 `widget` 136 个；`gioui.org/shader`、`font/gofont`、`materialdesign/icons` 全部被拖入。

**影响**

1. 任何窗口程序都携带整套 Material 主题、图标、Go 字体与文本整形器 → 二进制体积与启动开销。
2. 分层倒置：最底层的窗口运行时依赖最高层的控件库，导致 `app` 无法在没有 `widget` 的场景（如自定义渲染器、嵌入式）中瘦身。
3. 默认装饰的高度测量发生在 `init`，与用户 Theme 无关，可能测出与真实装饰不一致的高度。

**建议**

- 把窗口装饰抽成 `app/internal/deco`（只依赖 `op`、`layout`、`unit`），或定义 `DecorationRenderer` 接口，由用户/上层注入；默认实现放在独立包，`app` 不 import `material`。
- 把 `material.Theme` 的创建延迟到"确实需要 fallback 装饰"时；`DefaultDecorated=false` 时不创建。
- 中期把 `app` 拆为 `app`（公共 API）+ `internal/window`（事件循环）+ `internal/platform/{wayland,x11,...}`。

---

### A2. `internal/ops` 是隐式公共 API，无 schema、无版本（S2/S4，高）

**证据**

- `ops.EncodedOp` 被 `gpu` 与 `io/input` 直接消费（[internal/ops/reader.go:20-24](../internal/ops/reader.go#L20-L24)）。
- 两者都靠类型断言读 `Refs`：`encOp.Refs[0].(event.Tag)`（[io/input/router.go:831](../io/input/router.go#L831)）、`refs[0].(*image.RGBA)`（[gpu/gpu.go:209](../gpu/gpu.go#L209)）。断言失败即 panic。
- op 类型到长度的映射是手写表（`internal/ops/ops.go` 的 `props`，[L399-L434](../internal/ops/ops.go#L399-L434)），新增 op 必须同步改三处（常量、长度、props）。
- 没有版本号或魔数，op 流不可跨版本复用/落盘。

**影响**

- 任何内核格式变更都会同时破坏 GPU 与输入两条链路，且编译器不会提示。
- 第三方无法安全地实现自定义 op 消费者（例如录制回放、UI 测试快照）。
- 调试只能靠字节偏移，缺少"反汇编"工具。

**建议**

1. 为 `EncodedOp` 提供**受检访问器**（`func (e EncodedOp) Tag() (event.Tag, error)`），把 panic 收敛到一处。
2. 在 `Ops` 头部写 magic + version，`Reader` 校验；为格式写一份 `docs/op-format.md`。
3. 用 `go:generate` 从一张声明表生成常量、长度、props、`gpu`/`input` 的分发骨架，消除三处手工同步。
4. 提供 `ops.Dump(w io.Writer, o *op.Ops)` 调试工具（类似字节码反汇编）。

---

### A3. panic-first 错误模型（S1/S2，高）

**证据**

- `gpu.frame` 末尾恒 `return nil`（[gpu/gpu.go:443](../gpu/gpu.go#L443)），`GPU.Frame` 的 error 是摆设；真正失败走 panic：图集超限（[gpu/gpu.go:804](../gpu/gpu.go#L804)）、纹理上传（[gpu/gpu.go:483](../gpu/gpu.go#L483)）、Vulkan descriptor 池（`vulkan.go:761`）、GL 缓冲溢出（`opengl.go:1057`）。
- op 栈错配 panic（`internal/ops/ops.go`）、未匹配的 `TypePopClip` panic（[gpu/gpu.go:1062](../gpu/gpu.go#L1062)）、未知事件类型 panic（[io/input/router.go:488-490](../io/input/router.go#L488-L490)）、零尺寸帧 panic（[app/window.go:628-630](../app/window.go#L628-L630)）。
- `app` 的 `ErrDeviceLost` 恢复逻辑（[app/window.go:190-210](../app/window.go#L190-L210)）因此只有 `Refresh/Lock` 可能触发，渲染期丢失设备无法恢复。

**影响**

- 任何 GPU 资源压力、驱动怪癖、恶意/错误输入都会让整个应用崩溃，而不是降级或重试。
- 无法在库内做"跳过这一帧"。

**建议**

- 区分**编程错误**（栈错配、无效 op）与**运行时错误**（OOM、设备丢失、尺寸超限）：前者保留 panic，后者返回 error。
- `gpu.Frame` 返回 `error`；把图集溢出映射为 `ErrSkipFrame`（裁剪到视口/降级），把设备错误映射为 `gpu.ErrDeviceLost`。
- 在 `app.validateAndProcess` 的 `w.frame` 外层加 `defer recover` 兜底（仅 release 构建关闭），把未知 panic 转为 `DestroyEvent.Err`。

---

### A4. 平台后端：一个接口、三套线程契约、八份复制粘贴（S1/S4，高）

**证据**

- `driver` 15 个方法混合了客户端 goroutine 入口（`Event/Invalidate/Run/Frame`）与窗口线程专用方法（`Configure/Perform/SetCursor/ShowTextInput/...`），只有一句注释提示（[app/os.go:180-210](../app/os.go#L180-L210)）。
- `Frame` 在 Wayland/X11/JS 上是同步渲染，在 Android/Windows/macOS/iOS/OHOS 上是通道会合 + ack（[app/os.go:260-263](../app/os.go#L260-L263)）；`Run` 在 Wayland/X11/JS 上是直接调用。
- 8 个后端各自实现 WindowMode 映射（合计约 360 行）、键位映射（约 600 行）、光标映射（约 180 行）、IME 状态机（OHOS 另起一套）。
- 新增 OHOS 需要修改 15 个已有文件的 build tag（`&& !gio_ohos && !openharmony`），且两个 tag 名混用。

**影响**

- 后端作者必须从同伴代码推断线程契约，易写出竞态。
- 同一功能在 8 处行为不一致（如 `Configure` 对 Size 的处理、IME 的坐标基准）。
- 新平台接入成本高，回归面大。

**建议**

1. **拆接口**：`type driver interface { EventSource; FrameSink }` + `type windowThread interface { Configure; Perform; ... }`，并在接口注释中写明 goroutine 归属。
2. **提取平台无关层**：`internal/platform` 提供 `ConfigPlanner`（WindowMode→动作列表）、`KeyMap`（键码→`key.Name`）、`CursorMap`、`IMEController`、`ClipboardQueue`。
3. **收敛 build tag**：一个 `app/platform_ohos.go` 负责注册，其它文件用 `//go:build !ohos` 的单一 tag 名。
4. 为每个后端建立最小冒烟测试（创建窗口→投递一次输入→收到一帧→关闭），可在 CI 用 headless/Xvfb 跑。

---

### A5. 输入 Router 缺少线程归属（S1 数据竞争，高，需 `-race` 确认）

**证据**

- `Router` 的 `handlers map`、`changes`、`pointer.queue` 全部无锁（[io/input/router.go:24-60](../io/input/router.go#L24-L60)）。
- Windows 后端在专用 OS 线程的消息循环中调用 `ProcessEvent` → `Window.processEvent` → `queue.Queue`（[app/os_windows.go:104-138](../app/os_windows.go#L104-L138)、[app/window.go:695-728](../app/window.go#L695-L728)）；OHOS 在 `loopRun` goroutine 中做同样的事（[app/os_ohos.go:125-136](../app/os_ohos.go#L125-L136)）。
- 客户端 goroutine 在布局期间调用 `Source.Event`/`Source.Queue`（[io/input/router.go:211-216](../io/input/router.go#L211-L216)）。两者之间只有帧握手（`frames`/`frameAck`，[app/os.go:260-263](../app/os.go#L260-L263)），不覆盖 Router 内部状态。

**影响**

- 在 Windows/OHOS/macOS 上，用户拖动/输入与布局并发时存在真实数据竞争；map 并发读写会直接 fatal（`concurrent map read and map write`）。
- 由于时序窗口小，常规测试难以发现，线上表现为偶发崩溃。

**建议**

- 方案 A（推荐）：平台线程只把事件写入一个**无锁/加锁的待处理队列**，由持有 Router 的客户端 goroutine 在 `Event()`/`Frame()` 边界统一 `Queue`。
- 方案 B（最小改动）：给 `Router` 加 `sync.Mutex`，`Queue`/`Frame`/`Event` 全部串行化；注意不要在持锁时回调用户代码。
- 无论哪种，补 `go test -race ./io/input ./app` 与"并发 Invalidate + 关闭"用例。

---

### A6. 键盘事件没有目标 tag，"先轮询者得"（S2，中高）

**证据**

- 键盘事件入队时不带 tag：`taggedEvent{event: e}`（[io/input/router.go:450-455](../io/input/router.go#L450-L455)）。
- 匹配使用全局并集 `q.key.filter`（[io/input/router.go:300-309](../io/input/router.go#L300-L309)），哪个控件先调用 `Event` 且过滤器匹配，谁就拿到。
- 未消费的事件在 `Frame` 被丢弃（注释明说 "Remaining events are discarded"，[io/input/router.go:338-340](../io/input/router.go#L338-L340)）。

**影响**

- 两个过滤器相同的控件（例如两个都监听 `key.Filter{Name: "K"}`）行为依赖布局顺序，且无法诊断"快捷键没生效"。
- 焦点控件与快捷键控件竞争时，结果不确定。
- 事件被静默丢弃，没有未处理计数/告警。

**建议**

- 入队时记录目标：焦点 tag 或 z-order 最高的匹配 tag；按 tag 维护 key filter，而不是全局并集。
- 提供 `Router.Unhandled()` 统计与 debug 日志。
- 明确优先级：焦点 > 显式 `key.Filter.Focus` > 全局快捷键。

---

### A7. GPU 后端注册表与类型断言（S4，中）

**证据**

- 包级可变函数变量 + `init()` 注册（[gpu/internal/driver/api.go:86-89](../gpu/internal/driver/api.go#L86-L89)、[gpu/gpu.go:33-38](../gpu/gpu.go#L33-L38)），`NewDevice` 是类型 switch。
- 后端通过类型断言认领对象：`desc.VertexShader.(*glshader)`、`pipe.(*Pipeline)`。
- `driver.Device` 有 8 个 compute 相关方法从未被调用；`PrepareTexture` 在 GL/Metal/D3D11 是空实现、在 Vulkan 是真 barrier。

**影响**

- 无法注入 mock 后端；测试被迫编译全部后端。
- 传错对象直接 panic；新后端必须实现无用方法。
- 调用方无法知道 `PrepareTexture` 的代价。

**建议**

- `NewDevice(api, opts...)` 显式注册；保留包级注册作为兼容层。
- 把 `Driver` 的 capability 与方法语义写成文档；删除 compute 方法（或标注 experimental）。
- `PrepareTexture` 改名为 `EnsureTextureLayout` 并在接口注释说明"可能插入 barrier"。

---

### A8. 缓存契约靠约定，容易静默出错（S2/S3，中高）

**证据与实例**

| 缓存 | 问题 | 后果 |
|---|---|---|
| `layoutCache` | key 漏 `DisableSpaceTrim`（[text/lru.go:152-164](../text/lru.go#L152-L164) vs [text/shaper.go:441-454](../text/shaper.go#L441-L454)） | **已复现**：行尾空格宽度串味，光标/选区偏移 |
| `opCache` | key 含 `ops.Key.version`（每帧 `Reset` 自增），跨帧必失效（[gpu/gpu.go:162-167](../gpu/gpu.go#L162-L167)） | 圆角/描边 clip 每帧重新 tessellate + 新建 GPU buffer |
| `opCache.put` | 覆盖旧值时不解引用旧 `pathData`（[gpu/caches.go:124-126](../gpu/caches.go#L124-L126)） | 依赖调用方"永不覆盖有主条目"的隐含约定 |
| `pathCache`/`bitmapShapeCache` | key 只含 `GlyphID` 与相对 X，不含 `Offset`/`Bounds`（[text/lru.go:98-141](../text/lru.go#L98-L141)、[text/gotext.go:579-696](../text/gotext.go#L579-L696)） | 组合字符/不同 mark 位置可能命中错误几何 |
| 所有 LRU | 以"条目数"计（1000），不按字节 | 大字形/长文档内存不可控 |

**建议**

- 为每个缓存写**不变量注释**与**参数覆盖测试**：对 `Parameters` 的每个字段做两两组合，断言输出不同。
- `opCache` key 改为"内容哈希（路径字节 + 变换 + 描边宽度）"，version 只用于宏身份失效。
- LRU 增加字节预算与命中率指标（`expvar` 或 debug 页面）。

---

### A9. 生命周期与取消语义不完整（S1/S2，中高）

**证据**

- `w.driver` 写加 `invMu`、读裸访问（[app/window.go:666-669](../app/window.go#L666-L669) vs [app/window.go:308](../app/window.go#L308)、336、748、898）→ 数据竞争 + TOCTOU。
- `Window.Run` 发送到 `driverFuncs` 后等 `done`；若 `DestroyEvent` 先被处理，driver 循环停止，`done` 永不关闭（[app/window.go:335-346](../app/window.go#L335-L346)）。
- Destroy 后 `timer.quit` 未清空；再次 `init` → `scheduleInvalidate` 永久阻塞（[app/window.go:367-394](../app/window.go#L367-L394)）。
- OHOS `loopRun` 无退出条件、`osMain` 是 `select {}`、全局队列无界；destroy 后迟到回调会解引用 nil driver（[app/os_ohos.go:162-166](../app/os_ohos.go#L162-L166)）。
- `Invalidate` 受 `mayInvalidate` 门控，帧在途时的失效会被丢弃（[app/window.go:293-300](../app/window.go#L293-L300)）。

**建议**

- `driver` 用原子指针/单一锁访问；加 `windowDone chan struct{}`，`Run`/`Perform`/`Option` 都 `select` 它。
- Destroy 时：停 timer 并置 nil、停止 backend loop、drain 队列、将迟到事件丢弃。
- 删除 `mayInvalidate`，让后端自己合并（它们已经会合并）。
- 为"创建→关闭→再创建"补测试。

---

### A10. fork 增量引入的耦合与债（S4，中）

**证据**

- OHOS 1,693 行，其中约 20–23% 是 Android/IME 代码的再表达；`ohosState` 是进程单例；ABI 版本 2 被假定但未协商（`GIO_OHOS_ABI_VERSION`/`gio_ohos_host_ready` 未被 Go 使用）。
- 工作区有未提交的 OHOS 调试代码：每个 touch/axis 事件 `fmt.Sprintf` + `Fprintf(stderr)`（[app/ohos_pump.go:370](../app/ohos_pump.go#L370)、444）。
- build tag 侵入 15 个文件，`gio_ohos` 与 `openharmony` 混用。
- 注释引用 `docs/44`、`docs/45`，但仓库此前没有 `docs/` 目录（本次分析新建）。

**建议**

- 提交前删除逐事件日志，或统一走 `internal/debug` 开关。
- 用 `ohosSink`（已有）作为唯一平台边界，把 ABI 导出拆到 `ohos_abi.go`，IME 拆到 `ohos_ime_bridge.go`。
- 在 `gio_ohos_install_host` 时查询 ABI 版本，低于 2 时明确报错或降级。
- 把 `docs/44`、`docs/45` 的引用改为本仓库文档链接。

---

### A11. 其他架构级观察（S3/S4，中低）

| 观察 | 证据 | 影响 | 建议 |
|---|---|---|---|
| 共享代码出现 `runtime.GOOS` 分支 | [app/window.go:220-227](../app/window.go#L220-L227)、707 | 新平台需改共享文件 | 下沉到平台能力（`Caps.TransparentBackground`、`Caps.MobileFocusKeys`） |
| `gpu` 依赖 `layout` | [gpu/gpu.go:28](../gpu/gpu.go#L28)，仅用 `layout.FPt` | 几何层与布局层耦合 | 改用 `f32.FRect` 或本地 helper |
| `text` 依赖 `op/paint` 并写 op | [text/gotext.go:654-698](../text/gotext.go#L654-L698) | 整形器承担绘制职责 | 返回位图/回调，由 widget 写 op |
| 全局字体缓存写 CWD | [text/gotext.go:249-257](../text/gotext.go#L249-L257) | `UserCacheDir` 失败时污染工作目录 | 失败时禁用缓存 |
| 无 `context.Context` | 全项目 | 无法统一取消窗口/GPU 初始化 | 仅对 I/O 边界引入，不必全库 |

---

### A12. op 生命周期未硬化：一次失误永久毒化，或静默丢帧（S1/S2，高）

这一组是**框架自身的不变量没有被强制**，四个问题相互关联：

| 子项 | 证据 | 后果 |
|---|---|---|
| 未 `Stop` 的 `Record` 静默吞掉后续 op | reader 把未完成宏视作"包含剩余全部 op"（[internal/ops/reader.go:150-160](../internal/ops/reader.go#L150-L160)） | 一帧剩余 UI 全部不绘制，无任何报错 |
| `Reset` 不清 `multipOp` | [internal/ops/ops.go:172-187](../internal/ops/ops.go#L172-L187) 未重置该字段；`Write` 在它为 true 时 panic（[internal/ops/ops.go:189-193](../internal/ops/ops.go#L189-L193)） | 一次未 `End` 的 `clip.Path` 会让该 `Ops` **永久** panic（**已复现**，见 verification §4.6） |
| 陈旧 `CallOp`/`PathSpec` 无代次校验 | 文档声称 Reset 后 `Add` 会 panic（[op/op.go:173-176](../op/op.go#L173-L176)），但 `AddCall` 直接写 PC（[internal/ops/ops.go:238-246](../internal/ops/ops.go#L238-L246)） | Reset 后复用旧 CallOp 会产生零长度 `EncodedOp`，消费者取 `Data[0]` 崩溃 |
| push/pop 不平衡只在 pop 检查 | 栈清空发生在 `Reset`，帧末无断言（[internal/ops/ops.go:305-314](../internal/ops/ops.go#L305-L314)） | 漏掉 `Pop` 只影响后续控件外观，难以定位 |

**建议**

1. `Reset` 中加 `o.multipOp = false`，并在 `clip.Path.End` 缺失时给出可读错误（"unfinished clip.Path"）。
2. `MacroOp`/`CallOp`/`PathSpec` 记录 `version` 或 generation，`Add` 时校验；Reader 对 PC 做边界检查。
3. `Reader` 在帧末断言宏栈为空；开发构建下对所有状态栈做平衡断言。
4. 为上述每一项加回归测试（当前 `internal/ops` 覆盖率为 0）。

---

### A13. 布局约束契约存在漏洞（S2，中）

**证据**

- `Stack` 先给 Expanded 子级设 `Min = maxSZ`，之后才 `Constrain`（[layout/stack.go:87](../layout/stack.go#L87) vs [layout/stack.go:100](../layout/stack.go#L100)）。若某个 Stacked 子级返回超出父约束的尺寸（约束是建议性的），Expanded 子级会收到 `Min > Max`。实测可出现 `{Min:(100,100), Max:(50,50)}`。
- `Stack`/`Flex` 都不裁剪超出部分（[layout/stack.go:117-119](../layout/stack.go#L117-L119)、[layout/flex.go:209-212](../layout/flex.go#L209-L212)），而 `layout.go` 文档要求父级"适当处理"（[layout/layout.go:13-20](../layout/layout.go#L13-L20)）。
- `Gap` 是像素 `int`，而其余尺寸 API 用 `unit.Dp`（[layout/flex.go:25-26](../layout/flex.go#L25-L26)、`list.go:34-35`），跨 DPI 容易错。

**建议**

- 在进入 Expanded 循环前 `maxSZ = gtx.Constraints.Constrain(maxSZ)`。
- 在 `Stack`/`Flex` 返回前用 `clip.Rect` 裁剪到返回值；或明确文档"由调用方裁剪"并给出示例。
- `Gap` 改为 `unit.Dp`（破坏性变更，放入次版本）。

---

### A14. 控件的 Update/Layout 语义不统一（S2/S4，中）

**证据**

- `Clickable.Layout` 在一次调用里消费事件、绘制、加 clip/语义/输入 op（[widget/button.go:82-98](../widget/button.go#L82-L98)）；`Bool.Layout` 内部调用 `Update`（[widget/bool.go:41](../widget/bool.go#L41)）；`RadioButton` 同时调用 `Update` 和 `Layout`，两者都更新状态（[widget/material/radiobutton.go:41](../widget/material/radiobutton.go#L41)、`enum.go:118`）。
- `widget.Float` 在 `Layout` 里才写入 `axis/length`，而 `Update` 使用上一帧的值（[widget/float.go:29-32](../widget/float.go#L29-L32)、56）→ 首帧或尺寸变化后的拖动被丢弃。
- 零值 `Context` 静默表现为"单位 1:1、时间为零"（[layout/context.go:15-16](../layout/context.go#L15-L16)、`unit/unit.go:73-78`），而不是报错。

**建议**

- 统一为"`Update(gtx)` 处理事件 / `Layout(gtx, w)` 只绘制"；需要合并时提供 `LayoutAndUpdate` 并注明。
- `Float` 的几何从 `gtx.Constraints` 或显式参数推导，不依赖上一帧。
- `app.NewContext` 校验 `Metric`/`Ops` 非零（debug 构建），零值 Context 文档写明限制。

---

### A15. 公开 API 的数学错误：`Affine2D.Shear`（S2，中，已复现）

**证据**：[f32/affine.go:150-156](../f32/affine.go#L150-L156)

```go
func (a Affine2D) shear(radiansX, radiansY float32) Affine2D {
    ...
    return Affine2D{
        (a.a+1) + a.d*tx - 1, a.b + (a.e+1)*tx, a.c + a.f*tx,
        (a.a+1)*ty + a.d, a.b*ty + (a.e + 1) - 1, a.f*ty + a.f, // ← 最后一项应为 a.c*ty + a.f
    }
}
```

仿射矩阵为 `[a b c; d e f]`（内部 `e` 存 `sy-1`）。Y 方向错切应满足 `y' = ty*x + y + f`，平移项应为 `c*ty + f`，当前实现用的是 `f*ty + f`。

**复现**：`AffineId().Offset(10,20).Shear(Point{}, 0, 0.4).Transform(Point{})` 得到 `(10, 28.4559)`，正确值 `(10, 24)`。

**影响**：使用 `op.Affine(...Shear...)` 做斜切/伪 3D 的场景会整体偏移；纯错切（无平移）时 `c=f=0`，错误不显现，因此长期未被发现。

**修复**：改为 `a.c*ty + a.f`，并补"先平移后错切"的双轴测试。

---

### A16. 小范围缺陷与不一致（S4，低）

| 问题 | 证据 | 修复 |
|---|---|---|
| `Spacing.String()` 把 `SpaceBetween` 返回为 `"SpaceAround"` | [layout/flex.go:255-258](../layout/flex.go#L255-L258) | 改返回值 + 表驱动测试 |
| `TypeRedrawLen` 未使用 | `internal/ops/ops.go:133` | 删除或实现 |
| `ClipOp.Shape` 解码后无人消费 | `internal/ops/ops.go:169` | 使用或删除 |
| `OpType.String()` 对 semantic 类型 panic | `internal/ops/ops.go:494-495` | 补全映射 |
| `paint.NewImageOp` 在 Layout 内调用会每帧重建句柄并重传 | [op/paint/paint.go:72-97](../op/paint/paint.go#L72-L97) | 文档建议缓存到状态；GPU 侧做纹理池（P5） |

---

## 2. 性能瓶颈

> 数据来自 §3 基准；"影响面"指典型 UI 是否命中。

### P1. GPU：每个绘制 op 一次 draw call，无批处理（S3，高）

**证据**：`drawOps` 逐 op 执行 `BindTexture` → `BindPipeline` → `BindVertexBuffer` → 上传 128B uniform → `DrawArrays(0,4)`（[gpu/gpu.go:1235-1315](../gpu/gpu.go#L1235-L1315)）。文本的每个 32 字形块、每个图标都是独立 op。

**影响**：文本密集界面每帧数千次 2 三角形 draw；CPU 提交成为瓶颈，移动端功耗高。

**建议**：按 (pipeline, clip, atlas) 合并相邻 op，用 instancing 或顶点流一次提交；先做"连续同 pipeline 同 clip"的保守合并，风险最低。

**度量**：在 `gpu` 增加 `drawCalls` 计数，目标：文本页面 < 200。

---

### P2. GPU：路径几何每帧重建（S3，高）

**证据**：`opKey` 内嵌 `EncodedOp.Key`，其中 `version` 每次 `Ops.Reset` 自增（[internal/ops/ops.go:16-17,186](../internal/ops/ops.go#L16-L186)）；`collectOps` 用该 key 查 `pathCache`（[gpu/gpu.go:1040-1053](../gpu/gpu.go#L1040-L1053)）。跨帧必 miss → `buildVerts` + `NewImmutableBuffer`（[gpu/path.go:320-329](../gpu/path.go#L320-L329)），旧 buffer 被释放（[gpu/caches.go:129-143](../gpu/caches.go#L129-L143)）。

**影响**：圆角卡片、描边、图标 mask 每帧重新细分 + 上传；GPU 缓冲频繁创建/销毁。

**建议**：key = hash(aux 路径字节, 变换, 描边宽度, outline)；保活 N 帧的 LRU；只在宏身份变化时用 version。

---

### P3. GPU：逐 draw 的冗余状态与 uniform 解包（S3，中高）

**证据**：未裁剪路径会 `BindPipeline` 两次（[gpu/gpu.go:1255-1256](../gpu/gpu.go#L1255-L1256) 与 1287）；每个 draw 复制 128B 并解包成最多 6 次 `glUniform*`（[gpu/internal/opengl/opengl.go:1018-1049](../gpu/internal/opengl/opengl.go#L1018-L1049)）；D3D11 每次 draw 重设 shader/layout/blend/buffer/topology（`d3d11_windows.go:575-597`）。

**建议**：去掉重复 bind；缓存上次 pipeline/buffer/uniform 块；uniform 改 ring buffer + `glBindBufferRange`（1 次调用）。

---

### P4. GPU：Vulkan 每纹理 op 一个 barrier/descriptor（S3，中）

**证据**：`imageBarrier` 在 `srcStage != 0` 时总发 `CmdPipelineBarrier`，即使 layout 已匹配（`vulkan.go:791-821`）；`prepareDrawOps` 对每个 op 调 `PrepareTexture`（[gpu/gpu.go:1214-1233](../gpu/gpu.go#L1214-L1233)）。

**建议**：layout 相同直接返回；每纹理每帧只 Prepare 一次；descriptor set 按 (pipeline, textures) 缓存。

---

### P5. GPU：纹理与图集的生命周期抖动（S3，中）

**证据**：`texHandle` 以 `(filter, handle any)` 为 key，miss 就新建纹理并全量上传（[gpu/gpu.go:450-488](../gpu/gpu.go#L450-L488)）；一帧未用即删除（[gpu/caches.go:73-83](../gpu/caches.go#L73-L83)）。图集每帧重新打包（[gpu/gpu.go:789](../gpu/gpu.go#L789)、825），`fboSet.resize` 可能重建纹理（`path.go:258-286`，注释自认会 stall）。

**影响**：每帧重建 `paint.ImageOp` 的应用（动画、滚动文本、生成的位图）每帧分配 + 上传整张图。

**建议**：尺寸分桶纹理池 + 原地更新；条目保留 N 帧；图集"只增不减"。

---

### P6. 文本：动态路径每帧 2–3 次 O(N) 扫描（S3，高）

**证据与实测**

- 每帧 `textView.Layout` 在 `viewSize` 变化时 `invalidate`（[widget/text.go:305-309](../widget/text.go#L305-L309)），尽管 viewport 只影响滚动/光标几何。
- 失效后：段落 key 查找 + `yield(string(...))` 拷贝（[text/shaper.go:343](../text/shaper.go#L343)）→ 遍历字形 → 全量重建 `glyphIndex`（[widget/text.go:494-504](../widget/text.go#L494-L504)）→ 全量 grapheme 重分段（[widget/text.go:514-521](../widget/text.go#L514-L521)）。
- 实测 `EditorDynamic`：1000 runes 308 µs / 1,813 allocs；1000 runes RTL+emoji 751 µs；`EditorStatic` 仅 21 µs / 1 alloc。**差 15–35 倍**。

**建议**

1. 去掉 viewport 失效（在绘制时 clamp 可见范围）。
2. `glyphIndex` 增量 splice（只重建被编辑段落）。
3. grapheme 边界按段落懒计算，或直接复用 shaping cluster。
4. `SetText` 加相等短路（[widget/editor.go:879-887](../widget/editor.go#L879-L887)）；撤销历史按字节预算合并。

---

### P7. 文本：`document.append` 的 O(P·L) 行偏移（S3，中高）

**证据**：`append` 每次都对全部已累积行调用 `calculateYOffsets`（[text/gotext.go:45-49](../text/gotext.go#L45-L49)、[text/gotext.go:463-476](../text/gotext.go#L463-L476)）；`layoutText` 每段落调用一次。

**实测**（子代理，固定 1 万 runes）：10 段 117µs、100 段 129µs、1000 段 410µs；约 1 万段（10 万 runes）≈ 50ms/帧，且这是**缓存全命中**的代价。

**建议**：`append` 只累加 `runningY`；在 `layoutText` 结束后一次线性写回；或按段落缓存 `yOffset`。

---

### P8. 输入：命中测试超线性（S3，高，已实测）

**证据**：`hitTest` 对每个命中节点沿父链求逆变换（[io/input/pointer.go:550-599](../io/input/pointer.go#L550-L599)）；`addHandler`/`searchTag` 线性去重（[io/input/pointer.go:825-902](../io/input/pointer.go#L825-L902)）。

**实测**（`BenchmarkRouterAdd`）：

| handler | ns/op | 倍数 |
|---:|---:|---:|
| 3 | 617 | 1× |
| 9 | 1,706 | 2.8× |
| 27 | 8,235 | 13.3× |
| 81 | 55,547 | 90× |

**建议**

- `collect` 时预计算每个 area 的逆变换与局部包围盒；hitTest 先用矩形剔除。
- 命中链缓存：同一位置的连续事件（Move/Drag）复用上一次结果，仅在 `areas` 版本变化时失效。
- 用 bitmap/索引替代线性 `hitTree` 扫描（先按 Y 分桶）。

---

### P9. 输入：事件消费的 O(n) 删除（S3，中）

**证据**：每消费一个事件 `slices.Delete(change.events, j, j+1)`（[io/input/router.go:311](../io/input/router.go#L311)），并扫描所有 `changes`（[io/input/router.go:292-317](../io/input/router.go#L292-L317)）。

**建议**：每 `stateChange` 维护 `cursor`/位图；`Frame` 时统一 compact。

---

### P10. 输入：每帧克隆指针切片（S3，中）

**证据**：`deliverEnterLeaveEvents` 对未按下指针也 `changed = true; p.handlers = hits`（[io/input/pointer.go:855-862](../io/input/pointer.go#L855-L862)），触发 `slices.Clone`。

**建议**：比较 slice 是否真的变化再克隆。

---

### P11. app：装饰每帧全量布局（S3，中）

**证据**：`decorate` 每帧重建 `material.Decorations`、做一次 `layout` 与 `op.Record`（[app/window.go:815-865](../app/window.go#L815-L865)）。即使窗口未变（标题/焦点/模式都没变），也照做。

**建议**：缓存装饰 op，仅在 Config/Title/Focus/Mode 变化时重建；把 `deco.Update` 的指针事件处理保留每帧。

---

### P12. app：平台特定空转/日志（S3，中）

| 平台 | 问题 | 证据 |
|---|---|---|
| Windows | OCCLUDED 时动画分支仍空转，`Present` 错误被吞 | [app/os_windows.go:759-764](../app/os_windows.go#L759-L764)、`d3d11_windows.go:62-77` |
| OHOS | 每个 touch/axis 事件 `Sprintf`+stderr（**未提交调试代码**） | [app/ohos_pump.go:370](../app/ohos_pump.go#L370)、444 |
| Android | 每帧 2 次 JNI 查询 + a11y 查询 | `os_android.go:868,900` |
| macOS | 每帧 5 次 ObjC 调用；Metal 等待上一 command buffer | `os_macos.go:419-438`、`metal_darwin.go:512-516` |
| Wayland | 每次唤醒分配 1 字节切片 | `os_wayland.go:1560` |

---

### P13. 布局：`Flex` 的 scratch 临时分配（S3/S4，低）

**证据**：`scratch = append(scratch, make([]scratchSpace, len(children))...)`（[layout/flex.go:111-113](../layout/flex.go#L111-L113)）先分配完整临时切片，再拷进栈数组。注释声称"≤32 时在栈上"，实际临时切片仍会分配（编译器可能优化掉，但语义上多余）。

**建议**：

```go
var scratch []scratchSpace
if len(children) <= len(scratchArray) {
    scratch = scratchArray[:len(children)]
} else {
    scratch = make([]scratchSpace, len(children))
}
```

---

### P14. 事件循环的固定开销（S3，低-中）

**证据**：每帧 3 次通道会合（`frames`、`frameAck`、`events`），Android 上跨 Go↔Java；`app` 无任何 benchmark。

**建议**：加 `BenchmarkEventLoopFrame`；评估把 `frameAck` 改为"帧号 + 条件变量"或复用同一通道，减少唤醒。

---

### P15. 其他可优化点（S3/S4，低）

- `packLayers` 在循环内 `slices.Delete`（[gpu/gpu.go:814-823](../gpu/gpu.go#L814-L823)）→ 单遍 compact。
- `drawOps` 每个 op 复制两次 `imageOp`（约 100B material）→ 指针迭代。
- `stencilClips` 再次 `pathCache.get`，忽略 miss（[gpu/gpu.go:684](../gpu/gpu.go#L684)）。
- `headless` 每帧起一个 goroutine（`headless.go:145-157`）→ 固定 worker + recover。
- `opCache` 用 `[]opCacheValue` 线性查找？需确认（`caches.go:100-107`）；若为线性，路径多时是 O(n²)。

---

### P16. material：每帧的全文复制与重复计算（S3，中）

**证据与实测**

- `material.LabelStyle` 每帧用 `State.Text() != l.Text` 判断变化（[widget/material/label.go:134](../widget/material/label.go#L134)），而 `Selectable.Text()` 会 `string(l.scratch)` 复制全文（[widget/selectable.go:156-157](../widget/selectable.go#L156-L157)）。实测 **1 KB 文本 = 1024 B / 1 alloc / 帧**。
- 编辑器 hint 每帧都整形并录宏，即使编辑器非空（[widget/material/editor.go:72-93](../widget/material/editor.go#L72-L93)）。
- `LabelStyle` 无论是否选中都构造选中色宏（[widget/material/label.go:126-131](../widget/material/label.go#L126-L131)）。
- `Switch` 更新两次（`switch.go:42` + `bool.go:41`）；`RadioButton` 两次（`radiobutton.go:41` + `enum.go:118`）。
- 宏层层嵌套：每个 Button→Background→Inset→Label 链多付 26 B/层与一次 reader 栈帧（实测）。

**建议**

- 用 `Selectable.Equal(string)` 或版本号比较，避免复制；只在变化时 `SetText`。
- hint 用 `Editor.Len() == 0` 门控并缓存其 CallOp。
- 条件构造颜色宏；每个控件只 Update 一次；合并 Button 的 `Background`/`Inset` 宏。

---

### P17. 形状编码与描边的固定开销（S3，中）

**证据与实测**

- 同一个圆角矩形：Clip 只写 20 B，而 `clip.RRect` 走 `clip.Path` 要 376 B（100×50、r=8）；椭圆 216 B；空 `Record/Stop/Add` 26 B。
- `clip.Path` 的命令哈希被计算并写入，但**没有任何消费者读取**（[op/clip/clip.go:74-78](../op/clip/clip.go#L74-L78)、[op/clip/clip.go:224-227](../op/clip/clip.go#L224-L227)）。
- `decodeToStrokeQuads` 对每条描边路径新建 quads 与 scratch 切片（[internal/stroke/stroke.go:637-638](../internal/stroke/stroke.go#L637-L638)）。
- `layout.List` 外层 `op.Record` 包住的只是子项宏定义，随后整体回放，实际没有内容（[layout/list.go:121](../layout/list.go#L121)、370-375）。

**建议**

- 消费已有内容哈希：把它作为 GPU 路径缓存 key 的一部分（见 P2），避免"算了不用"。
- 为 Rect/RRect/Ellipse 增加专用 op（或 GPU 侧形状快路径），避免每个圆角 376 B 的路径编码。
- 描边 scratch 用 `sync.Pool` 或 `drawOps` 级复用。
- 删除 `List` 的外层宏。

---

## 3. 可读性与优雅性

### R1. 巨型文件（S4，中）

| 文件 | 行数 | 混合的关注点 |
|---|---:|---|
| `app/os_wayland.go` | 2,221 | C 回调、协议注册、输入、IME、剪贴板、fling、CSD |
| `internal/vk/vulkan.go` | 2,141 | 全部 Vulkan 绑定 |
| `internal/d3d11/d3d11_windows.go` | 1,695 | COM/DXGI/D3D11 |
| `gpu/gpu.go` | 1,593 | 解码、路径缓存、图集、pass 调度 |
| `app/os_android.go` | 1,583 | JNI、a11y、IME、输入、权限 |
| `app/os_windows.go` | 1,492 | 消息循环、DPI、输入、D3D11 |
| `widget/editor.go` | 1,210 | 输入、撤销、IME、绘制、选择 |

**建议**：按关注点拆文件（不改包边界、不改 API）。优先 `gpu/gpu.go`（拆 `collector.go`/`renderer.go`/`layers.go`）与 `io/input/pointer.go`（拆 `collector.go`/`hit.go`/`semantic.go`）。

---

### R2. 巨型 switch 与重复的配置逻辑（S4，中）

- `windowProc` 276 行（[app/os_windows.go:294-569](../app/os_windows.go#L294-L569)）。
- 7 份 WindowMode 应用逻辑（约 360 行）。
- `collectOps` 180 行 switch（[gpu/gpu.go:971-1148](../gpu/gpu.go#L971-L1148)）。
- `Router.collect` 95 行 switch（[io/input/router.go:787-883](../io/input/router.go#L787-L883)）。

**建议**：配置逻辑提取为 `planConfig(Config) []action` + 平台 executor；op 分发用表驱动/生成代码；`windowProc` 拆成 `handleSize`/`handleInput`/`handleDPI` 等。

---

### R3. 重复实现（S4，中）

| 重复 | 位置 | 建议 |
|---|---|---|
| 顶点编码两份，靠注释保持同步 | `gpu/clip.go:20-48` vs `gpu/path.go:117-128` | 抽 `vertexLayout` + 编译期断言 |
| 键位映射 8 份 | `ohos_input.go`、`os_android.go`、`os_windows.go`… | `internal/keymap` 表 |
| 光标映射 7 份 | 各 `os_*` | `internal/cursor` 表 |
| IME 两套 | `app/ime.go`（Wayland） vs OHOS | 统一 `IMEController` |
| caret 调整两处 | `widget/editor.go:1044-1056` vs `widget/text.go:594-603` | 单一 `textView.AdjustCaret` |
| `atStartOfLine` 死代码 | `widget/index.go:277` | 删除或接入 |

---

### R4. 死代码与半成品（S4，低-中）

`staticcheck` 21 条（见 [verification.md §5](verification.md#5-staticcheck-结果全量)），关键项：

- `app/os.go:212-243` `windowRendezvous` 整个类型 + 后台 goroutine 未使用（若被启用会泄漏一个 goroutine）。
- `app/ime.go` 6 处未使用字段/函数（`sentValid`、`external`、`reset`、`clearPreedit`、`requestSurroundingText`、`prepareSurroundingText`）。
- `app/window.go:827-833` `actions` 计算后被丢弃——**可能是功能缺陷**（装饰按钮动作未生效）。
- `gpu/gpu.go:389,434` `if false && ...` 计时器整段死代码。
- `gpu/clip.go:57`/`gpu/path.go:117` 两套顶点编码，一套未用。

**建议**：清空 staticcheck；对 `actions` 那处单独确认是否为 bug。

---

### R5. 注释与文档漂移（S4，低）

- `router.go:133-134` 注释说 `prevFilter`，实际字段是 `nextFilter`，且语义已变。
- `gpu/pack.go:58-59` 注释"Go backwards"但循环向前。
- `pathData.ncurves` 实际是顶点数（除以 4 使用）。
- `1e6` 哨兵值无解释（`gpu/gpu.go:1084`、`pack.go:44`）。
- OHOS 注释引用不存在的 `docs/44`、`docs/45`。
- 中英文注释混排。

**建议**：把关键不变量写成 `// INVARIANT:` 注释；CI 检查 TODO/文档引用。

---

### R6. 命名与 API 小瑕疵（S4，低）

- `Window.Run` 的文档说"或窗口关闭时返回"，但实现不能（见 A9）。
- `event.Tag` 要求"指针稳定且可比较"，但无编译期/运行期校验；每帧新建 tag 会静默重置手势状态。
- `op.Defer` 是 FIFO，与 Go `defer` 相反（已文档化）。
- `FrameEvent.Source` 与 `Window` 都可 `Queue`，职责重叠。
- `input.Source` 的零值"disabled"是隐式行为，容易误用。

---

### R7. 测试结构（S4，中）

| 区域 | 现状 | 建议 |
|---|---|---|
| 内核 | `internal/ops`、`scene`、`stroke` 几乎无测试 | 先补 op 编解码 golden test、stroke 性质测试 |
| GPU | 只有 `rendertest`（端到端像素对比） | 增加 `drawOps.collect` 的纯函数测试与 draw-call 计数 |
| 平台 | 0 测试 | headless/Xvfb 冒烟；C ABI 用 fake host |
| 并发 | 无 `-race` CI | 加 `go test -race ./...` |
| 基准 | 25 个，集中在 widget/f32 | 补 `app`/`gpu`/输入命中 |

---

### R8. `widget/material` 与 `widget` 的重复表达（S4，中）

**证据**

- `LabelStyle` 重复 `widget.Label` 的六个 shaping 字段（Alignment、MaxLines、WrapPolicy、Truncator、LineHeight、LineHeightScale），并每帧写回 Selectable（[widget/material/label.go:21-55](../widget/material/label.go#L21-L55)、133-143）。
- `checkable` 重新拼装 checkbox/radio 的组合（[widget/material/checkable.go:32-84](../widget/material/checkable.go#L32-L84)）。
- 三种 Button 样式 + Switch 各自重复 background/ink/hover 逻辑（`button.go:93-198`、`switch.go:41-130`）。
- `ListStyle` 重写了滚动条的位置/布局数学（`material/list.go:24-59`、263-323）。
- 硬编码度量绕过主题：`trackWidth := gtx.Dp(36)`（`switch.go:43-45`）、`size := gtx.Dp(c.Size) * 4 / 3`（`checkable.go:44`）。
- `shaper` 字段在 Button/EditorStyle 是私有，在 LabelStyle 是公开（`button.go:32`、`editor.go:37`、`label.go:51`）。

**建议**

- 提取 `material.Surface`/`Ink` 共享绘制；`LabelOptions` 单一结构；度量从 `Theme` 读取。
- 统一 `shaper` 的可见性（建议都私有 + `WithShaper`）。
- 为每个样式补一张 golden 截图测试（headless），防止重构改变视觉。

---

## 4. 优化路线图

### 阶段 0：止血（1–2 周，低风险）

1. `layoutKey` 增加 `DisableSpaceTrim` + 参数组合测试。
2. 语义 `valid = valid || gestures != 0` + 测试。
3. GPU layer 区间越界保护 + `packLayers` 断言。
4. 删除 OHOS 逐事件日志；清空 staticcheck。
5. `timer.quit = nil` 复位；`Run` 增加 `done` 取消；`driver` 读写统一加锁。
6. `textView` 去掉 viewport 失效；`document.append` 去掉全量 y 重算。
7. `ops.Reset` 清 `multipOp`；未完成 Record/Path 在帧末断言并给出可读错误。
8. 修 `f32.Affine2D.Shear` 的 Y 平移项；修 `Spacing.String`。
9. `material` 去除每帧全文复制（`Selectable.Equal`）与空编辑器 hint 布局。
10. CI 加 `go vet` + `staticcheck` + `-race`（先在 `io/input`、`app`）。

**验收**：`go test ./...` 与 `-race` 全绿；编辑 1000 runes 每键 < 200µs（阶段 1 后 < 100µs）；无 staticcheck。

### 阶段 1：结构性改善（1–2 月，中风险）

1. `app` 拆分 + 装饰注入，去掉 `app→material` 依赖。
2. `driver` 拆成 client/window-thread 两个接口；平台无关层 `internal/platform`（config/key/cursor/IME）。
3. `internal/ops` 受检访问器 + 版本头 + dump 工具。
4. 文本 `glyphIndex` 增量更新；`SetText` 短路；撤销合并。
5. 输入命中索引（预计算逆变换/包围盒）+ 事件消费游标。
6. GPU：去掉重复 bind、uniform ring、路径缓存 key 改内容哈希。

**验收**：`BenchmarkRouterAdd/81` 下降 5×；`EditorDynamic/1000` < 100µs；圆角 clip 的 GPU buffer 每帧新建数 ≈ 0。

### 阶段 2：渲染与平台深化（1 季度，中高风险）

1. GPU draw batching（先保守合并，再 instancing）。
2. 纹理池 + 跨帧图集策略。
3. Vulkan barrier/descriptor 优化 + 环形 staging。
4. 平台后端拆分与共享 key/cursor 表；每个后端冒烟测试。
5. OHOS：生命周期、ABI 协商、队列合并；把 IME 并入统一控制器。

**验收**：文本页面 draw calls < 200；Vulkan 下 4K 圆角窗口不 panic；新增后端只需 1 个文件 + 1 个注册点。

### 阶段 3：可选（按需）

- `internal/ops` 代码生成 + 正式格式文档。
- WebAssembly 多线程/OffscreenCanvas。
- 录制回放调试器（基于 op dump）。

---

## 5. 目标架构（建议）

```
app/                      公共 API：Window/FrameEvent/Option
  internal/window/        事件循环、coalesce、装饰编排、语义桥
  internal/platform/
    wayland/ x11/ windows/ macos/ android/ ios/ js/ ohos/
      driver.go           实现统一 driver
      config.go           planConfig + executor
      input.go            key/cursor 表
      ime.go              统一 IMEController
  internal/deco/          默认装饰（只依赖 op/layout/unit）

gpu/
  collector.go            ops → imageOps/pathOps/layers（纯函数，可测）
  renderer.go             pass 编排
  layers.go               不透明度层
  batch.go                批处理（阶段 2）
  internal/driver/        不变
  internal/{opengl,vulkan,metal,d3d11}/

io/input/
  router.go               路由与命令
  collector.go            ops → areas/hitTree
  hit.go                  命中索引（预计算）
  semantic.go             语义树
  focus.go                焦点
  clipboard.go

text/
  shaper.go               公共 API
  layout.go               段落切分 + 增量 y 偏移
  cache.go                统一 LRU（字节预算 + 命中率）
  gotext.go               整形后端

internal/ops/             受检编码/解码 + 版本 + dump
```

**不改变的东西**：op 流作为单一事实来源；`layout.Context` 值语义；控件状态由用户持有；`f32`/`unit` 类型。

---

## 6. 不建议做的事

| 反模式 | 原因 |
|---|---|
| 引入保留模式控件树 / 虚拟 DOM | 会摧毁零分配与跨后端一致性，收益不确定 |
| 把 op 流整体替换为结构体切片 | 分配与缓存局部性变差；op 流是性能基石 |
| 给所有 API 加 `context.Context` | Gio 是同步帧模型，取消语义只对窗口/GPU 初始化有意义 |
| 并行化布局/绘制 | 单帧内并行收益小、同步成本高；应优化单线程常数 |
| 为 OHOS 单独 fork 仓库 | 共享代码收益大于隔离；应通过 `internal/platform` 收敛 |
| 全面重写平台后端 | 逐平台替换 + 冒烟测试更稳；先抽共享层 |

---

## 7. 度量与验收指标

| 维度 | 指标 | 目标 |
|---|---|---|
| 正确性 | `go test -race ./...` | 0 race |
| 静态 | staticcheck 告警 | 0 |
| 文本 | `EditorDynamic/1000runes` | < 100 µs，< 500 allocs |
| 文本 | 1 万段（10 万 runes）布局 | < 5 ms |
| 输入 | `RouterAdd/81` | < 10 µs |
| GPU | 文本页 draw calls | < 200 |
| GPU | 跨帧 path cache 命中率 | > 90% |
| GPU | 每帧新建 GPU buffer 数 | ≈ 0（稳定 UI） |
| app | 关闭→重开窗口 | 无挂起、无竞态 |
| 包体 | `app` 依赖闭包 | < 120 包（去掉 material） |

---

## 8. 附：问题索引

| ID | 严重度 | 一句话 | 位置 |
|---|---|---|---|
| A1 | S4 | app 上帝包 + 依赖 material | `app/window.go` |
| A2 | S2 | ops 无 schema/版本，类型断言 | `internal/ops` |
| A3 | S1 | panic-first，Frame 恒返回 nil | `gpu/gpu.go:443` |
| A4 | S1 | driver 线程契约模糊 + 后端复制 | `app/os.go` |
| A5 | S1 | Router 无同步 | `io/input/router.go` |
| A6 | S2 | 键盘事件无 tag | `io/input/router.go:450` |
| A7 | S4 | GPU 注册表全局可变 | `gpu/internal/driver/api.go:86` |
| A8 | S2 | 缓存 key 契约脆弱 | `text/lru.go`、`gpu/caches.go` |
| A9 | S1 | 生命周期/取消不完整 | `app/window.go` |
| A10 | S4 | OHOS 耦合与调试残留 | `app/ohos_*` |
| A11 | S3 | 平台分支/反向依赖等杂项 | 见 A11 表 |
| A12 | S1 | op 生命周期未硬化（静默丢帧/永久 panic） | `internal/ops/reader.go:150`、`ops.go:172` |
| A13 | S2 | Stack Expanded Min>Max、不裁剪 | `layout/stack.go:87` |
| A14 | S2 | Update/Layout 语义不一致、Float 用旧几何 | `widget/float.go:29` |
| A15 | S2 | `Affine2D.Shear` 数学错误（**已复现**） | `f32/affine.go:154` |
| A16 | S4 | 元数据/命名小缺陷 | 见 A16 表 |
| P1 | S3 | 无 draw batching | `gpu/gpu.go:1235` |
| P2 | S3 | 路径每帧重建 | `gpu/gpu.go:162` |
| P3 | S3 | 冗余 bind/uniform | `gpu/gpu.go:1255` |
| P4 | S3 | Vulkan barrier 过多 | `vulkan.go:791` |
| P5 | S3 | 纹理/图集抖动 | `gpu/gpu.go:450` |
| P6 | S3 | 文本动态 O(N) 多次 | `widget/text.go` |
| P7 | S3 | O(P·L) 行偏移 | `text/gotext.go:45` |
| P8 | S3 | 命中测试 O(n²) | `io/input/pointer.go:550` |
| P9 | S3 | 事件消费 O(n) delete | `io/input/router.go:311` |
| P10 | S3 | 每帧克隆指针切片 | `io/input/pointer.go:859` |
| P11 | S3 | 装饰每帧全量布局 | `app/window.go:815` |
| P12 | S3 | 平台空转/日志 | 见 P12 表 |
| P13 | S4 | Flex scratch 临时分配 | `layout/flex.go:111` |
| P14 | S3 | 事件循环 3 次 handoff | `app/os.go:260` |
| P15 | S3 | 其它微优化 | 见 P15 |
| P16 | S3 | material 每帧全文复制/重复更新 | `widget/material/label.go:134` |
| P17 | S3 | 形状编码与描边分配 | `op/clip/clip.go:224`、`internal/stroke/stroke.go:637` |
| R1–R8 | S4 | 可读性/测试 | 见 §3 |