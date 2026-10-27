# 跨平台触控板与手势优化提升方案

> 依据：[input-gestures-cross-platform.md](input-gestures-cross-platform.md) 的矩阵、源码复核与平台 API 结论。
> 状态：方案文档，暂未修改源码。
> 目标：把 Gio 从“鼠标/触摸 + Scroll”升级为“可区分设备、可拿语义手势、可回退识别”的输入模型，并修正 Windows/macOS/Wayland/OHOS 的具体缺口。
> 实施进展（2026-10-27）：事件模型与语义手势路由、macOS pinch/rotate/swipe、OHOS axis pinch、Windows PT_* 分类/触控板注册已落地；Wayland、Windows pinch/rotate、gesture 回退包、Android/iOS/Web 仍待做。

---

## 0. 结论摘要

### 必须先解决的 P0

| # | 问题 | 影响 | 处理 |
|---|---|---|---|
| P0-1 | io/pointer 没有 Trackpad/Pen 来源，没有 Pinch/Rotate/Swipe/Hold | 应用无法正确实现缩放/旋转/多指手势 | 扩展事件模型 |
| P0-2 | macOS pinch 被降级为 Ctrl+Scroll | 语义错误；无法拿 scale/旋转 | 直接映射 NSEvent 手势 |
| P0-3 | Wayland 未绑定 pointer-gestures，忽略 axis_source | Linux 触控板手势完全缺失 | 绑定协议 |
| P0-4 | Windows PT_TOUCHPAD 被归为 Touch，且未注册 Precision Touchpad | Windows 触控板原始帧/手势不可用 | 修正分类 + 注册 |
| P0-5 | OHOS axis pinch 只合成 Scroll | 语义丢失；无法与 Ctrl+滚轮区分 | 输出 Pinch 语义事件 |

### 四个主战场

1. **事件模型**：Source、Kind、Phase、Scale、Rotation、Fingers、GestureID。
2. **桌面语义手势**：Windows Precision Touchpad、macOS NSEvent、Wayland pointer-gestures。
3. **移动/回退识别**：Android/iOS/Web/X11 的原始多点 + gesture 包识别器。
4. **仲裁与测试**：系统保留手势、能力查询、事件轨迹回放、真机矩阵。

### 建议节奏

| 里程碑 | 范围 | 工期（单人） | 退出标准 |
|---|---|---|---|
| M1 语义手势 | 事件模型 + Windows/macOS/Wayland/OHOS 的 pinch | 2 周 | 四个平台 pinch 可用；旧应用不受影响 |
| M2 完整手势 | rotate/swipe/hold + OHOS 全量 + 仲裁 | 2 周 | 桌面与 OHOS 手势矩阵通过 |
| M3 移动/Web | Android/iOS/Web/X11 回退识别器、Pointer Events | 2.5 周 | 移动端 pinch/rotate/swipe 可用 |
| M4 长尾 | Pen、压力/倾斜、原始触控板帧、能力查询 | 1–2 周 | 按需交付 |

### 关键决策（ADR）

| 决策 | 选择 | 理由 | 被否方案 |
|---|---|---|---|
| 手势如何进入事件系统 | 扩展 pointer.Kind + pointer.Event 字段 | 复用现有 tag/filter/router；应用已接收 pointer.Event；Filter 天然 opt-in | 新增独立 GestureEvent（需第二套路由/过滤） |
| Pinch scale 语义 | 相对 begin 的绝对比例 | 与 Wayland/libinput 一致；避免应用累积误差 | 每次 update 的增量 |
| Rotate 单位 | 弧度，相对 begin 的累计角 | 跨平台统一；方向可测 | 平台角度/增量混用 |
| 来源 | 显式 SourceTrackpad/SourcePen | 应用可区分设备；避免猜 | 继续挤进 Mouse/Touch |
| 原生手势 vs 应用识别 | 优先原生语义；无原生时用 gesture 包回退 | 体验与一致性最佳 | 所有平台都在 Go 里识别 |
| macOS 行为 | 只输出语义手势，不再保留 Ctrl+Scroll 旧路径 | 本次允许破坏性升级；避免双语义 | 过渡开关 |
| 系统手势 | 永不抢占；只处理未消费事件 | 避免返回/Home/多任务失效 | 应用强行 hook 全局 |

