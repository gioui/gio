# 跨平台触控板 / 触摸屏 / 手势支持梳理

> 范围：Windows、macOS、Linux（X11/Wayland）、Android、iOS/iPadOS、HarmonyOS/OpenHarmony、Web；并对照 Gio 当前实现。
> 结论用于决定 Gio 的输入事件模型与 OHOS/桌面后端下一步改造，不替代各平台 API 文档。
> 实现进展（2026-10-27）：pointer 事件模型（Source/Kind/Phase/Scale/Rotation/Swipe）与语义手势路由已落地；macOS pinch/rotate/swipe、OHOS axis pinch、Windows PT_* 分类与 Precision Touchpad 注册、gesture.Pinch/Rotate/Swipe/Hold 回退识别器已接入；Wayland/Android/iOS/Web 接线与 Windows pinch/rotate 仍待做。

---

## 1. 结论摘要

1. **触摸屏和触控板是两套不同的输入范式**：触摸屏以“原始多点坐标”为主，触控板以“指针 + 语义手势”为主。系统通常把触控板的 pinch/rotate/swipe 识别成语义事件，而触摸屏的多指通常原样交给应用，由应用自己识别。
2. **桌面系统普遍有系统级语义手势**：Windows Precision Touchpad、macOS NSEvent、Wayland pointer-gestures、iPadOS、HarmonyOS 2in1 都有 pinch/rotate/swipe 等能力；但“应用能否拿到”差异很大。
3. **移动系统以应用级识别器为主**：Android 的 ScaleGestureDetector/GestureDetector、iOS 的 UIGestureRecognizer 都是应用自己挂识别器；系统同时保留返回/Home/多任务等边缘手势。
4. **Gio 当前只有 Mouse/Touch + Scroll**：io/pointer 没有 Trackpad/Pen 来源，也没有 Pinch/Rotate/Swipe/Hold 语义；只有 macOS 把 magnify 合成 Ctrl+Scroll、OHOS 把 axis pinch 合成 Scroll，语义已经丢失。
5. **Windows 后端存在一个具体缺陷**：PT_TOUCHPAD 在 pointerUpdate 中被归为 pointer.Touch（只判断了 PT_MOUSE），且没有调用 RegisterTouchpadCapableWindow，精度触控板原始帧和语义手势都拿不到。

---

## 2. 术语与边界

| 术语 | 含义 |
|---|---|
| 原始多点 | 每个触点独立上报（id、x/y、phase、pressure 等），由应用识别手势 |
| 语义手势 | 系统/驱动已识别为 scroll / pinch / rotate / swipe / hold，应用只拿结果 |
| 直接输入 | 手指直接接触屏幕（触摸屏、手写笔） |
| 间接输入 | 触控板/鼠标，指针在屏幕上移动，手势发生在另一块表面 |
| 系统保留手势 | 边缘返回、Home、多任务、通知中心等，应用通常不能完全抢占 |
| 应用仲裁 | 应用声明“这块区域/这个手势我要自己处理”，如 Android exclusion rects、iOS 边缘延迟、Web touch-action |

> 重要区别：**“系统支持某手势”不等于“普通应用能收到该手势”**。很多三指/四指手势被桌面环境或移动系统保留。

---

## 3. 总览矩阵

图例：✅ 原生完整；◐ 有条件/取决于设备、版本或合成器；⚪ 仅系统使用或应用不可直接获取；❌ 无。

