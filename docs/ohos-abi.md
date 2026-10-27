# OHOS C ABI

> 本文描述 Gio OHOS 后端（`libgpofd.so`）与 ArkTS/napi shim（`libgpofdohos.so`）之间的二进制契约。
> 代码位置：[app/ohos_glue.h](../app/ohos_glue.h)、[app/ohos_glue.c](../app/ohos_glue.c)、[app/ohos_abi.go](../app/ohos_abi.go)。
> 当前 Go 侧 `GIO_OHOS_ABI_VERSION = 4`。

---

## 1. 两个方向

```
Go libgpofd.so  ── gio_ohos_*（Go 导出，shim 调用） ──▶  ArkTS shim
Go libgpofd.so  ◀── gio_ohos_host(_v3)（函数表，Go 调用） ──  ArkTS shim
```

- **入站**：surface、touch、key、IME、lifecycle、drop 等事件。
- **出站**：invalidate、键盘、窗口管理、IME 上下文、语义、日志等 host 回调。

## 2. 版本

| 版本 | 安装入口 | 内容 |
|---|---|---|
| v1 | `gio_ohos_install_host` | 14 个基础导出；7 个 host 回调 |
| v2 | `gio_ohos_install_host` | 追加 mouse/frame/insets/IME/pick；host 结构 11 个回调 |
| v3 | `gio_ohos_install_host_v3(host, size)` | 独立 `gio_ohos_host_v3`：窗口管理、剪贴板二进制、语义、动画、picker ex |
| v4 | `gio_ohos_install_host_v4(host, size)` | 独立 `gio_ohos_host_v4`：select/create/destroy window + 全部 `_w` 事件 |

**兼容规则**

- `gio_ohos_install_host` 固定按 v2 尺寸拷贝；v1 shim 必须升级到 v2 结构（v1 结构更小，Go 无法安全探测）。
- v3 shim 调用 `gio_ohos_abi_version()`（返回 3）后，再用 `install_host_v3`。
- C 侧 `gio_ohos_store_host_v3(host, size)` 先清零，再 `memcpy(min(size, sizeof))`，并按 size 推断 ABI。
- 所有 v3 回调可 NULL；C wrapper 判空后 no-op。

## 3. `gio_ohos_host_v3` 字段

```c
typedef struct gio_ohos_host_v3 {
    gio_ohos_host v2;                 /* 必须位于首位 */
    void (*set_window_mode)(void*, int);       /* 0 windowed 1 fullscreen 2 minimized 3 maximized */
    void (*set_title)(void*, const char*);
    void (*set_orientation)(void*, int);       /* 0 any 1 landscape 2 portrait */
    void (*set_status_color)(void*, uint32_t argb);
    void (*set_navigation_color)(void*, uint32_t argb);
    void (*set_top_most)(void*, int);
    void (*set_window_limits)(void*, int minw,int minh,int maxw,int maxh);
    void (*pick_file_ex)(void*, int mode, const char *mimes, int multiple);
    void (*request_permission)(void*, const char *name, int requestID);
    void (*request_close)(void*);
    void (*raise_window)(void*);
    void (*center_window)(void*);
    void (*start_window_move)(void*);
    void (*set_clipboard_data)(void*, const char *mime, const void *data, int length);
    void (*semantics_update)(void*, const char *json, int length);
    void (*semantics_announce)(void*, const char *text);
    void (*set_animating)(void*, int);
} gio_ohos_host_v3;

typedef struct gio_ohos_host_v4 {
    gio_ohos_host_v3 v3;              /* 必须位于首位 */
    void (*select_window)(void*, uint64_t windowID);
    void (*create_window)(void*, uint64_t windowID);
    void (*destroy_window)(void*, uint64_t windowID);
} gio_ohos_host_v4;
```

## 4. 关键约定

| 主题 | 约定 |
|---|---|
| 线程 | `gio_ohos_*` 可从任意线程调用；Go 只入队，不阻塞 ArkUI 线程 |
| 内存 | 传入的 `char*`/`void*` 仅在调用期间有效；shim 必须复制后再异步使用 |
| 坐标 | surface/触摸/insets/cursor 均为物理像素 |
| IME | `ime_selection` 的 start/end 相对传入 text 的 rune；`ime_set_selection` 是绝对文档 rune；preedit 选区相对 preedit 文本 |
| 文本长度 | `ime_selection` 的 length、clipboard 的 length 为字节 |
| 关闭 | `request_close` 后由系统走 lifecycle(destroy)，Go 不直接杀进程 |
| vsync | `invalidate` 请求一帧；`frame/frame_at` 只在被调度或 animating 时回调 |

## 5. 能力探测

```c
int abi = gio_ohos_abi_version();       // Go 侧实现版本
int host = gio_ohos_host_abi_version(); // 已安装 shim 的版本
```

- `host < 3`：窗口管理/语义/二进制剪贴板/拖放不可用，Go 侧降级并只警告一次。
- 具体回调仍需判空：ABI 版本相同也可能个别字段为 NULL。

## 6. 当前实现状态

| 能力 | Go 侧 | shim 侧 |
|---|---|---|
| 生命周期/队列/帧调度 | 已实现 | 需实现 invalidate/frame/destroy |
| 窗口管理 | 已实现（v3 调用） | 待实现 |
| IME | 已实现（复用 textInputState） | 待实现单位对齐 |
| 文件拖放/二进制剪贴板 | 已实现 | 待实现 |
| 运行时权限 | 已实现（request_permission/result） | 待实现 |
| 特性开关 | SYNC_FRAME / NO_A11Y 已实现；IME_LEGACY 未实现 | — |
| 无障碍 | 待实现（S10） | 待评估 ArkUI 虚拟节点 |
| 多窗口 | Go 侧路由/ABI v4 已就绪；shim 需实现 create/select/destroy 与 `_w` 事件 | 待实现 |