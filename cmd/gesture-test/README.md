# Gio Gesture Test

交互式手势诊断程序，用于检查本机 Gio 后端是否能收到原生语义手势。

## 运行

```bash
./cmd/gesture-test/run.sh
```

如果直接 `go run` 报 `compile: version "go1.27.1" does not match go tool version "go1.24.5"`，说明 shell 的 `go` 与 `GOROOT` 不一致（本机 `~/Public/ohos_golang_go` 是 1.24.5，`.bash_profile` 的 `GOROOT` 指向 1.27.1）。用上面的脚本，或显式指定：

```bash
GOROOT=/opt/homebrew/Cellar/go/1.27.1/libexec \
  /opt/homebrew/Cellar/go/1.27.1/libexec/bin/go run ./cmd/gesture-test
```

窗口打开后，把指针移到中间面板，按下面清单操作。

## 检查清单

| 操作 | 预期 |
|---|---|
| 双指捏合/张开 | Pinch 计数增加；Last 显示 `Pinch Trackpad Begin/Update/End`；Scale 从 1 连续变化 |
| 双指旋转 | Rotate 计数增加；Rotation 为弧度，符号稳定 |
| 双指滚动 | Scroll 计数增加；Source 通常为 Mouse（当前 Wayland/OHOS 尚未区分 Trackpad） |
| 三指滑动 | Swipe 计数增加（取决于系统是否把该手势交给应用；macOS 三指手势常被 Mission Control 占用，可能表现为 Scroll） |
| 长按/多指按住 | Hold 计数增加（当前仅 gesture 回退包支持；macOS 后端尚未合成 Hold） |
| 左/右键点击 | Press 的 Buttons 分别显示 ButtonPrimary/ButtonSecondary |
| 双击 | Double 计数增加（测试程序按 300ms/10px 自行识别） |
| Reset | 清空计数与日志 |

## 显示字段

- Kind：Press/Release/Move/Drag/Scroll/Pinch/Rotate/Swipe/Hold
- Source：Mouse/Touch/Trackpad/Pen
- Phase：Begin/Update/End/Cancel（仅语义手势）
- Scale：相对 begin 的绝对值，1 表示无变化
- Rotation：相对 begin 的累计弧度
- Swipe：相对 begin 的逻辑中心位移
- Fingers：参与手指数；macOS 精确触控板滚动为 2，鼠标滚轮为 0
- Buttons：当前按下的鼠标键
- Position/PointerID/GestureID/Time

## 平台现状

| 平台 | 状态 |
|---|---|
| macOS | 原生 magnify/rotate/swipe 已接；三指 Swipe 可能被系统占用；Hold 未合成 |
| Windows | WM_GESTURE 触摸屏 pinch/rotate；Precision Touchpad InteractionContext 待做 |
| OHOS | axis pinch 已输出 Pinch；rotate/swipe/hold 待做 |
| Wayland | 尚未绑定 pointer-gestures，因此通常看不到 Pinch/Rotate/Swipe |
| X11 | 只有原始指针/滚动，需应用级 gesture 回退 |
| Android/iOS/Web | 后端仍输出原始触摸；可用 `gesture` 包回退识别 |

## 说明

这个程序只监听 `pointer.Event`。它不默认启用 `gesture` 包的原始多点回退识别器，因为：
- 桌面触控板的手势已由系统给出语义事件，避免双份；
- 触摸屏平台需要按窗口区域接入回退识别器，属于应用层策略。

如果要在触摸屏上验证回退识别，可参考 `gesture.Pinch`、`gesture.Rotate`、`gesture.Swipe`、`gesture.Hold` 的 host 测试。

### macOS 三指手势说明

macOS 可能把三指滑动识别成系统手势（Mission Control/切换桌面）或滚动，而不是 `swipeWithEvent`。可在
系统设置 → 触控板 → 更多手势中检查“在页面之间轻扫”和“三指拖移”等设置；这不是 Gio 能强制覆盖的。