| 平台 | 触摸屏原始多点 | 触控板原始帧 | 触控板滚动 | Pinch | Rotate | 3/4 指 Swipe | Hold | 应用仲裁 |
|---|---|---|---|---|---|---|---|---|
| Windows 10/11 | ✅ WM_POINTER/WM_TOUCH | ◐ Precision Touchpad，需注册 | ✅ | ✅ | ✅ | ◐ 三指全局手势，前台可接 | ◐ | ◐ |
| macOS | ❌ 无触摸屏 | ✅ NSEvent trackpad touches | ✅ | ✅ | ✅ | ◐/✅ swipeWithEvent | ⚪ | ✅ 响应链/系统偏好 |
| Linux X11 | ◐ XInput2，取决于客户端 | ❌ 通常只有指针/按钮 | ✅ 按钮 4–7 | ❌ | ❌ | ⚪ 桌面环境 | ❌ | ◐ |
| Linux Wayland | ✅ wl_touch | ◐ 由合成器/协议决定 | ✅ wl_pointer.axis | ◐ pointer-gestures | ◐ pointer-gestures | ◐ pointer-gestures | ◐ 协议 v3 | ◐ 合成器 |
| Android | ✅ MotionEvent 多点 | ◐ 桌面/OEM 模式，通常按鼠标处理 | ✅ generic motion | ✅ 应用级 ScaleGestureDetector | ◐ 应用自定义 | ✅ GestureDetector/系统导航 | ✅ 长按 | ◐ exclusion rects |
| iOS / iPadOS | ✅ UITouch | ◐ iPadOS 间接指针 | ✅ | ✅ UIPinchGestureRecognizer | ✅ UIRotationGestureRecognizer | ✅ UISwipeGestureRecognizer | ✅ 长按 | ✅ 边缘延迟/手势依赖 |
| HarmonyOS / OpenHarmony | ✅ XComponent touch | ◐ AxisEvent/触控板 | ✅ AxisEvent | ✅ ArkUI PinchGesture；Native 需合成 | ✅ ArkUI RotationGesture；Native 需合成 | ✅ ArkUI SwipeGesture；Native 需合成 | ✅ LongPressGesture | ◐ 2in1 系统手势 |
| Web | ✅ Touch/Pointer Events | ❌ 无原始触控板 | ✅ wheel | ◐ ctrl+wheel/Safari GestureEvent | ❌ | ◐ 浏览器前进/后退 | ◐ | ❌ 浏览器主导 |

---

## 4. 手势语义矩阵（应用视角）

| 手势 | Windows | macOS | Wayland | Android | iPadOS | HarmonyOS | Web |
|---|---|---|---|---|---|---|---|
| 单击/双击 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 长按 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ◐ |
| 拖动/平移 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 双指滚动 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 双指捏合缩放 | ✅ | ✅ | ◐ | ✅ 应用识别 | ✅ | ✅ | ◐ 浏览器缩放 |
| 双指旋转 | ✅ | ✅ | ◐ | ◐ 应用识别 | ✅ | ✅ | ❌ |
| 三指滑动 | ◐ 前台全局 | ◐/✅ | ◐ | ✅ 应用/系统 | ✅ | ✅ | ◐ 浏览器导航 |
| 四指滑动 | ◐ 前台全局 | ⚪ 系统 | ◐ | ◐ OEM | ⚪ 系统 | ◐ 系统 | ⚪ 系统 |
| 边缘返回 | ⚪ | ⚪ | ⚪ | ✅ 系统手势 | ✅ 系统手势 | ✅ 系统手势 | ◐ 浏览器 |
| 悬停 Hover | ✅ | ✅ | ✅ | ✅ 鼠标 | ✅ 指针 | ✅ 鼠标 | ✅ |

> “应用识别”表示系统只给原始触点，应用需自己实现；Gio 目前没有内置这些识别器。

---

## 5. 逐系统分析

### 5.1 Windows 10/11

**触摸屏**

- 统一指针栈：WM_POINTERDOWN/UPDATE/UP，用 GetPointerInfo/GetPointerTouchInfo 拿触摸点、压力、接触区域。
- 旧接口：WM_TOUCH + GetTouchInputInfo。
- 手势：WM_GESTURE 提供 pan/zoom/rotate/two-finger tap 等；InteractionContext 提供 manipulation、inertia、rails；Direct Manipulation 适合低延迟平移/缩放。
- 系统保留：边缘滑动（操作中心/任务视图）、三/四指手势由系统设置决定。

**触控板**

- Precision Touchpad：窗口需 RegisterTouchpadCapableWindow，之后收到 WM_POINTER，指针类型为 PT_TOUCHPAD；用 GetPointerFrameTouchpadInfoHistory/GetPointerTouchpadInfo 拿触控板帧。
- 双指手势可用 InteractionContext 识别平移/缩放/旋转，并用 ReportWindowContentInertia 报告惯性。
- 三指手势可用 WinRT TouchpadGesturesController + PhysicalGestureRecognizer（前台进程），支持 ThreeFingerActions/ThreeFingerManipulations；系统仍可能保留部分全局手势。
- 不注册 Precision Touchpad 时，触控板通常以鼠标滚轮/指针事件出现，应用拿不到原始帧。

