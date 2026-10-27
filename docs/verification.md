# 验证与基准

> 本文记录分析过程中实际执行过的构建、测试、覆盖率、基准与缺陷复现，供复核。
> 环境：macOS（Apple M4 Max，arm64），Go 1.27.1，仓库 `7efc307f`（分支 `ohos`）。

---

## 1. 工具链问题（先读这里）

本机默认 `go` 是 **混合工具链**：

```
$ which go
/opt/homebrew/Cellar/go/lastest/libexec/bin/go
$ go version
go version go1.24.5 darwin/arm64      # 但 GOROOT 指向 go1.27.1
$ go build ./...
compile: version "go1.27.1" does not match go tool version "go1.24.5"
```

原因是 `/opt/homebrew/Cellar/go/lastest` → `1.27.1` 的 `GOROOT` 中，`pkg/tool` 是 1.27.1 的编译器，而 PATH 上的 `bin/go` 驱动是 1.24.5。**不是项目问题**，但会让所有构建/测试直接失败，容易误判。

**可用命令**（下文所有结果均由此得出）：

```bash
export PATH=/opt/homebrew/Cellar/go/1.27.1/libexec/bin:$PATH
go build ./...
go test ./...
```

> 建议在仓库文档或 CI 中固定 `GOTOOLCHAIN=go1.24.0`（或至少记录 go1.27.x 可用），避免贡献者踩同一个坑。

---

## 2. 构建与测试结果

```bash
go build ./...     # 通过，无警告
go test ./...      # 全部通过
go vet ./...       # 仅 unkeyed fields 警告（13 处，均在 internal/f32 与 app/internal/ibus）
staticcheck ./...  # 21 处，见 §5
```

### 覆盖率（`go test -cover ./...`，节选）

| 包 | 覆盖率 | 说明 |
|---|---:|---|
| `io/input` | 90.0% | 路由核心，测试最充分 |
| `unit` | 88.9% | |
| `layout` | 84.0% | |
| `text` | 76.6% | |
| `widget` | 75.8% | |
| `gpu/headless` | 75.3% | 渲染回归的主要抓手 |
| `op` | 46.0% | |
| `internal/fling` | 42.5% | |
| `op/clip` | 32.7% | |
| `internal/f32color` | 28.0% | |
| `widget/material` | 27.0% | 大量样式代码无测试 |
| `io/pointer` | 21.5% | 只是事件类型的工具函数 |
| `internal/stroke` | 5.0% | 关键几何代码几乎无测试 |
| `app` | 无覆盖数据 | 测试只覆盖 IME/OHOS pump 等纯函数；平台后端为 0 |
| `gpu/internal/{opengl,vulkan,metal,d3d11}` | 0% | 由 `gpu/internal/rendertest` 间接覆盖 |
| `internal/{ops,scene,gl,vk,d3d11,egl}` | 0% | |

测试/代码比：整体 0.24；`app` 0.08、`widget/material` 0.04、`gpu` 0.02。

---

## 3. 基准数据

命令：

```bash
go test -run XXX -bench 'BenchmarkLabel|BenchmarkEditor|BenchmarkRouterAdd|BenchmarkPacker|BenchmarkStack' \
        -benchmem -benchtime=200ms ./widget ./io/input ./gpu ./layout
```

### 3.1 文本：静态 vs 动态（同一 1000 runes）

| 场景 | 时间/op | 分配 | 解读 |
|---|---:|---:|---|
| `EditorStatic/1000runes-LTR-latin` | 21.2 µs | 1 alloc, 508 B | 缓存全命中，仅遍历 |
| `EditorDynamic/1000runes-LTR-latin` | 308 µs | 1,813 allocs, 365 KB | 每帧重建索引 + 重新整形 |
| `LabelDynamic/1000runes-LTR-latin` | 144 µs | 579 allocs, 312 KB | |
| `LabelDynamic/10runes-LTR-latin` | 13.2 µs | 113 allocs, 44 KB | 小文本也非零分配 |
| `EditorDynamic/1000runes-RTL-emoji` | 751 µs | 2,507 allocs, 458 KB | bidi/emoji 最差 |