### 不建议的做法

- 不要继续用 Ctrl+Scroll 编码 pinch；这会永久丢失语义。
- 不要在 pointerQueue 里做手势识别；识别属于后端或 gesture 包。
- 不要在每个事件上起 goroutine 或分配新对象；输入热路径必须 0 alloc。
- 不要同时启用原生语义手势和回退识别器，避免双份事件。
- 不要试图覆盖 Android 返回手势、iOS Home indicator、Windows 三指系统手势。

---

## 1. 目标、非目标与验收口径

### 1.1 目标

- **可区分**：Mouse、Touch、Trackpad、Pen 四类来源可区分。
- **可语义**：Pinch/Rotate/Swipe/Hold 有完整 begin/update/end/cancel、scale、rotation、手指数。
- **可回退**：没有原生语义手势的平台，可用 gesture 包从原始多点识别。
- **可仲裁**：应用能 opt-in；系统保留手势不被破坏。
- **可测试**：router、gesture 状态机、后端映射都有 host 测试；真机有清单。
- **可迁移**：旧应用不 opt-in 时行为不变；macOS 旧 Ctrl+Scroll 有过渡开关。

### 1.2 非目标

| 项 | 处理 |
|---|---|
| 原始触控板帧的完整设备模型（压力/面积/温度） | M4 评估；先不做 |
| 手写笔压力/倾斜/橡皮擦 | M4 单独设计 |
| 触觉反馈/Force Touch | 平台特定，暂不做 |
| 全局系统手势接管 | 明确不做 |
| Web 原始触控板 | 浏览器不提供，不做 |
| 多窗口手势路由 | 跟随现有窗口模型，不单独设计 |

### 1.3 验收口径

| 场景 | 通过标准 |
|---|---|
| 旧应用 | 不请求新 Kind 时，不收到 Pinch/Rotate/Swipe/Hold；Press/Move/Scroll 不变 |
| macOS pinch | 双指捏合产生 Pinch，scale 相对 begin；不再产生 Ctrl+Scroll（默认） |
| Windows 触控板 | PT_TOUCHPAD 产生 SourceTrackpad；双指 pinch/rotate 可用；触摸屏仍为 Touch |
| Wayland | 支持 pointer-gestures 的合成器下 pinch/rotate/swipe/hold 可用；axis_source 可区分 |
| OHOS | axis pinch 产生 Pinch；rotate/swipe/hold 按能力输出；触摸屏不误触 |
| Android/iOS | 原始多点可被 gesture.Pinch/Rotate/Swipe 识别；系统手势仍可用 |
| Web | Pointer Events 优先；ctrl+wheel 仅回退；touch-action 不被破坏 |
| 性能 | 每个输入事件 0 alloc；手势更新不触发额外帧 |
| 冲突 | 双指/三指系统手势不被应用抢占；有明确 opt-in |

---

## 2. 源码复核：方案必须覆盖的具体问题

### 2.1 Windows

- pointerUpdate 只把 PT_MOUSE 归为 Mouse，其他（含 PT_TOUCHPAD、PT_PEN、PT_TOUCH）全部归为 Touch。
- 没有 RegisterTouchpadCapableWindow；Precision Touchpad 不进入 WM_POINTER 触控板帧。
- 只有 WM_POINTERWHEEL；没有 WM_GESTURE、InteractionContext、TouchpadGesturesController。
- 结论：先修来源分类与注册，再决定用 InteractionContext 还是 WM_GESTURE 产出语义。

### 2.2 macOS

- gio_onMagnify 把 magnification 转成 Scroll，并强制加 Ctrl。
- rotateWithEvent/swipeWithEvent 未接。
- scrollWheel 没有使用 phase/momentumPhase 做 begin/end。
- 结论：删掉 Ctrl 编码；用 NSEventPhase 映射 Phase；rotate/swipe 直接输出。

### 2.3 Wayland/X11

- Wayland 只绑 wl_pointer/wl_touch；未绑 zwp_pointer_gestures_v1。
- gio_onPointerAxisSource 只记录/忽略，未进入 Event。
- X11 只有核心按钮滚动，无 XI2 触摸。
- 结论：协议优先；X11 用 gesture 包回退，不承诺桌面环境手势。

### 2.4 OHOS