**Gio 现状**

- WM_POINTER → pointer.Event，WM_POINTERWHEEL → Scroll。
- pointerUpdate 只把 PT_MOUSE 映射为 Mouse，**PT_TOUCHPAD 会落到 Touch**；且没有 RegisterTouchpadCapableWindow，因此精度触控板帧/手势基本不可用。
- 没有处理 WM_GESTURE、InteractionContext、TouchpadGesturesController；pinch/rotate/swipe 全部缺失。

### 5.2 macOS

**触控板**

- 系统把触控板手势识别为 NSEvent：
  - scrollWheel：双指滚动，带 scrollingDeltaX/Y、phase、momentumPhase、hasPreciseScrollingDeltas。
  - magnifyWithEvent：pinch 缩放，magnification。
  - rotateWithEvent：双指旋转，rotation。
  - swipeWithEvent：三指滑动，deltaX/deltaY。
- NSResponder 方法沿响应链传递；NSEventPhase 有 begin/changed/ended/cancelled，适合做连续手势。
- Apple 明确提醒：触控板手势/触摸事件不能作为关键功能的唯一入口；NSGestureRecognizer 不接收触控板触摸。
- 系统偏好里可配置滚动、缩放、旋转、三指拖移等；部分三/四指手势被系统占用。

**触摸屏**

- Mac 没有触摸屏；iPad 见 5.5。

**Gio 现状**

- scrollWheel → Scroll。
- gio_onMagnify → Scroll{Y: magnifyScroll(delta)} 并强制加 Ctrl；**pinch 语义被降级为 Ctrl+滚轮**。
- rotateWithEvent/swipeWithEvent/原始 trackpad touches 未处理；没有 phase/velocity。

### 5.3 Linux：X11 与 Wayland

**X11**

- 传统客户端只有核心指针事件和按钮 4–7 的滚轮；触摸/手势依赖 XInput2（XI_Touch*、valuator）。
- libinput 在 Xorg 下通常由 xf86-input-libinput 消费；三/四指手势多由桌面环境（GNOME/KDE）处理，普通客户端没有标准协议。
- 应用可拿 XInput2 原始触摸，但跨桌面一致性差。

**Wayland**

- wl_pointer.axis：滚动；axis_source 可区分 wheel/finger/continuous（Gio 目前忽略）。
- wl_touch：原始多点。
- zwp_pointer_gestures_v1：语义 swipe/pinch/hold；pinch 事件包含 dx/dy、scale（相对 begin 的绝对比例）、rotation（相对上一次的顺时针角度）；swipe 三阶段 begin/update/end；hold 两阶段 begin/end（v3）。
- 是否可用取决于合成器是否绑定并转发；应用需要自己 bind 协议。
- libinput 负责识别触控板手势，但**触摸屏手势不由 libinput 解释**，由客户端/合成器按窗口上下文处理。

**Gio 现状**

- X11：只有核心指针 + 按钮滚动，无 XI2 触摸/手势。
- Wayland：wl_touch + wl_pointer.axis；未绑定 pointer-gestures，忽略 axis_source；pinch/rotate/swipe/hold 全缺。

### 5.4 Android

**触摸屏**

- MotionEvent：ACTION_DOWN/MOVE/UP/CANCEL、ACTION_POINTER_DOWN/UP、getPointerCount/getPointerId/getHistorical*；支持压力、尺寸、工具类型（finger/stylus/mouse）。
- 应用级识别：ScaleGestureDetector（pinch）、GestureDetector（tap/double-tap/long-press/scroll/fling）、View.onTouchEvent。
- 系统手势：手势导航的返回/Home/多任务；setSystemGestureExclusionRects 可排除有限区域（有面积/高度限制，且不能完全禁用返回）。

**触控板/鼠标**

- onGenericMotionEvent 收到 ACTION_SCROLL、AXIS_VSCROLL/HSCROLL；鼠标按键、hover 通过 MotionEvent/View 处理。
- Android 桌面模式/OEM 桌面（DeX 等）下触控板通常按鼠标/滚动事件处理；三指系统手势由系统/厂商决定，应用没有统一语义 API。

**Gio 现状**

