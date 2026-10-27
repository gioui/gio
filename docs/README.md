# Gio 项目分析文档

> 仓库：`/Users/mahesh/AiWork/gio`（`gioui.org`，分支 `ohos`，HEAD `7efc307f`）
> 分析时间：2026-10-27 · 方法：全量源码通读 + `go build/test/vet` + `staticcheck` + 定向基准 + 缺陷复现

---

## 文档索引

| 文档 | 内容 | 适合谁 |
|---|---|---|
| [architecture.md](architecture.md) | 分层、核心抽象、一帧/一次事件的完整流程、并发模型、平台矩阵、fork 增量 | 新加入的开发者、架构评审 |
| [module-reference.md](module-reference.md) | 50 个包逐个说明：职责、关键类型、依赖、问题 | 要改某个模块的人 |
| [risks-and-optimizations.md](risks-and-optimizations.md) | 架构风险、性能瓶颈、可读性问题、路线图、目标架构、验收指标 | 技术负责人、性能优化 |
| [verification.md](verification.md) | 构建/测试/覆盖率、基准数据、已复现缺陷、staticcheck 清单、复现命令 | 想复核结论的人 |
| [ohos-backend.md](ohos-backend.md) | 本 fork 的 OHOS 后端专项：架构、功能矩阵、风险、未提交改动 | 维护 OHOS 分支的人 |
| [ohos-remediation-plan.md](ohos-remediation-plan.md) | OHOS 能力补齐方案：ABI v3、生命周期/帧/IME/窗口/文件/无障碍，含工作包与排期 | OHOS 维护者、shim 开发者 |
| [ohos-stage-plan.md](ohos-stage-plan.md) | OHOS 阶段执行计划：S0–S11、Gate、PR 切片、日期、证据与停止线 | 项目执行者、QA |

**推荐阅读顺序**：本页 → `architecture.md` → `risks-and-optimizations.md` → 按需查 `module-reference.md` / `ohos-backend.md` → 若维护 OHOS，继续读 `ohos-remediation-plan.md` → `ohos-stage-plan.md` → `verification.md` 复核。

---

## 一页速览

### 这是什么

Gio 是一个 **Go 即时模式 GUI 框架**：每帧把 UI 重新"编译"成一段二进制 op 流，`gpu` 读它渲染、`io/input` 读它做命中测试与语义树。同一份数据、两个消费者、零保留控件树。

- 50 个包，51k 非测试 LOC，测试/代码比 0.24。
- 8 个平台后端（Wayland/X11/Windows/macOS/Android/iOS/JS/OHOS）+ 4 个 GPU 后端（GL/Vulkan/Metal/D3D11）。
- 相对上游新增约 2.4k 行：OHOS 后端、macOS 捏合、文件拖放、4/5 号鼠标键。

### 架构一句话

```
widget/material ─▶ widget ─▶ layout / text / gesture ─▶ op / io ─▶ internal/ops
                                                                    ▲        ▲
                                        app ─▶ gpu ─────────────────┘        │
                                         └──────────────▶ io/input ──────────┘
```

### 最值得先做的 9 件事

| # | 事项 | 类型 | 工作量 | 收益 |
|---|---|---|---|---|
| 1 | `layoutKey` 补 `DisableSpaceTrim` | 正确性 | 0.5 天 | 修编辑器/标签共享 Shaper 的光标偏移（**已复现**） |
| 2 | 语义 `valid` 不要被手势覆盖 | 无障碍 | 0.5 天 | hover-only 区域的 label 恢复 |
| 3 | GPU 全屏清除时丢弃越界 layer | 崩溃 | 1 天 | 消除可达 panic |
| 4 | 文本行偏移增量计算 + 去掉 viewport 失效 | 性能 | 2–3 天 | 长文档输入从 O(N) 到 O(段落) |
| 5 | 路径缓存 key 去掉 `Ops.version` | 性能 | 3–5 天 | 圆角/描边 clip 只 tessellate 一次 |
| 6 | 输入命中索引（预计算逆变换/包围盒） | 性能 | 1 周 | 重叠区域从 110µs 降到微秒级 |
| 7 | `driver` 生命周期收口 | 稳定性 | 2–3 天 | 关闭/重开窗口不挂起、不竞态 |
| 8 | `Affine2D.Shear` 的 Y 平移项 | 正确性 | 0.5 天 | 公开 API 错切不再偏移（**已复现**） |
| 9 | `ops.Reset` 清 `multipOp` + 帧末校验 | 可用性 | 1 天 | 一次 `clip.Path` 漏 `End` 不再永久 panic（**已复现**） |

### 最重要的 6 条结论