- axis pinch 被换算成 Scroll；触摸后 150ms 抑制 axis pinch。
- 没有 rotate/swipe/hold；没有 trackpad 来源。
- Native XComponent 拿不到 ArkUI 高层 GestureRecognizer。
- 结论：axis 直接输出 Pinch；rotate 可由双指角度合成；swipe/hold 由 touch 合成；新增可选导出，不改 v4 结构。

### 2.5 路由层

- pointerQueue 目前只对 Scroll 做“无 PointerID、按位置分发、多个 handler”的特殊处理。
- 新 Kind 如果不显式处理，会落入 default/pointerOf，可能产生 enter/leave 或只投递给第一个 handler。
- 结论：为 Pinch/Rotate/Swipe/Hold 增加“语义手势”分支，语义与 Scroll 类似但不做滚动 clamp。

### 2.6 上层

- gesture 包只有 Click/Drag/Scroll/Hover；没有 pinch/rotate/swipe/hold。
- 没有能力查询；应用无法知道某平台是否原生支持。
- 结论：增加回退识别器与最小能力查询。

---

## 3. 设计原则

1. **语义优先**：有原生语义手势就用，不要退化到 Ctrl+Scroll。
2. **opt-in**：新 Kind 只有显式 Filter 才收到；旧应用零行为变化。
3. **相位完整**：begin/update/end/cancel 必须成对；取消要能回滚。
4. **单位统一**：scale 绝对、rotation 弧度、swipe 位移。
5. **来源显式**：Trackpad/Pen 不猜；不能区分时保持 Mouse/Touch。
6. **不抢系统**：系统保留手势优先；应用只处理未消费事件。
7. **热路径 0 alloc**：事件值传递；状态机复用；不做每事件 goroutine。
8. **可回放**：后端事件轨迹可序列化，host 测试用 golden trace。

---

## 4. 目标事件模型

### 4.1 扩展 pointer.Event

~~~go
// io/pointer/pointer.go
type Source uint8
const (
    Mouse Source = iota
    Touch
    Trackpad
    Pen
)

type Kind uint
const (
    Cancel Kind = 1 << iota
    Press
    Release
    Move
    Drag
    Enter
    Leave
    Scroll
    Pinch
    Rotate
    Swipe
    Hold
)

type GesturePhase uint8
const (
    GestureBegin GesturePhase = iota
    GestureUpdate
    GestureEnd
    GestureCancel
)

type Event struct {
    // existing fields...
    Kind      Kind
    Source    Source
    PointerID ID
    Position  f32.Point
    Scroll    f32.Point
    Buttons   Buttons
    Modifiers key.Modifiers
    Time      time.Duration

    // gesture fields; valid only for Pinch/Rotate/Swipe/Hold
    GestureID uint32
    Phase     GesturePhase
    Scale     float32   // pinch: absolute relative to begin, 1.0 = no change
    Rotation  float32   // radians, cumulative relative to begin
    Swipe     f32.Point // swipe: logical center delta relative to begin
    Fingers   uint8
}
~~~

说明：

- Pinch：Position 为手势逻辑中心；Scale 相对 begin；Fingers 为手指数。
- Rotate：Rotation 为相对 begin 的累计弧度；正方向统一为屏幕坐标顺时针（后端换算）。
- Swipe：新增 **Swipe f32.Point** 表示相对 begin 的逻辑中心位移；不要复用 Scroll，避免与滚动语义混淆。
- Hold：Position 为中心；Phase begin/end/cancel；无位移。
- GestureID 用于关联同一次手势的 begin/update/end；不同后端可自增。

最终建议字段：

~~~go
GestureID uint32
Phase     GesturePhase
Scale     float32
Rotation  float32
Swipe     f32.Point
Fingers   uint8
~~~

### 4.2 路由语义

在 io/input/pointer.go 增加语义手势分支：

- Pinch/Rotate/Swipe/Hold 不做 pointerOf；不产生 Press/Release/Enter/Leave。
- 按 Position 命中当前区域；投递给所有 Matches 的 handler（与 Scroll 类似，不 break）。
- 不做 ScrollRange clamp；Scroll 仍只用于滚动。
- Phase=GestureCancel 时，向所有正在跟踪该 GestureID 的 handler 投递 Cancel，并清理状态。
- Filter.Kinds 直接复用；旧 Filter 不含新 bit，因此不会收到。