- Java 层逐 pointer 调 onTouchEvent，Gio 得到独立 PointerID 的 Press/Move/Release；滚轮 → Scroll。
- 没有挂 ScaleGestureDetector/GestureDetector；pinch/rotate/swipe 需应用自己从多点事件识别。
- 鼠标/触控板来源都走 pointer.Mouse，没有 Trackpad 区分。

### 5.5 iOS / iPadOS

**触摸屏**

- UITouch 多点，UIEvent 有 coalesced/predicted touches；UIGestureRecognizer 子类覆盖 tap/pinch/rotation/swipe/pan/screen-edge/long-press/hover。
- 手势识别器与 view 绑定，支持 require(toFail:)、delegate 做冲突仲裁。
- 系统手势：Home indicator、边缘返回、iPad 多任务；preferredScreenEdgesDeferringSystemGestures 可延迟，但不能完全禁用。

**iPadOS 触控板/鼠标**

- iPadOS 13.4+ 支持间接指针；UIKit 提供 UIPointerInteraction、UIHoverGestureRecognizer，以及指针效果/形状定制。
- 触控板双指滚动、pinch、rotate 通常由 UIScrollView/UIGestureRecognizer 接收；三指/四指手势由系统保留。
- iPhone 的鼠标支持主要面向辅助功能，不是通用触控板范式。

**Gio 现状**

- 每个 UITouch → pointer.Touch；没有 UIGestureRecognizer、没有间接指针/hover；pinch/rotate/swipe 全缺。
- 没有滚动事件（iOS 后端未接 UIPanGestureRecognizer/scroll view）。

### 5.6 HarmonyOS / OpenHarmony

**ArkUI（声明式）**

- 内置手势：TapGesture、LongPressGesture、PanGesture、PinchGesture、RotationGesture、SwipeGesture；支持手势组、并行/互斥、优先级。
- 轴事件：onAxisEvent/AxisEvent 提供 getVerticalAxisValue、getHorizontalAxisValue、getPinchAxisScaleValue、getAxisAction；触控板/滚轮/捏合可走 axis。
- 鼠标：onMouse、onHover；触摸：onTouch。
- 2in1/PC 模式有三指/四指系统手势、窗口管理手势；应用通常只能拿到未被系统消费的事件。

**Native XComponent（Gio 走这条）**

- OH_NativeXComponent 注册 touch/mouse/key 回调；axis 通过 OH_ArkUI_AxisEvent_* 读取。
- **ArkUI 的高层 GestureRecognizer 不直接作用于 XComponent 自绘内容**；native 侧必须从 touch/axis 自己合成手势。
- 当前 Gio OHOS shim 已把 axis pinch 转成 Scroll，并做“触摸后 150ms 忽略 axis pinch”的混合设备抑制；没有 rotate/swipe/hold。

**Gio 现状**

- touch/mouse/axis → pointer.Touch/Mouse/Scroll；pinch → Scroll（语义丢失）。
- 没有 rotate/swipe/hold；没有 trackpad 来源；多指触点是独立 PointerID。

### 5.7 Web

- Pointer Events：pointerdown/move/up/cancel，pointerType 为 mouse/pen/touch；多指针可跟踪。
- Touch Events：touchstart/move/end/cancel，changedTouches/touches；与 Pointer Events 二选一或并存。
- wheel：deltaX/Y/Z、deltaMode；触控板双指滚动通常走 wheel；浏览器常把 pinch 映射为 ctrlKey=true 的 wheel。
- Safari 专有 GestureEvent（gesturestart/change/end，scale/rotation），非标准。
- 浏览器保留：页面缩放（pinch）、前进/后退滑动、下拉刷新；touch-action/preventDefault 可有限控制，passive listener 下不能阻止。
- 没有标准“原始触控板帧”或“三指手势”API。

**Gio 现状**

- JS 后端监听 mouse/touch/wheel；没有 Pointer Events、没有 GestureEvent、没有 touch-action 仲裁；pinch 仅可能以 ctrl+wheel 形式出现。

---

## 6. Gio 当前输入模型与差距

### 6.1 模型（改造前基线）

