# OHOS shim 契约

> 面向 `libgpofdohos`（ArkTS/napi）开发者。ABI 细节见 [ohos-abi.md](ohos-abi.md)。

## 1. 生命周期

1. 启动：`gio_ohos_store_host_v3` → `gio_ohos_configure` → `gio_ohos_run`。
2. XComponent 创建：`gio_ohos_surface_created(win,w,h)`；尺寸变化用 `surface_changed`。
3. 前后台：`gio_ohos_lifecycle(0/1)`；销毁：`gio_ohos_lifecycle(2)`。
4. destroy 后不得再投递输入；surface destroy 只释放 GPU surface，不结束窗口。

## 2. 帧

- Go 调 `invalidate` 表示"请调度一帧"；shim 在 vsync 调 `gio_ohos_frame_at(ns)`（旧版 `gio_ohos_frame`）。
- 连续动画：Go 会通过 `set_animating(1)` 或每次 frame 后再次 invalidate 维持；`set_animating(0)` 后停止。
- 空闲时不得主动连续调 `frame`，否则破坏 0 帧目标。

## 3. 窗口管理

- `set_window_mode`：0 windowed / 1 fullscreen / 2 minimized / 3 maximized。
- `set_title`、`set_orientation`、系统栏颜色、`set_top_most`、`set_window_limits`。
- 用户手势改变模式后调 `gio_ohos_window_mode(mode)`；方向变化调 `gio_ohos_orientation`。
- `request_close` → 销毁窗口 → `lifecycle(2)`。

## 4. IME

| Go → shim | 含义 |
|---|---|
| `show_keyboard(show)` | 显示/隐藏软键盘 |
| `ime_selection(text,len,start,end)` | 周围文本（UTF-8，len 字节）+ 选区（相对 text 的 rune） |
| `ime_cursor(x,y,w,h)` | 光标框，surface 像素 |
| `ime_config(hint)` | 0 any / 1 text / 2 numeric / 3 email / 4 url / 5 telephone / 6 password |
| `ime_keyboard_status(0/1)` | Go 报告键盘状态 |

| shim → Go | 含义 |
|---|---|
| `gio_ohos_preedit(text,selStart,selEnd)` | 组合文本；sel 相对 preedit 的 rune |
| `gio_ohos_text(text)` | 上屏文本；硬件键不要重复发 |
| `gio_ohos_ime_delete(before,after)` | 相对选区删 rune |
| `gio_ohos_ime_move(dir)` | 1 up 2 down 3 left 4 right |
| `gio_ohos_ime_set_selection(start,end)` | 绝对文档 rune |
| `gio_ohos_ime_enter(type)` | 8 = newline |

## 5. 输入

- touch action：0 down / 1 move / 2 up / 3 cancel；id 为触点编号。
- mouse action：0 press / 1 release / 2 move；button 1 left / 2 right / 3 middle。
- axis：dx/dy 为滚动像素；pinch 的 scale 相对 1.0。
- key：action 0 down / 1 up；mods 位见 `ohos_input.go`；Meta 用 bit 4。
- 硬件按键不要再发 `text`；软键盘只发 `text/preedit`。

## 6. 文件与剪贴板

- `gio_ohos_drop(uris_json)`：JSON 数组，绝对路径；兼容换行分隔。
- `gio_ohos_clipboard(mime,text)`：文本。
- `gio_ohos_clipboard_data(mime,data,len)`：二进制，保留 NUL。
- `pick_file_ex(mode,mimes,multiple)` → `gio_ohos_pick_result` / `pick_error`。
- `set_clipboard_data` 用于 Go 写出二进制；shim 必须复制 data。

## 7. 权限

- Go 调 `request_permission(name, requestID)`；shim 完成后调
  `gio_ohos_permission_result(requestID, granted, errMsg)`。
- 权限名使用 OHOS 原生字符串（如 `ohos.permission.CAMERA`）；Go 侧提供常量。
- `request_permission` 为 NULL 时 Go 立即回调 unsupported。

## 8. 无障碍（S10）

- `semantics_update(json,len)`：节点 diff；schema 见 S10 实现。
- `semantics_announce(text)`：播报。
- `gio_ohos_semantic_action(id,action)`：读屏触发点击/滚动。

## 9. 降级

- 未实现的能力保持 NULL；Go 只警告一次。
- 不得因为缺少可选回调而崩溃或阻塞。