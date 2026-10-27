# 模块参考

> 按包说明职责、关键文件/类型、依赖、设计要点与已知问题。行号对应当前 HEAD。
> 只列同仓库内的重要依赖；通用库（`golang.org/x/*`、`go-text`）不展开。

---

## 0. 速查表

| 包 | 角色 | LOC | 测试比 | 主要依赖 | 健康度 |
|---|---|---:|---:|---|---|
| [`app`](#app) | 窗口/事件循环/平台后端 | 15,105 | 0.08 | gpu, io, layout, text, widget, material | ⚠️ 上帝包 |
| [`gpu`](#gpu) | 渲染器 | 2,568 | 0.02 | driver, ops, scene, stroke, layout | ⚠️ 无测试 |
| [`layout`](#layout) | 布局 | 1,288 | 0.63 | op, clip, pointer, gesture | ✅ |
| [`text`](#text) | 文本整形 | 1,980 | 0.70 | font, op, paint | ⚠️ 缓存契约 |
| [`widget`](#widget) | 控件 | 4,442 | 0.82 | layout, text, gesture | ✅ |
| [`widget/material`](#widgetmaterial) | Material 主题 | 1,853 | 0.04 | widget, icons | ⚠️ 测试少 |
| [`gesture`](#gesture) | 手势 | 480 | 0.40 | pointer, fling | ⚠️ 边界语义 |
| [`op` / `op/clip` / `op/paint`](#op) | 绘制原语 | 991 | 0.16–0.38 | internal/ops, scene, stroke | ✅ |
| [`io/input`](#ioinput) | 事件路由 | 2,385 | 0.86 | internal/ops, io/* | ⚠️ 单文件过大 |
| [`io/*`](#io) | 事件类型 | 1,274 | 0–0.21 | op | ✅ |
| [`internal/ops`](#internalops) | op 编解码 | 689 | 0 | 无 | ⚠️ 核心无测试 |
| [`internal/*`](#internal) | 几何/描边/颜色 | 1,606 | 0–0.26 | f32 | ⚠️ stroke 5% |
| [`gpu/internal/*`](#gpuinternal) | GPU 后端 | 4,792 | 0 | driver, shader | ⚠️ FFI 密集 |
| [`internal/{gl,vk,d3d11,egl}`](#platformffi) | 图形 API 绑定 | 7,698 | 0 | — | ⚠️ unsafe 密集 |
| [`app/internal/*`](#appinternal) | 平台 API 绑定 | 1,737 | 0 | — | ⚠️ |
| [`f32` / `unit`](#f32unit) | 基础类型 | 322 | 0.62–1.59 | 无 | ✅ |
| [`font` / `font/*`](#font) | 字体 | 402 | 0 | go-text | ✅ |

---

## app

**职责**：`Window` 生命周期、帧/事件调度、平台后端、IME、剪贴板、窗口装饰、语义桥。

**关键文件**

| 文件 | 行数 | 内容 |
|---|---:|---|
| `app/window.go` | 1,016 | `Window`、`FrameEvent`、coalesce、`validateAndProcess`、装饰、语义、IME 回调 |
| `app/os.go` | 367 | `driver`/`context` 接口、`eventLoop`、`Config` |
| `app/app.go` | 199 | `Main`/`main` 入口、`osMain` 调度 |
| `app/ime.go` | 420 | 平台无关 IME 状态机（仅 Wayland 使用） |
| `app/os_wayland.go` | 2,221 | Wayland + EGL + XKB + CSD |
| `app/os_android.go` | 1,583 | JNI、权限、a11y |
| `app/os_windows.go` | 1,492 | Win32 消息循环、DPI、D3D11 |
| `app/os_macos.go` | 1,246 | Cocoa、Metal、捏合、拖放 |
| `app/os_js.go` | 1,163 | DOM/WebGL |
| `app/os_x11.go` | 1,045 | Xlib + IBus |
| `app/os_ohos.go` + `ohos_*` | 1,693 | OHOS（fork） |
| `app/os_ios.go` | 451 | UIKit |
| `app/window.go` 中的 `decorate` | — | 用 `material.Decorations` 画客户端装饰 |

**设计要点**

- `eventLoop` 是"客户端 goroutine ↔ 平台线程"的会合点，通道无缓冲（[app/os.go:245-340](../app/os.go#L245-L340)）。
- `validateAndProcess` 处理 surface 重建与 device lost 重试（[app/window.go:134-217](../app/window.go#L134-L217)），但 `gpu.Frame` 永远返回 nil，见 GPU A1。
- 装饰 op 与用户 op 通过宏拼接，一次提交（[app/window.go:234-249](../app/window.go#L234-L249)）。

**已知问题**（详见 [risks-and-optimizations.md](risks-and-optimizations.md#3-架构风险)）

- 反向依赖 `widget/material`；初始化即建主题与整形器（[app/window.go:761-793](../app/window.go#L761-L793)）。
- `w.driver` 读写不加锁（写 `invMu`，读裸访问）；`Run` 在 destroy 后可能永久阻塞；timer 关闭后 `quit` 未清空导致再次初始化死锁。
- 8 个后端的 key/cursor/configure/IME 代码大量重复。
- 5 个平台文件 >1,000 行；Windows `windowProc` 276 行。
- OHOS 后端为进程单例、无 teardown、无 ABI 协商、未提交代码里有逐事件日志。

---

## gpu

**职责**：把 op 流编译成 GPU 命令；裁剪/stencil 图集；纹理与缓冲缓存。

**关键文件/类型**

| 文件 | 行数 | 内容 |
|---|---:|---|
| `gpu/gpu.go` | 1,593 | `gpu`、`renderer`、`drawOps`、`collectOps`、绘制 pass |
| `gpu/path.go` | 426 | stencil 图集、FBO、顶点编码 |
| `gpu/caches.go` | 153 | 纹理/路径 cache |
| `gpu/clip.go` | 146 | 四边形剖分与顶点编码 |
| `gpu/pack.go` | 109 | shelf 装箱 |
| `gpu/api.go` | 40 | 公共类型别名（`API`/`RenderTarget`/`ErrDeviceLost`） |

**数据流**：`collect` 解码 op → `imageOps`/`pathOps`/`layers` → `frame` 做 stencil → intersect → upload → layer FBO → final draw。

**设计要点**

- 全项目最热的两个函数：`collectOps`（180 行 switch，[gpu/gpu.go:971-1148](../gpu/gpu.go#L971-L1148)）与 `drawOps`（逐 op 一次 draw，[gpu/gpu.go:1235-1279](../gpu/gpu.go#L1235-L1279)）。
- 缓存复用做得很好（`vertCache`、`pathOpCache`、`slices[:0]`），但跨帧路径缓存因 key 含 version 而失效。
- 失败模型是 panic；`Frame` 的 error 恒为 nil。

**已知问题**：逐 op draw call（无批处理）、路径每帧重建、Vulkan 4096 纹理上限 panic、全屏清除作废 opacity layer、`if false` 死代码。

---

## layout

**职责**：整数像素约束求解、`Flex`/`Stack`/`List`、`Context`。

| 文件 | 行数 | 内容 |
|---|---:|---|
| `layout/layout.go` | 344 | `Constraints`/`Dimensions`、`Axis`、单位换算 |
| `layout/flex.go` | 264 | 主轴/交叉轴分配、权重、gap |
| `layout/list.go` | 409 | 可回收的滚动列表（只布局可见子项） |
| `layout/stack.go` | 163 | 叠放 + 对齐 |
| `layout/context.go` | 53 | `Context` |

**设计要点**

- `List` 是"测量—回收—复用 CallOp"的典型：只保留可见元素的宏，滚动时把已测量的 op 重放（[layout/list.go:270-383](../layout/list.go#L270-L383)）。
- `Flex` 为逃逸分析做了特殊处理（scratch 数组），但实现有浪费：`scratch = append(scratch, make([]scratchSpace, len(children))...)`（[layout/flex.go:111-113](../layout/flex.go#L111-L113)）会先分配整个临时切片；应改为 `scratchArray[:len(children)]` 或 `make` 二选一。
- `Stack` 极轻（34 ns/0 alloc，见验证文档）。

**已知问题**

- `Stack` 给 Expanded 子级的 `Min` 可能大于 `Max`（先设 Min、后 Constrain，[layout/stack.go:87](../layout/stack.go#L87) vs 100），且 `Stack`/`Flex` 不裁剪溢出。
- `Flex`/`Stack` 的 scratch 写法 `append(scratch, make([]T, n)...)` 先分配整个临时切片（[layout/flex.go:111-113](../layout/flex.go#L111-L113)、`stack.go:66-68`），与注释声称的"栈上分配"矛盾。
- `Spacing.String()` 把 `SpaceBetween` 返回为 `"SpaceAround"`（[layout/flex.go:255-258](../layout/flex.go#L255-L258)）。
- `Gap` 是像素 `int`，与 `unit.Dp` 体系不一致。
- `List` 用"已布局平均高度"估算总长（[layout/list.go:135-140](../layout/list.go#L135-L140)），变高列表的 `ScrollBy` 不精确；`Position` 既是输入又是输出，语义耦合。
- `List` 外层宏只包住子项宏定义，回放是死工作（[layout/list.go:121](../layout/list.go#L121)、370-375）。
- 缺少 `Grid`/`Flow` 等常用布局（可基于 Flex 组合，但重复代码多）。

---

## text

**职责**：文本整形（go-text/typesetting）、段落切分、换行/bidi、字形/位图/段落缓存。

| 文件 | 行数 | 内容 |
|---|---:|---|
| `text/shaper.go` | 664 | 公共 `Shaper`、`Parameters`、`NextGlyph`、`Shape/Bitmaps` |
| `text/gotext.go` | 826 | `shaperImpl`：字体解析/回退、HarfBuzz、UAX#14 换行、bidi、位图 |
| `text/lru.go` | 183 | 三类 LRU：文档/路径/位图 |
| `text/family_parser.go` | 246 | CSS 风格 family 解析 |
| `text/text.go` | 56 | `Glyph`、`Alignment`、`WrapPolicy` |

**设计要点**

- 缓存边界：`layoutCache`（按段落文本+10 个参数，1000 条）→ `pathCache`/`bitmapShapeCache`（按 32 字形块）→ `bitmapGlyphCache`（按 GlyphID）。
- `GlyphID` 自描述：`face(16) | ppem(16) | gid(32)`（[text/shaper.go:609-623](../text/shaper.go#L609-L623)），因此 `Shape`/`Bitmaps` 不需要重新整形。
- `NextGlyph` 是带状态的迭代器，RTL 反向遍历，靠 flag 传递换行/段落/簇边界。

**已知问题**：`layoutKey` 漏 `DisableSpaceTrim`（**已复现**）、`document.append` 的 O(P·L) 行偏移重算、段落级 `string(...)` 拷贝、`textView` 因 viewport 变化而整体失效、缓存 key 不含 `Offset/Bounds`（可能几何串味）、缓存以"块数"而非字节计。

---

## widget

**职责**：可复用有状态控件。

| 文件 | 行数 | 内容 |
|---|---:|---|
| `widget/editor.go` | 1,210 | 编辑器：输入、选择、撤销、IME、绘制 |
| `widget/text.go` | 855 | `textView`：参数同步、索引、光标、绘制 |
| `widget/index.go` | 537 | `glyphIndex`：字形→rune/坐标、行信息、grapheme |
| `widget/selectable.go` | 394 | 只读可选择文本 |
| `widget/list.go` | 203 | 滚动条 + 阻尼滚动 |
| `widget/button.go` | 187 | 通用按钮状态机 |
| `widget/label.go` | 226 | 文本绘制、截断、省略号 |
| `widget/dnd.go` | 92 | 拖放源/目标 |
| `widget/buffer.go` | 128 | gap buffer |
| 其它 | — | `Bool`/`Enum`/`Float`、`Border`、`Fit`、`Image`、`Icon`、`Decorations` |

**设计要点**

- 状态全部由调用方持有；控件本身可零值使用（`var e widget.Editor`）。
- `textView` 是 Editor/Selectable/Label 的共享内核：12 个字段变化触发 `invalidate`，然后重建 `glyphIndex` 与 grapheme 表。
- IME 通过 `gtx.Queue` 发命令，`EditorState` 由 `input.Router` 收集，窗口在 `updateState` 中推给平台。

**已知问题**

- 每帧 O(N) 索引重建 + O(N) grapheme 重分段；`Len()`/`ByteOffset` 会触发 `makeValid` → 全文整形，因此 `Replace` 在一次编辑中可能整形多次。
- `SetText` 无相等短路（[widget/editor.go:879-887](../widget/editor.go#L879-L887)），绑定模型的写法会每帧全量替换并追加撤销记录。
- 撤销历史无上限、无合并；`filterAndLimit` 按字节过滤多字节字符。
- 编辑器与 `textView` 的 caret 调整逻辑重复（`editor.go:1044-1056` vs `text.go:594-603`）。
- Update/Layout 语义不统一：`Clickable.Layout` 同时消费事件与绘制；`Bool.Layout` 内部 Update；`RadioButton` 更新两次；`Float` 用上一帧的 axis/length。
- `material.LabelStyle` 每帧通过 `Selectable.Text()` 复制全文。

---

## widget/material

**职责**：Material Design 主题化包装（颜色、排版、图标、状态）。

| 文件 | 行数 | 内容 |
|---|---:|---|
| `material/theme.go` | 95 | `Theme`（颜色/字体/Shaper/图标） |
| `material/button.go` | 297 | 按钮/图标按钮 |
| `material/list.go` | 330 | 列表项、分隔线、滚动条 |
| `material/decorations.go` | 197 | 窗口装饰（**被 app 依赖**） |
| `material/label.go` | 154 | 文本样式 |
| `material/editor.go` | 102 | 编辑器样式 + hint |
| 其它 | — | checkbox/radio/switch/slider/progress/loader |

**设计要点**：`Layout(gtx, ...)` 是纯函数式的样式包装，不持有业务状态。`Theme.Shaper` 被全窗口共享。

**已知问题**

- 测试比 0.04；每个控件都重新拼装 `layout.Flex`/`Stack`，存在重复的尺寸计算。
- `LabelStyle` 重复 `widget.Label` 的 6 个 shaping 字段，每帧写回并复制文本（1 KB → 1024 B/1 alloc）。
- 编辑器 hint 每帧整形并录宏（即使非空）；`LabelStyle` 无条件构造选中色宏；`Switch`/`RadioButton` 重复 Update。
- 硬编码度量绕过主题（`gtx.Dp(36)`、`Dp(c.Size)*4/3`）。
- `shaper` 字段可见性不一致（Button/Editor 私有，Label 公开）。

---

## gesture

**职责**：把指针事件流识别为 Click/Drag/Scroll/Fling/Hover。

**设计要点**

- 每个手势是独立的小状态机，通过 `pointer.Filter` 声明关注的事件类型。
- `Click` 用"按到按"200ms 判定双击次数。
- `Scroll` 用 `fling.Extrapolation` 估计速度，并用 `fling.Animation` 做惯性。

**已知问题**

- `Scroll` 不持有 `ScrollRange`，无法在边界停止/移交；惯性期间新的按下不会 `Stop`。
- Grab 时会话首帧可能一次跳变（`s.last` 未同步）。
- `Both` 轴是 X+Y，不是投影。
- 速度采样用事件时间，惯性 tick 用 `FrameEvent.Now`，两个时钟可能不一致。

---

## op

**职责**：op 的公共 API 与宏/变换/裁剪/绘制原语。

| 文件 | 行数 | 内容 |
|---|---:|---|
| `op/op.go` | 230 | `Ops`、`MacroOp`、`CallOp`、`Transform`、`Save/Load`、`Defer` |
| `op/clip/clip.go` | 356 | `Rect`/`Ellipse`/`Outline`/`Path`、`Push/Pop` |
| `op/clip/shapes.go` | 175 | 常用形状 |
| `op/paint/paint.go` | 195 | `ColorOp`/`PaintOp`/`ImageOp`/`LinearGradientOp` |

**设计要点**：`Ops` 只有 3 个字段（`Internal ops.Ops`），零值可用；`MacroOp` 用栈 ID 校验配对。`clip.Path` 用 `Begin/LineTo/QuadTo/CubeTo` 构建，写入 scene 命令。

**已知问题**

- `Record` 未 `Stop` 时，reader 把未完成宏当作"包含剩余全部 op"，静默丢弃后续绘制（[internal/ops/reader.go:150-160](../internal/ops/reader.go#L150-L160)）。
- `ops.Reset` 不清 `multipOp`：一次未 `End` 的 `clip.Path` 会让 `Ops` 永久 panic（**已复现**）。
- `CallOp.Add` 文档称 Reset 后会 panic，实际无代次校验，旧 CallOp 可产生零长度 op。
- `clip.Path` 的内容哈希被计算但无人读取；每个 RRect 走路径编码要 376 B（Rect 只要 20 B）。
- `op.Defer` 是 FIFO，与 Go 的 `defer` 相反（已文档化但易错）。

---

## io/input

**职责**：事件路由、指针/键盘队列、焦点、语义树、剪贴板命令。

| 文件 | 行数 | 内容 |
|---|---:|---|
| `router.go` | 905 | `Router`、`Source`、命令执行、焦点移动、语义入口 |
| `pointer.go` | 1,023 | `pointerQueue`、`pointerCollector`、命中测试、Enter/Leave、传输 |
| `key.go` | 366 | `keyQueue`、焦点、编辑器状态、方向键焦点 |
| `clipboard.go` | 71 | 剪贴板读写队列 |

**设计要点**：每帧从 op 重建 area/hitTree/semantic；过滤器双缓冲（上一帧声明、本帧路由）；指针 capture 是"按下时固定 handlers 切片"。

**已知问题**：无同步（平台线程 `Queue` vs 客户端 `Event`）；键盘事件不携带目标 tag（先到先得）；语义 valid 被覆盖；`hitTest` O(n·depth) 且逐事件分配；`slices.Delete` 消费事件是 O(n)；文件混合 6 类关注点。

---

## io

| 包 | 关键类型 | 说明 |
|---|---|---|
| `io/event` | `Event`/`Filter`/`Tag`/`Op` | `event.Op` 写入 `TypeInput`，tag 必须是可比较且指针稳定的值 |
| `io/pointer` | `Event`/`Kind`/`Filter`/`Cursor`/`GrabCmd` | 指针事件；`Priority` 由 Router 派生 |
| `io/key` | `Event`/`Filter`/`FocusFilter`/`Range`/`EditEvent` | 键盘、IME 事件 |
| `io/system` | `Action`/`Locale`/`Decoration` | 窗口动作与区域设置 |
| `io/semantic` | `ClassOp`/`LabelOp`/`DescriptionOp` | 无障碍语义 |
| `io/transfer` | `SourceFilter`/`TargetFilter`/`DataEvent` | 拖放/剪贴板传输 |
| `io/clipboard` | `ReadCmd`/`WriteCmd` | 剪贴板命令 |

---

## internal/ops

**职责**：op 的二进制编码、Writer、Reader、栈校验、宏展开。

**关键点**

- `Ops.data` 是唯一存储；`refs` 保存指针/接口；`stringRefs` 避免字符串装箱。
- `Reader.Decode` 返回值类型 `EncodedOp`，通过 `Data` 与 `Refs` 暴露原始内容；没有 schema。
- `props` 表（[internal/ops/ops.go:399-434](../internal/ops/ops.go#L399-L434)）把 op 类型映射到长度/变长/是否含引用。
- 栈 ID 校验防止 `Push`/`Pop` 错配（panic）。

**问题**：0 测试；`Refs` 的类型断言分散在 `io/input` 与 `gpu`；op 格式无版本号；`Reset` 的 version 递增被 GPU 当作缓存 key。

---

## internal

| 包 | 职责 | 问题 |
|---|---|---|
| `internal/scene` | 路径命令的紧凑编码（Line/Quad/Cubic/Gap） | 无测试 |
| `internal/stroke` | 描边、贝塞尔细分、`SplitCubic` | 5% 覆盖；`inv` 未使用 |
| `internal/f32` | 浮点仿射/点运算（比公共 `f32` 更快） | 与 `f32` 重复，需保持同步 |
| `internal/f32color` | sRGB↔linear、预乘 | 28% 覆盖；有生成器 |
| `internal/fling` | 速度外推、衰减动画 | 42.5% 覆盖；用 `runtime.nanotime` |
| `internal/byteslice` | `[]byte` ↔ 任意切片的零拷贝视图 | 23 行，unsafe |
| `internal/debug` | `GIO_DEBUG` 等环境开关 | 无测试 |

---

## gpu/internal

| 包 | 行数 | 说明 |
|---|---:|---|
| `driver` | 384 | `Device` 即时模式接口、`Caps`、`Texture`/`Buffer`/`Pipeline` |
| `opengl` | 1,552 | GL/GLES；共享上下文状态保存/恢复；`glUniform*` 解包 |
| `vulkan` | 1,168 | Vulkan；descriptor set、push constants、barrier |
| `metal` | 1,194 | Metal（Darwin）；command buffer 串行 |
| `d3d11` | 878 | D3D11（Windows） |
| `rendertest` | 5 + 1,688 测试 | 跨后端渲染回归（headless） |

**驱动接口问题**：全局注册表、`OpenGLRenderTarget` 泄漏具体类型、~8 个未使用的 compute 方法、`PrepareTexture` 语义各后端不同、坐标约定藏在 shader 里。

---

## 平台 FFI

| 包 | 行数 | 说明 |
|---|---:|---|
| `internal/gl` | 3,177 | GL/GLES 手写绑定（unix/js/windows 三套） |
| `internal/vk` | 2,281 | Vulkan 绑定 + Wayland/X11 surface |
| `internal/d3d11` | 1,695 | D3D11/DXGI/COM 绑定 |
| `internal/egl` | 545 | EGL（unix/windows/ohos） |

全部 0 测试、unsafe 密集；建议用 `go generate` 从 XML 注册表生成（Vulkan 已有官方 vk.xml），减少手写错误。

---

## app/internal

| 包 | 行数 | 说明 |
|---|---:|---|
| `windows` | 1,019 | Win32 用户32/GDI/DPI 绑定 |
| `xkb` | 384 | XKB 键映射与 compose |
| `ibus` | 334 | D-Bus IBus 客户端（同步调用） |

---

## f32 / unit

- `f32.Affine2D` 是 3×2 仿射（内部 `a`/`e` 存 `sx-1`/`sy-1`），`Point` 是 `float32` 对；`internal/f32` 提供无分配版本。
- `unit.Metric` 负责 `Dp`/`Sp` → 像素，四舍五入；`unit.Dp` 是 `float32`。
- **缺陷**：`Shear` 的 Y 平移项误用 `a.f` 而非 `a.c`（[f32/affine.go:154](../f32/affine.go#L154)），先平移后错切会偏移；已在验证文档复现。
- 其余测试充分（测试比 1.59）。

---

## font

- `font.Font`（Typeface/Style/Weight）是"字体查询"，不是字体实例。
- `font/opentype` 把 `[]byte` 解析为 `Face`，每次 `Face()` 返回新的 `*font.Font`（导致 `text.Load` 去重失效）。
- `font/gofont` 嵌入 Go 字体，`Collection()` 会原地修改全局切片。

---

## 横切主题

| 主题 | 现状 | 建议 |
|---|---|---|
| 状态 | 用户持有控件状态；窗口持有 Router/GPU | 文档化"控件零值可用、不可跨 goroutine" |
| 缓存 | 6 类 LRU + GPU 纹理/路径缓存 | 统一 key 约定 + 字节预算 + 命中率指标 |
| 错误 | panic 为主；`error` 返回值稀少 | 在 `app`/`gpu` 边界引入 error + recover |
| 并发 | 仅 `Invalidate` 明确可并发；其余靠约定 | 明确 owner 线程，Router 加锁或改为事件队列 |
| 测试 | 内核/文本/布局好；平台/FFI/GPU 零 | 先补 `internal/ops`、`scene`、`gpu/drawOps` 的纯函数测试 |
| 文档 | 公共 API godoc 尚可；平台差异缺失 | 为 `driver` 写线程契约，为 `op` 写格式规范 |