> 下面是本次改造前的模型，用于解释为什么需要新增 Source/Kind 字段；当前实现已扩展，见第 7 节。 
~~~go
// io/pointer
type Source uint8
const (
    Mouse Source = iota
    Touch
)
type Kind uint
const (
    Cancel Kind = 1 << iota
    Press; Release; Move; Drag; Enter; Leave; Scroll
)
type Event struct {
    Kind Kind
    Source Source
    PointerID ID
    Position f32.Point
    Scroll f32.Point
    Buttons Buttons
    Modifiers key.Modifiers
    Time time.Duration
}
~~~

- 只有两种来源；没有 Trackpad、Pen。
- 没有 Pinch/Rotate/Swipe/Hold；没有手势 phase、scale、rotation、velocity。
- Scroll 是唯一“语义”通道，导致 macOS/OHOS 把 pinch 硬塞进 Scroll（macOS 加 Ctrl，OHOS 直接合成）。
- 多点只能靠 PointerID 自己跟踪；gesture 包只有 Click/Drag/Scroll/Hover，没有 pinch/rotate/swipe。

### 6.2 后端映射

| 后端 | 触摸 | 滚动 | Pinch | Rotate | Swipe | Trackpad 来源 |
|---|---|---|---|---|---|---|
| macOS | ❌ 无触摸屏 | scrollWheel | Pinch（magnify） | Rotate | Swipe | ✅ Trackpad |
| X11 | ❌ 无 XI2 touch | 按钮 4–7 | ❌ | ❌ | ❌ | ❌ |
| Wayland | wl_touch | wl_pointer.axis | ❌ 未绑 pointer-gestures | ❌ | ❌ | ❌ 忽略 axis_source |
| Android | MotionEvent 多点 | generic motion | ❌ | ❌ | ❌ | ❌ |
| iOS | UITouch | ❌ | ❌ | ❌ | ❌ | ❌ |
| OHOS | XComponent touch | AxisEvent | Pinch（axis） | ❌ | ❌ | ❌ |
| Windows | WM_POINTER；PT_TOUCHPAD 已分类 | WM_POINTERWHEEL | ❌ 尚未接 InteractionContext | ❌ | ❌ | ✅ Trackpad/Pen 分类 |
| Web | Touch Events | wheel | ❌（可能 ctrl+wheel） | ❌ | ❌ | ❌ |

### 6.3 关键问题

1. **语义丢失**：pinch 曾降级为 Ctrl+Scroll；**macOS/OHOS 已改为语义 Pinch**，Wayland/Windows 仍待接。
2. **来源丢失**：触控板、鼠标、触摸、笔曾挤进 Mouse/Touch；**Windows 已按 PT_* 分类，macOS 手势为 Trackpad，OHOS pinch 为 Trackpad**；Wayland/Android/iOS 仍待。
3. **相位丢失**：新 Event 已有 Phase；**macOS/OHOS 已填 begin/update/end/cancel**；滚动 phase 仍待补。
4. **系统手势冲突**：没有声明/仲裁 API，应用无法知道哪些边缘/三指手势被系统保留；**仍待设计**。
5. **Windows 具体 bug**：PT_TOUCHPAD 曾归为 pointer.Touch；**已修分类并注册 Precision Touchpad**，但 pinch/rotate 尚未接 InteractionContext。
6. **Wayland 具体缺口**：axis_source 被忽略；未绑定 zwp_pointer_gestures_v1；**仍待做**。
7. **macOS 具体缺口**：rotateWithEvent/swipeWithEvent 曾未接、magnify 被标 Ctrl；**已修**。
8. **OHOS 具体缺口**：axis pinch 曾只合成 Scroll；**已改为语义 Pinch**；rotate/swipe/hold 仍待。

---

## 7. 建议的事件模型

### 7.1 扩展 io/pointer

建议新增语义手势 Kind（或独立 GestureEvent，用 Filter.Kinds 匹配）：

~~~go
const (
    Pinch Kind = 1 << iota
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
    // existing...
    Source Source // + SourceTrackpad, SourcePen
    Phase  GesturePhase
    Scale  float32 // pinch: absolute scale relative to begin
    Rotation float32 // radians
    Swipe    f32.Point // logical center delta
    Fingers  uint8
}
~~~

设计要点：