**结论**：稳态（缓存命中）性能优秀；**动态路径（输入、宽度变化、滚动）比稳态慢 15–35 倍**，且分配与文本长度线性相关。这是 Gio 文本体验的主要瓶颈（对应风险 P-Text-2/P-Text-3）。

### 3.2 输入路由：重叠 handler 的扩展性

`BenchmarkRouterAdd`（n 个完全重叠的 handler，每次投递一个 `pointer.Move`）：

| handler 数 | ns/op | B/op | allocs/op | 相对 3 个 |
|---:|---:|---:|---:|---:|
| 3 | 617 | 1,912 | 12 | 1.0× |
| 9 | 1,706 | 3,424 | 22 | 2.8× |
| 27 | 8,235 | 5,800 | 42 | 13.3× |
| 81 | 55,547 | 18,801 | 100 | 90× |

**3 倍 handler 数带来约 4.8–6.7 倍耗时，明显超线性（≈O(n²)）**。根因：

- `hitTest` 对每个命中节点沿 `area.parent` 链逐级求逆变换（[io/input/pointer.go:550-599](../io/input/pointer.go#L550-L599)）；
- 每个命中都要 `addHandler`/`searchTag` 线性去重（[io/input/pointer.go:825-902](../io/input/pointer.go#L825-L902)）；
- 每个事件都会分配闭包与 `hits`/`taggedEvent` 切片。

对深层嵌套的卡片/列表 UI（几十个重叠区域）意味着**单次鼠标移动百微秒级**。

### 3.3 其它微基准

| 基准 | 结果 | 备注 |
|---|---:|---|
| `BenchmarkStack` | 90 ns, 0 alloc | 布局纯计算很轻 |
| `BenchmarkBackground` | 34.5 ns, 0 alloc | |
| `BenchmarkPacker` | 78 µs, 0 alloc | 图集装箱（每次调用） |
| `BenchmarkEditorIMEState/unchanged` | 2.8 ns, 0 alloc | IME 状态短路有效 |
| `BenchmarkEditorIMEState/invalidated` | 579 ns | |

---

## 4. 已复现的缺陷

### 4.1 `layoutKey` 遗漏 `DisableSpaceTrim`（文本缓存污染）

**位置**：[text/lru.go:152-164](../text/lru.go#L152-L164)（key 定义） vs [text/shaper.go:441-454](../text/shaper.go#L441-L454)（key 构造） vs [text/gotext.go:397](../text/gotext.go#L397)（使用该参数）。

**复现程序**（临时模块，`replace gioui.org => /Users/mahesh/AiWork/gio`）：

```go
sh := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Regular()))
// 第一次：DisableSpaceTrim=false
sh.LayoutString(text.Parameters{PxPerEm: fixed.I(16), MaxWidth: 60}, "hello world")
// 第二次：DisableSpaceTrim=true，期望行尾空格宽度保留
sh.LayoutString(text.Parameters{PxPerEm: fixed.I(16), MaxWidth: 60, DisableSpaceTrim: true}, "hello world")
```

**实测输出**（每个字形 `Advance`，行尾空格是关键）：

```
fresh, trim=true : [8:58 8:58 4:18 4:18 8:58 4:29 11:36 ...]   ← 行尾空格 = 29/64 px
fresh, trim=false: [8:58 8:58 4:18 4:18 8:58 0:00 11:36 ...]   ← 行尾空格 = 0
shared false→true: [8:58 ... 0:00 ...]                          ← 错误地复用了 false 的结果
```

**影响**：编辑器强制 `DisableSpaceTrim=true`（[widget/editor.go:639](../widget/editor.go#L639)），Label 默认 false，而 `material.Theme.Shaper` 被整个窗口共享。首次整形的顺序决定后续所有同文本同宽度的行尾空格宽度 → 光标/选区终点偏移 1 个空格宽度。

**修复**：在 `layoutKey` 增加 `disableSpaceTrim bool` 字段，并加参数两两组合的回归测试。

### 4.2 语义节点被子孙手势覆盖

**位置**：[io/input/pointer.go:631](../io/input/pointer.go#L631)。

```go
area.semantic.valid = area.semantic.content.gestures != 0
```

`semanticLabel` 等已经把它置为 `true`（[pointer.go:361-393](../io/input/pointer.go#L361-L393)），这一行在 `pointerQueue.Frame` 中无条件覆盖。**只声明 `Enter|Leave`（hover）但有 label 的区域会从语义树中消失**，屏幕阅读器读不到。修复：`valid = valid || gestures != 0`。

### 4.3 GPU：全屏清除会作废 opacity layer 区间

**位置**：[gpu/gpu.go:1109-1117](../gpu/gpu.go#L1109-L1117)（`d.imageOps = d.imageOps[:0]`）与 [gpu/gpu.go:865-873](../gpu/gpu.go#L865-L873)（`ops[l.opStart:l.opEnd]`）。

当"不透明度组"之后出现一个全屏不透明纯色填充时：

1. layer 已关闭，`opStart/opEnd` 记录的是旧下标；
2. `imageOps` 被清空；
3. `drawLayers` 仍按旧下标切片 → **slice bounds out of range panic**。

未在真实设备上触发，但按代码路径可达。修复：截断时同步丢弃/重定位越界的 layer，并在 `packLayers` 加断言。

### 4.4 死代码（staticcheck U1000）

```
app/os.go:212-243            windowRendezvous 整个类型 + 构造 + 常驻 goroutine 未被使用
app/ime.go:201,207,213,275,396,414   字段/函数未使用
gpu/clip.go:57               encodeVertex 未使用（与 gpu/path.go:117 重复实现）
widget/editor.go:356         condFilter 未使用
text/gotext.go:217           bidiParagraph 字段未使用
widget/index.go:277          atStartOfLine 未使用
app/window.go:827-833        actions 赋值后未使用（4 处）
internal/stroke/stroke.go:596 inv 赋值后未使用
app/internal/ibus/ibus.go:75 ok 赋值后未使用
```

---

### 4.5 `Affine2D.Shear` 的 Y 平移项错误

**位置**：[f32/affine.go:150-156](../f32/affine.go#L150-L156)，最后一项写作 `a.f*ty + a.f`，应为 `a.c*ty + a.f`。

**复现**：

```go
a := f32.AffineId().Offset(f32.Pt(10, 20))
s := a.Shear(f32.Point{}, 0, 0.4) // 先平移 (10,20)，再沿 Y 错切 0.4
fmt.Println(s.Transform(f32.Point{}))
```

**实测**：

```
origin -> (10,28.455864)
want    -> (10,24)        // y' = 0.4*10 + 20 + 0
```

纯错切（`c=f=0`）时错误不显现，因此长期未被发现。

### 4.6 `ops.Reset` 不清 `multipOp`，一次未 `End` 的 `clip.Path` 永久毒化 `Ops`

**位置**：[internal/ops/ops.go:172-187](../internal/ops/ops.go#L172-L187)（Reset 未清 `multipOp`）；[internal/ops/ops.go:189-193](../internal/ops/ops.go#L189-L193)（`Write` 检查）。

**复现**：

```go
var ops op.Ops
p := clip.Path{}
p.Begin(&ops) // 故意不 End
ops.Reset()   // 开始"新的一帧"
paint.ColorOp{Color: color.NRGBA{A: 255}}.Add(&ops) // panic
```

**实测**：

```
panic after Reset: cannot mix multi ops with single ones
```

**影响**：一次编码错误（漏掉 `End`）会让该 `Ops` 之后每一帧都 panic，且无法通过 `Reset` 恢复。修复只需一行 `o.multipOp = false`。

### 4.7 代码确证但未构造运行时复现的缺陷

| 缺陷 | 证据 | 结论 |
|---|---|---|
| 未 `Stop` 的 `Record` 静默吞掉后续 op | [internal/ops/reader.go:150-160](../internal/ops/reader.go#L150-L160) | 解码器把未完成宏视为"剩余全部"，后续 op 被跳过 |
| 陈旧 `CallOp` 无代次校验 | [op/op.go:173-176](../op/op.go#L173-L176)、[internal/ops/ops.go:238-246](../internal/ops/ops.go#L238-L246) | 文档与实现不符；可产生零长度 `EncodedOp` |
| `Stack` Expanded 收到 `Min > Max` | [layout/stack.go:87](../layout/stack.go#L87) vs 100 | 违反 `Constraints` 不变量 |
| 语义 `valid` 被手势覆盖 | [io/input/pointer.go:631](../io/input/pointer.go#L631) | hover-only 区域的 label 从语义树消失 |
| GPU 全屏清除作废 layer 区间 | [gpu/gpu.go:1109-1117](../gpu/gpu.go#L1109-L1117) vs 865-873 | 可达 slice bounds panic |
| Vulkan `MaxTextureSize: 4096` | `gpu/internal/vulkan/vulkan.go:249-253` | 大窗口圆角 clip 触发图集 panic |
| Destroy 后 timer 无法重建 | [app/window.go:367-394](../app/window.go#L367-L394) | 关闭后再次 `Event()` 会阻塞 |
| Wayland 剪贴板 C 字符串泄漏 | `app/os_wayland.go:308` | 每次写入泄漏 MIME 字符串 |

---

## 5. staticcheck 结果（全量）

```
app/ime.go:201  field sentValid is unused (U1000)
app/ime.go:207  field external is unused (U1000)
app/ime.go:213  func (*textInputState).reset is unused
app/ime.go:275  func (*textInputState).clearPreedit is unused
app/ime.go:396  func requestSurroundingText is unused
app/ime.go:414  func prepareSurroundingText is unused
app/internal/ibus/ibus.go:75   value of ok is never used (SA4006)
app/os.go:212  type windowRendezvous is unused
app/os.go:218  type windowAndConfig is unused
app/os.go:223  func newWindowRendezvous is unused
app/window.go:827,829,831,833  value of actions is never used (SA4006)
gpu/clip.go:57  func encodeVertex is unused
gpu/path.go:117 func vertex.encode is unused
internal/stroke/stroke.go:596 value of inv is never used
text/gotext.go:217  field bidiParagraph is unused
widget/editor.go:356 func condFilter is unused
widget/editor.go:369,370,372 value of atBeginning/atEnd is never used
widget/index.go:277 func (*glyphIndex).atStartOfLine is unused
```

注意：`app/window.go:827-833` 的 `actions |= ...` 后未使用，说明 `decorate` 里对窗口模式的 `switch` **计算出的动作被丢弃**——这可能是逻辑缺陷而非纯冗余（见风险 A-App-3）。

---

## 6. 局限与未验证项

- **真实 GPU 后端**：所有 GPU 结论来自源码阅读（`gpu/internal/rendertest` 可在本机运行，但未对每个后端单独压测）；draw call 数量、Vulkan barrier 开销等属于静态推断，需 `apitrace`/GPU 计数器确认。
- **平台后端**：Windows/Android/OHOS 未在真机运行；相关线程模型结论来自代码路径（`newWindow` → `go func` → `ProcessEvent`）。
- **文本微基准**：动态路径数据来自 `widget` 自带 benchmark；`/tmp` 下的额外 harness（子代理使用）未纳入仓库。
- **未做**：完整 `-race` 跑测试套件（建议补做，见风险 A1"输入路由并发"）。

---

## 7. 建议的持续验证

1. CI 增加 `go vet` + `staticcheck`（当前 21 条可清零）。
2. 增加 `go test -race ./io/input ./widget ./app`。
3. 把 `docs/` 中的复现用例（4.1/4.2/4.3）落成回归测试。
4. 为 `gpu/internal/rendertest` 增加 draw-call 计数与 path-cache 命中率指标。