1. **内核设计优秀，不要重写。** op 流 + 双消费者 + 零分配热路径是核心竞争力；问题集中在边界（平台、模块、缓存、错误）。
2. **`app` 是上帝包且反向依赖 `widget/material`。** 任何窗口程序都被拖入整套 Material 主题/图标/字体（173 个依赖包）。
3. **GPU 的主要性能债是提交粒度**：每个绘制 op 一次 draw call，路径几何每帧重建。文本密集界面可达数千 draw call。
4. **文本的稳态很快，动态路径慢 15–35 倍**：1000 runes 静态 21µs / 1 alloc，动态 308µs / 1,813 allocs；输入延迟随文档长度线性增长。
5. **输入路由在重叠区域超线性**：3→81 个重叠 handler，单事件 0.6µs → 55.5µs（≈90×），根因是逐节点逆变换与线性去重。
6. **不变量缺少强制校验**：未 `Stop` 的 `Record` 会静默丢帧；漏 `End` 的 `clip.Path` 会让 `Ops` 永久 panic；陈旧 `CallOp` 无代次校验。这类问题应通过"帧末断言 + 代次字段"消灭。

### 立刻可做的低风险清理

- `staticcheck` 21 条（死代码、无效赋值、重复实现），其中 `app/os.go` 的 `windowRendezvous` 整段未使用。
- OHOS 工作区未提交代码里每个 touch/axis 事件都 `Sprintf`+写 stderr——提交前必须删。
- `layout/flex.go` 与 `stack.go` 的 `scratch = append(scratch, make(...)...)` 可改为直接切片。
- `layout.Spacing.String()` 把 `SpaceBetween` 返回成 `"SpaceAround"`。
- `f32.Affine2D.Shear` 的 Y 平移项把 `a.c` 写成 `a.f`。
- `ops.Reset` 补 `multipOp = false`；`Record` 未 `Stop` 时在帧末报错而不是静默吞掉后续 op。

---

## 关键数据

### 覆盖率（节选）

| 高 | | 低 | |
|---|---:|---|---:|
| `io/input` | 90% | `widget/material` | 27% |
| `unit` | 89% | `io/pointer` | 21.5% |
| `layout` | 84% | `internal/stroke` | 5% |
| `text` | 77% | `gpu/internal/*` | 0% |
| `widget` | 76% | `internal/ops` | 0% |
| `gpu/headless` | 75% | `app` 平台后端 | 0% |

### 基准（M4 Max）

| 场景 | 时间/op | 分配 |
|---|---:|---:|
| `EditorStatic/1000` | 21 µs | 1 |
| `EditorDynamic/1000` | 308 µs | 1,813 |
| `EditorDynamic/1000`（RTL+emoji） | 751 µs | 2,507 |
| `RouterAdd/3` | 617 ns | 12 |
| `RouterAdd/81` | 55.5 µs | 100 |
| `Stack` | 90 ns | 0 |

---

## 复核方式

```bash
# 本机默认 go 是混合工具链（1.24.5 驱动 + 1.27.1 标准库），必须显式指定：
export PATH=/opt/homebrew/Cellar/go/1.27.1/libexec/bin:$PATH

go build ./...
go test ./...
go test -cover ./...
go vet ./...
staticcheck ./...

# 基准
go test -run XXX -bench 'BenchmarkLabel|BenchmarkEditor|BenchmarkRouterAdd' \
        -benchmem -benchtime=200ms ./widget ./io/input ./gpu ./layout
```

缺陷复现代码与输出见 [verification.md §4](verification.md#4-已复现的缺陷)。

---

## 方法与局限

**做了什么**

- 通读 50 个包的非测试源码（重点：`app`、`gpu`、`io/input`、`text`、`widget`、`op`、`layout`）。
- 运行构建、全部测试、覆盖率、`go vet`、`staticcheck`、5 组基准。
- 复现 2 个缺陷（`layoutKey` 缓存污染、文本行偏移），逐行确证 6 个（语义覆盖、GPU layer 越界、Vulkan 4096、timer 重初始化、Wayland C 字符串泄漏、OHOS 生命周期）。
- 对比上游 `origin/main` 的 fork 增量。

**没做什么 / 局限**

- 未在 Windows/Android/OHOS 真机运行；平台相关结论来自代码路径。
- GPU 结论来自源码与 `rendertest`，未做 GPU profiler / draw-call 抓取。
- 未跑完整 `-race`（建议补做，尤其是 `io/input` 与 `app`）。
- 未做二进制体积测量（`app` 依赖闭包 173 包为 `go list` 结果）。

**证据等级标注**：已复现 / 代码确证 / 静态推断，详见各文档条目。

---

## 后续建议（如果要继续深入）

1. 用 `-race` 跑 `io/input` + `app`，确认 A5 的数据竞争范围。
2. 在真机抓一次 OHOS 触摸 → 帧的完整时序，确认 P1/P3 的实际影响。
3. 用 `apitrace`/Xcode GPU 工具统计真实 draw call 数，验证 P1 的量级。
4. 为 `app` 增加 `BenchmarkEventLoopFrame`，为 `gpu` 增加 draw-call 计数与 cache 命中率。
5. 把本次的 2 个复现用例与 6 个代码确证问题落成回归测试。