- **Pinch 的 Scale 用相对 begin 的绝对值**（与 Wayland/libinput 一致），避免应用自己累积误差。
- **Rotate 用弧度**，平台角度统一换算；明确是“相对上一次”还是“相对 begin”，建议统一相对 begin 的累计角度。
- **Swipe 用逻辑中心位移 + 手指数**，方向由应用判断；不要只给“左/右”。
- **Phase 必须完整**；Cancel 时应用回滚未提交状态。
- **保留 Scroll 给双指平移/滚轮**，不要继续用 Ctrl 编码 pinch。

### 7.2 来源扩展

~~~go
const (
    Mouse Source = iota
    Touch
    Trackpad
    Pen
)
~~~

- Windows：PT_TOUCHPAD → Trackpad；PT_PEN → Pen；PT_TOUCH → Touch；PT_MOUSE → Mouse。
- macOS：NSEvent 触控板手势 → Trackpad；鼠标滚轮 → Mouse。
- Wayland：axis_source=finger → Trackpad；wheel → Mouse。
- OHOS：axis 事件 → Trackpad（若 shim 能区分）或保持 Mouse；触摸 → Touch。
- Android：TOOL_TYPE_STYLUS → Pen；SOURCE_MOUSE → Mouse；触控板若可识别 → Trackpad。

### 7.3 各后端映射建议

| 平台 | 建议 |
|---|---|
| Windows | RegisterTouchpadCapableWindow；WM_POINTER 按 PT_* 分类；WM_GESTURE/InteractionContext 输出 Pinch/Rotate/Pan；三指全局手势用 TouchpadGesturesController 可选接入 |
| macOS | 接 magnifyWithEvent/rotateWithEvent/swipeWithEvent/scrollWheel；使用 NSEventPhase 映射 Phase；不要再把 magnify 变 Ctrl+Scroll |
| Wayland | bind zwp_pointer_gestures_v1；pinch 的 scale/rotation 直接映射；swipe/hold 映射；保留 axis_source |
| X11 | 可选接 XInput2 touch；应用级识别 pinch/rotate；桌面环境手势不假设可用 |
| Android | 可选内置 ScaleGestureDetector/GestureDetector 适配；或保持原始多点，由 gesture 包提供识别器 |
| iOS/iPadOS | 在 GioView 上挂 UIPinchGestureRecognizer/UIRotationGestureRecognizer/UISwipeGestureRecognizer；接 UIHoverGestureRecognizer/UIPointerInteraction |
| OHOS | axis pinch 直接输出 Pinch（带 scale）；如能拿到 rotation 则输出 Rotate；swipe/hold 由 touch 合成或等待 ArkUI 能力；不要只合成 Scroll |
| Web | Pointer Events 优先；wheel ctrlKey 仅作为 pinch 回退；Safari GestureEvent 可选；touch-action 暴露仲裁 |

### 7.4 手势识别器

在 gesture 包增加可选的应用级识别器，供没有系统语义手势的平台使用：

- gesture.Pinch：从多点 PointerID 计算距离比例、中心、phase。
- gesture.Rotate：从两点角度变化计算旋转。
- gesture.Swipe：从多指中心位移、速度、手指数判断。
- gesture.Hold：从按下后无显著位移的时间判断。

这样 Android/iOS/Web/X11 可以共享一套回退逻辑，而 macOS/Windows/Wayland/OHOS 优先用系统语义事件。

### 7.5 系统手势仲裁

- Windows：前台全局手势 API 是 opt-in；不要抢占系统三/四指手势，提供显式开关。
- macOS：通过响应链让未处理手势冒泡；尊重系统偏好。
- Android：提供 SystemGestureExclusionRects 的封装；不要尝试禁用返回手势。
- iOS：preferredScreenEdgesDeferringSystemGestures/requiresExclusiveTouchType 等按需暴露。
- OHOS/2in1：系统手势优先；应用只处理未消费事件。
- Web：touch-action/preventDefault 仅在非 passive 监听下，且不能覆盖浏览器缩放/返回。

---

## 8. 优先级建议