### 4.3 能力查询

最小 API（可选，M2 再加）：

~~~go
// package pointer
type Capabilities struct {
    Trackpad bool
    Pinch    bool
    Rotate   bool
    Swipe    bool
    Hold     bool
    Pen      bool
}
func Capabilities() Capabilities
~~~

实现可由后端注册；不追求完整设备枚举。

---

## 5. 工作包

### WP0. 安全网（0.5 天）

- 新增 pointer.Event 轨迹序列化/回放测试工具。
- 建立 baseline：现有 Press/Move/Scroll 行为 golden。
- 所有新 Kind 默认不投递；加测试防止旧 Filter 收到。
- 验收：旧测试全绿；新 Kind 未 opt-in 零事件。

### WP1. 核心事件模型与路由（5 天）

文件：io/pointer/pointer.go、io/input/pointer.go、io/input/pointer_test.go、gesture/*。

任务：

1. 增加 SourceTrackpad/SourcePen。
2. 增加 Kind Pinch/Rotate/Swipe/Hold、GesturePhase、Event 字段。
3. 路由层增加语义手势分支；取消/结束清理。
4. 增加 Pinch/Rotate/Swipe/Hold 的 Filter/Matches 测试。
5. 增加最小 gesture 包状态机（可先 Pinch/Rotate，后 Swipe/Hold）。
6. 性能：事件路径 0 alloc；加 benchmark。

验收：

- 单测覆盖 begin/update/end/cancel、多 handler、重叠区域。
- Scroll 行为不变。
- 旧 Filter 不收到新 Kind。

### WP2. Windows（4 天）

文件：app/os_windows.go，必要时 internal/windows。

任务：

1. 启动时 RegisterTouchpadCapableWindow。
2. pointerUpdate 按 PointerType 分类：PT_TOUCHPAD → Trackpad，PT_PEN → Pen，PT_TOUCH → Touch，PT_MOUSE → Mouse。
3. 触控板双指：接 WM_POINTER 帧，用 InteractionContext 产出 Pinch/Rotate/Pan；或先解析触点距离/角度输出 Pinch/Rotate。
4. 触摸屏：接 WM_GESTURE 的 zoom/rotate/pan → 语义事件（可选，M2）。
5. 三指全局手势：TouchpadGesturesController 仅在前台且显式 opt-in；默认不抢。
6. 滚动：保留 WM_POINTERWHEEL；触控板双指平移与 Scroll 区分。

验收：

- Surface/Precision Touchpad/触摸屏/鼠标四类来源正确。
- 双指 pinch/rotate 在 Windows 11 实机通过；系统三指手势不被破坏。
- 旧鼠标/滚轮行为不变。

### WP3. macOS（3 天）

文件：app/os_macos.go、app/os_macos.m。

任务：

1. magnifyWithEvent → Pinch，使用 magnification 累积/相对 begin scale。
2. rotateWithEvent → Rotate，rotation 转弧度并累计。
3. swipeWithEvent → Swipe，delta 转 Swipe 位移；三指。
4. scrollWheel 使用 phase/momentumPhase 映射 begin/update/end/cancel；双指滚动保持 Scroll。
5. 删除 gio_onMagnify 的 Ctrl 修饰和旧 Scroll 合成路径；只输出语义 Pinch。
6. 原始 NSEvent touches 可选（M4）。

验收：

- MacBook 触控板 pinch/rotate/swipe 正确；无 Ctrl+Scroll 双份。
- 系统三指拖移/切换桌面不被破坏。
- 旧 Ctrl+Scroll 开关可用。

### WP4. Wayland / X11（4 天）

文件：app/os_wayland.go、app/os_wayland.c、app/os_x11.go。

任务：

1. 绑定 zwp_pointer_gestures_v1（swipe/pinch/hold）。
2. pinch 的 scale/rotation/dx/dy → Pinch/Rotate；swipe → Swipe；hold → Hold。
3. 保留 axis_source：finger → Trackpad，wheel → Mouse；写入 Event.Source。
4. 合成器不支持时静默降级，不报错。
5. X11 不承诺原生手势；用 gesture 包从 XI2 touch（若后续接）或触摸屏回退；当前至少不误报。

验收：

- GNOME/KDE Wayland 下 pinch/rotate/swipe/hold 按合成器能力出现。
- 触控板双指滚动仍为 Scroll；滚轮来源为 Mouse。
- X11 无回归。

### WP5. OHOS（5 天 Go + 4 天 shim）

文件：app/ohos_pump.go、app/ohos_abi.go、app/ohos_glue.h、shim（仓库外）。

任务：

1. axis pinch 输出 Pinch（Scale 相对 begin），不再合成 Scroll；保留触摸 150ms 抑制。
2. 双指旋转：从 axis/touch 角度变化计算 Rotate（若 shim 能提供角度则直接用）。
3. Swipe/Hold：由 touch 合成或由 ArkUI 手势事件转发；XComponent 没有高层手势，走合成。
4. 新增导出 gio_ohos_gesture_w(windowID, kind, phase, scale, rotation, dx, dy, fingers, gestureID)；不修改 ABI v4 结构，作为加法能力。
5. shim 侧必须同步升级：有 ArkUI Gesture 能力时转发；否则由 touch/axis 计算。不保留旧 shim 的 Scroll 回退。
6. 文档更新 ohos-shim.md。

验收：

- 真机/2in1 pinch 输出 Pinch；单指触摸不误触。
- rotate/swipe/hold 至少一个设备可用；不可用时不产生假事件。
- 旧 shim 无新导出时行为不变（仅 Scroll 回退可选）。

### WP6. Android / iOS（5 天）

文件：app/os_android.go + Java、app/os_ios.go + ObjC、gesture 包。

任务：

1. Android：保留原始多点；在 gesture 包实现 Pinch/Rotate/Swipe/Hold；可选接 ScaleGestureDetector（JNI 回调）减少重复。
2. iOS：在 GioView 上挂 UIPinchGestureRecognizer/UIRotationGestureRecognizer/UISwipeGestureRecognizer，输出语义事件；设置 cancelsTouchesInView=false 并处理 simultaneous recognition，确保 Gio 仍能拿到原始 UITouch。
3. iPadOS：接 UIHoverGestureRecognizer/UIPointerInteraction（至少 hover）。
4. 处理系统边缘手势：不覆盖返回/Home；必要时提供 defer 选项。

验收：

- Android 手机 pinch/rotate 可用；系统返回手势仍可用。
- iPad + 触控板 pinch/rotate 可用；三指系统手势不被抢。
- iPhone 触摸屏行为不变。

### WP7. Web（2 天）

文件：app/os_js.go。

任务：

1. Pointer Events 优先；Touch Events 回退；去重。
2. wheel ctrlKey 作为 pinch 回退；Safari GestureEvent 可选。
3. touch-action 不主动改；应用可配置。
4. 不尝试拦截浏览器缩放/返回。

验收：

- Chrome/Safari 触控板 pinch 至少二选一可用；页面缩放/返回不被破坏。
- 鼠标滚轮仍为 Scroll。

### WP8. 仲裁、能力与兼容（3 天）

- pointer.Capabilities 最小实现。
- macOS 旧 Ctrl+Scroll 开关。
- Windows 全局手势 opt-in 开关。
- 文档：系统手势保留清单、应用注意事项。
- 旧 Filter 不收新 Kind 的回归测试。

### WP9. 测试、CI、文档（3 天，贯穿）

- Host：router/gesture/backend fixture；0 alloc benchmark。
- CI：go test ./...，新包测试；无设备平台用 fake 事件。
- 真机矩阵：Windows Precision Touchpad、MacBook、GNOME/KDE Wayland、Android、iPad、OHOS、Chrome/Safari。
- 文档：input-gestures-cross-platform.md 更新为“实现状态”；新增本方案执行记录。

---

## 6. 里程碑与依赖

| 里程碑 | 工作包 | 工期 | 依赖 | 退出标准 |
|---|---|---|---|---|
| M1 语义骨架 | WP0 + WP1 + WP2(pinch) + WP3(pinch) + WP4(pinch) + WP5(pinch) | 2 周 | 无 | 四平台 pinch 可用；旧行为不变 |
| M2 完整手势 | WP2(rotate/swipe) + WP3(rotate/swipe) + WP4(全) + WP5(全) + WP8 | 2 周 | M1 | 桌面/OHOS 手势矩阵通过 |
| M3 移动/Web | WP6 + WP7 + WP9 部分 | 2.5 周 | M1 | Android/iOS/Web 回退可用 |
| M4 长尾 | Pen/压力/原始帧/能力增强 | 1–2 周 | M2/M3 | 按需交付 |

关键路径：WP1 → WP2/3/4/5 的 pinch；WP1 不完成，其他平台无法统一事件。

---

## 7. 测试矩阵

### 7.1 Host

| 测试 | 内容 |
|---|---|
| pointer/router | 新 Kind 的 begin/update/end/cancel；多 handler；重叠区域；旧 Filter 不收 |
| gesture.Pinch | 两指距离比例、中心、取消、阈值 |
| gesture.Rotate | 两点角度、方向、累计角 |
| gesture.Swipe/Hold | 三指位移、速度、手指数、hold 超时 |
| backend fixture | Windows/macOS/Wayland/OHOS 事件→Event 的 golden trace |
| benchmark | 事件处理 0 alloc；手势更新 < 1µs |

### 7.2 真机

| 平台 | 设备/环境 | 必测 |
|---|---|---|
| Windows | Precision Touchpad 笔记本 | 双指 pinch/rotate、三指系统手势不被抢、触摸屏 |
| macOS | MacBook | pinch/rotate/swipe、phase、系统三指拖移 |
| Linux | GNOME/KDE Wayland | pinch/rotate/swipe/hold、axis_source |
| Android | 手机/平板 | 双指 pinch/rotate、返回手势 |
| iPadOS | iPad + 触控板 | pinch/rotate/swipe、hover、系统手势 |
| OHOS | 手机 + 2in1 | axis pinch、rotate/swipe/hold 能力、触摸不误触 |
| Web | Chrome/Safari | ctrl+wheel、GestureEvent、页面缩放/返回 |

---

## 8. 兼容与迁移

本次允许破坏性升级，**不保留兼容性开关**：

- 新 Kind 是加法；旧 Filter 不包含新 bit，不会收到。
- pointer.Event 新增字段：keyed 字面量不受影响；unkeyed 字面量属于破坏性变更，release note 说明即可（本 fork 接受）。
- macOS 只输出 Pinch/Rotate/Swipe 语义事件；删除 Ctrl+Scroll 旧路径，不保留 GIO_MACOS_LEGACY_MAGNIFY。
- OHOS 新导出与 shim 同步升级；不保留旧 shim 的 Scroll 回退。
- Wayland 合成器不支持协议时无手势，不报错；应用可用 gesture 包回退。
- 不改变 Scroll 的既有语义；新应用应优先用 Pinch/Rotate，不再依赖 Ctrl。
- 不保留 GIO_WINDOWS_TOUCHPAD_GESTURES、GIO_WAYLAND_GESTURES、GIO_OHOS_GESTURES、GIO_ANDROID_GESTURES、GIO_IOS_GESTURES、GIO_WEB_GESTURES 等兼容开关；异常时用 git revert 或平台级禁用。


---

## 9. 风险与回滚

| 风险 | 影响 | 缓解 |
|---|---|---|
| 新 Kind 路由错误 | 旧应用收到意外事件 | 默认 opt-in；golden 测试；逐后端开关 |
| 平台单位不一致 | 缩放方向/速度错 | 统一 scale 绝对、rotation 弧度；每平台 fixture |
| 原生 + 回退双份 | 双倍缩放 | 能力查询；每平台只启用一条路径 |
| 系统手势冲突 | 返回/多任务失效 | 不抢系统手势；opt-in；真机清单 |
| macOS 行为变化 | 旧应用 Ctrl+Scroll 失效 | 破坏性变更说明；需要旧行为的分支自行回退提交 |
| OHOS shim 不支持 | 手势缺失 | 可选导出；不可用时不发假事件 |
| 性能回退 | 输入延迟 | 0 alloc；无每事件 goroutine；基准门禁 |

回滚：

- 不设兼容开关；回滚以 git revert 单个工作包/提交为单位。
- WP1 的事件模型是核心，回滚需整体 revert；因此先合 WP1 的测试与空实现，再逐平台启用。

---

## 10. 验收清单

### M1

- [ ] pointer.Event 新字段/新 Kind 通过 host 测试。
- [ ] 旧 Filter 不收 Pinch/Rotate/Swipe/Hold。
- [ ] Windows PT_TOUCHPAD 来源正确；双指 pinch。
- [ ] macOS pinch 输出 Pinch，不再 Ctrl+Scroll（默认）。
- [ ] Wayland 绑定 pointer-gestures，pinch 可用。
- [ ] OHOS axis pinch 输出 Pinch，单指不误触。

### M2

- [ ] rotate/swipe/hold 在 macOS/Wayland/OHOS 可用。
- [ ] Windows 触摸屏 WM_GESTURE 接入（可选）。
- [ ] 系统三/四指手势未被抢占。
- [ ] 性能基准达标。

### M3

- [ ] Android/iOS/Web 回退识别器可用。
- [ ] iPad 触控板 pinch/rotate/hover。
- [ ] Web ctrl+wheel 回退不破坏页面缩放/返回。

### M4

- [ ] Pen 来源/压力/倾斜评估完成。
- [ ] 原始触控板帧/能力查询按需交付。

---

## 11. 附录 A：文件改动清单

| 文件 | 改动 |
|---|---|
| io/pointer/pointer.go | Source/Kind/Event/Phase |
| io/input/pointer.go | 语义手势路由、cancel、filter |
| io/input/pointer_test.go | 新旧 Kind 行为 |
| gesture/gesture.go + 新文件 | Pinch/Rotate/Swipe/Hold 状态机 |
| app/os_windows.go | PT_* 分类、RegisterTouchpadCapableWindow、手势 |
| app/os_macos.go/.m | NSEvent 手势、phase、语义 Pinch/Rotate/Swipe |
| app/os_wayland.go/.c | pointer-gestures、axis_source |
| app/os_x11.go | 回退/XI2 可选 |
| app/os_android.go + Java | 原始多点/识别器 |
| app/os_ios.go + ObjC | UIGestureRecognizer/指针 |
| app/ohos_pump.go | axis pinch → Pinch、rotate/swipe/hold |
| app/ohos_abi.go/glue.h | 可选 gesture 导出 |
| app/os_js.go | Pointer Events/wheel/gesture fallback |
| docs/* | 状态、契约、测试矩阵 |

## 12. 附录 B：任务看板

| 任务 | 包 | 依赖 | 估算 |
|---|---|---|---|
| T1 事件模型 | WP1 | — | 2d |
| T2 路由语义 | WP1 | T1 | 2d |
| T3 gesture 状态机 | WP1 | T1 | 1d |
| T4 Windows 来源/注册 | WP2 | T1 | 1d |
| T5 Windows pinch/rotate | WP2 | T2,T4 | 3d |
| T6 macOS pinch/rotate/swipe | WP3 | T2 | 3d |
| T7 Wayland 协议 | WP4 | T2 | 4d |
| T8 OHOS pinch | WP5 | T2 | 2d |
| T9 OHOS rotate/swipe/hold | WP5 | T8 | 3d |
| T10 Android/iOS 回退 | WP6 | T3 | 5d |
| T11 Web | WP7 | T2 | 2d |
| T12 仲裁/能力 | WP8 | T4-T7 | 3d |
| T13 测试/CI/文档 | WP9 | 全程 | 3d |

总计约 34.5 Go 人日 + 4 天 shim；单人含缓冲约 8 周，双人可压到 5 周。

---

## 13. 附录 C：参考

- [跨平台手势梳理](input-gestures-cross-platform.md)
- [Gio pointer 定义](../io/pointer/pointer.go)
- [Gio 输入路由](../io/input/pointer.go)
- [Gio gesture 包](../gesture/gesture.go)
- Windows：Precision Touchpad Programming Guide、WM_GESTURE、TouchpadGesturesController
- macOS：Cocoa Event Handling Guide、NSEvent、UIGestureRecognizer
- Linux：libinput gestures、Wayland pointer-gestures
- Android：MotionEvent、ScaleGestureDetector
- HarmonyOS：ArkUI AxisEvent、PinchGesture、Native XComponent
- Web：Pointer Events、WheelEvent、GestureEvent

（完整外链见 input-gestures-cross-platform.md 第 9 节。）