| 优先级 | 事项 | 理由 |
|---|---|---|
| P0 | 修 Windows PT_TOUCHPAD 分类 + RegisterTouchpadCapableWindow | 当前是明确 bug；影响所有 Windows 精度触控板 |
| P0 | 增加 SourceTrackpad 与 Pinch/Rotate 语义事件 | 各桌面系统都有；Gio 当前完全缺失 |
| P0 | macOS 接 magnify/rotate/swipe，不再 Ctrl+Scroll | 语义正确性；macOS 用户占比高 |
| P0 | Wayland bind pointer-gestures + axis_source | Linux 桌面主路径；协议成熟 |
| P1 | OHOS axis pinch 输出 Pinch（scale），补 rotate/swipe/hold | 本轮 OHOS 重点；避免继续塞 Scroll |
| P1 | gesture 包增加 Pinch/Rotate/Swipe/Hold 回退识别器 | Android/iOS/Web/X11 共用 |
| P1 | iOS 接 UIGestureRecognizer/间接指针 | iPadOS 触控板体验 |
| P2 | Windows 三指全局手势、Android 桌面模式、Web GestureEvent | 平台特定、可用性差异大 |
| P2 | Pen 压力/倾斜、原始触控板帧、触觉反馈 | 需要扩展 Event 字段与设备能力查询 |

---

## 9. 参考

**Windows**

- [Precision Touchpad Programming Guide](https://raw.githubusercontent.com/MicrosoftDocs/win32/docs/desktop-src/input-precisiontouchpad/precision-touchpad-guide.md)
- [WM_GESTURE message](https://learn.microsoft.com/en-us/windows/win32/wintouch/wm-gesture)
- [Getting started with multi-touch gestures](https://learn.microsoft.com/en-us/windows/win32/wintouch/getting-started-with-multi-touch-gestures)

**Apple**

- [Cocoa Event Handling Guide: Handling Trackpad Events](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/HandlingTouchEvents/HandlingTouchEvents.html)
- [UIGestureRecognizer](https://developer.apple.com/documentation/uikit/uigesturerecognizer)
- [UIPinchGestureRecognizer](https://developer.apple.com/documentation/uikit/uipinchgesturerecognizer)
- [Enhancing your iPad app with pointer interactions](https://developer.apple.com/documentation/uikit/enhancing-your-ipad-app-with-pointer-interactions)

**Linux**

- [libinput: Gestures](https://wayland.freedesktop.org/libinput/doc/latest/gestures.html)
- [Wayland pointer-gestures protocol](https://gitlab.freedesktop.org/wayland/wayland-protocols/-/raw/main/unstable/pointer-gestures/pointer-gestures-unstable-v1.xml)

**Android**

- [MotionEvent](https://developer.android.com/reference/android/view/MotionEvent)
- [ScaleGestureDetector](https://developer.android.com/reference/android/view/ScaleGestureDetector)
- [Gestures](https://developer.android.com/develop/ui/views/touch-and-input/gestures)

**HarmonyOS / OpenHarmony**

- [ArkUI 轴事件](https://developer.huawei.com/consumer/cn/doc/harmonyos-references/ts-universal-events-axis)
- [ArkUI PinchGesture](https://developer.huawei.com/consumer/cn/doc/harmonyos-references-v5/ts-basic-gestures-pinchgesture-V5)
- [Native XComponent](https://developer.huawei.com/consumer/cn/doc/harmonyos-references-V5/_o_h___native_x_component-V5)
- [OH_NativeXComponent_TouchEvent](https://developer.huawei.com/consumer/cn/doc/doccenter-references/api/capi-oh-nativexcomponent-native-xcomponent-oh-nativexcomponent-touchevent)

**Web**

- [Pointer events (MDN)](https://developer.mozilla.org/en-US/docs/Web/API/Pointer_events)
- [Touch events (MDN)](https://developer.mozilla.org/en-US/docs/Web/API/Touch_events)
- [WheelEvent (MDN)](https://developer.mozilla.org/en-US/docs/Web/API/WheelEvent)
- [GestureEvent (MDN)](https://developer.mozilla.org/en-US/docs/Web/API/GestureEvent)

**Gio 源码**

- [io/pointer/pointer.go](../io/pointer/pointer.go)
- [gesture/gesture.go](../gesture/gesture.go)
- [app/os_windows.go](../app/os_windows.go)
- [app/os_macos.go](../app/os_macos.go)
- [app/os_x11.go](../app/os_x11.go)
- [app/os_wayland.go](../app/os_wayland.go)
- [app/os_android.go](../app/os_android.go)
- [app/os_ios.go](../app/os_ios.go)
- [app/ohos_pump.go](../app/ohos_pump.go)
- [app/os_js.go](../app/os_js.go)
