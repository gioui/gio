# OHOS Stage Gate 报告（M0–M2 / S0–S10）

- 日期：2026-10-27
- 分支：`fix/ohos-m0`
- 基线：`65fbc052`（分析文档 + 既有 WIP）
- 结论：**Go 侧自动化 Gate 通过；真机 Gate 条件通过（无 OHOS 设备/shim v3 实现）**

## 1. 已完成阶段

| 阶段 | 提交 | 内容 |
|---|---|---|
| S0 | `5944a367` | 删除逐事件日志、debug 开关、死字段、文档引用、标签检查脚本 |
| S1 | `9e173e90` | per-window 队列、staging、Stop/close、deliver 守卫、eventLoop.done |
| S2 | `6c9a0d28` | request-driven 帧调度、`ohosEvInvalidate`、Size 只读 |
| S3 | `cf60b609` | ABI v3 独立结构、size 协商、版本查询、C 契约测试 |
| S4 | `85ebbebc` | 配置 delta、窗口模式/标题/系统栏/方向/置顶/限制、Perform |
| S5 | `6cbe51f0` | IME 复用 textInputState、rune/byte 坐标、去重、选区 clamp |
| S6 | `295efd8c` | 键位/光标全表、Meta、鼠标 Buttons、捏合抑制窗、文本去重 |
| S7 | `5c983d1e` | 运行时 density/font-scale、Insets 重排、方向事件 |
| S8 | `67e0c72b` | 文件拖放、picker ex、二进制剪贴板 |
| S9 | `f4189967` | ABI 文件拆分、ABI/shim/testing 文档、CI 检查 |
| S10 | `14c13ad0` | 语义树 JSON、读屏动作、announce |

S11（本报告 + 基准）为收口阶段。

## 2. 自动化证据

| 检查 | 命令 | 结果 |
|---|---|---|
| OHOS host 单测 | `go test ./app -tags gio_ohos_test` | 通过（53 个 OHOS 测试） |
| 竞态 | `go test -race ./app -tags gio_ohos_test` | 通过 |
| 全仓测试 | `go test ./...` | 通过 |
| C ABI 契约 | `./script/test-ohos-abi.sh` | 通过（含 ASan） |
| 标签一致性 | `./script/check-ohos-tags.sh` | 通过 |
| OHOS arm64 交叉编译 | `go build -tags gio_ohos ./app`（DevEco NDK） | 通过 |
| 文档链接/围栏 | 本地脚本 | 通过 |
| gofmt | `gofmt -l app/ohos*.go` | 通过 |

### 基准（M4 Max，Go 1.27.1）

| 基准 | 结果 |
|---|---|
| `BenchmarkOHOSQueuePush` | ~8.1 ns/op，0 alloc |
| `BenchmarkOHOSQueuePushFull` | ~604 ns/op，0 alloc |
| `BenchmarkOHOSQueueTakeRecycle` | ~8.3 ns/op，0 alloc |

## 3. 真机 Gate（未执行）

以下必须在 OHOS 设备 + shim v3 上执行，当前环境无设备：

- 空闲 10s 0 帧；Invalidate 1 帧；动画连续；后台 0 帧。
- 销毁/重建无泄漏；迟到事件无效。
- 全屏/窗口化/标题/系统栏/方向/置顶/关闭。
- 拼音/候选/emoji/选区/删除/回车；无重复字符。
- 触摸/鼠标/触控板/外接键盘；混合设备捏合。
- 旋转/字体缩放/软键盘 Insets。
- 文件 picker/拖放；二进制剪贴板。
- 读屏播报/点击（ArkUI 虚拟节点能力待验证）。

## 4. 已知限制 / 后续

| 项 | 状态 | 说明 |
|---|---|---|
| v1 shim 兼容 | 限制 | v1 host 结构更小，`install_host` 仍按 v2 尺寸读取；v1 shim 必须升级或改用 v3 入口 |
| shim v3 实现 | 待做 | 仓库外 `libgpofdohos`；Go 侧已按判空降级 |
| 无障碍完整虚拟树 | 待验证 | 当前 JSON/announce/action 已就绪，ArkUI 支持需真机确认 |
| 多窗口 | 未支持 | `ohosState` 单窗口；需 ABI v4 window id |
| 权限（相机/麦克风/存储） | Go 侧已支持 | `request_permission` 已实现；shim 需对接系统弹窗 |
| Vulkan | 未启用 | 当前 EGL/GLES3 |
| 特性开关 | 部分实现 | `GIO_OHOS_SYNC_FRAME`、`GIO_OHOS_NO_A11Y` 已实现；`GIO_OHOS_IME_LEGACY` 未实现 |

## 5. 回滚

每个阶段是独立提交，可按 S0→S10 逆序 revert；共享层改动在 S1 的 PR-04 中，可单独回滚。ABI v3 对 v2 shim 无行为影响（回调判空）。

## 6. 签署

- Go owner：自动化 Gate 通过
- shim owner：待 v3 实现后真机签署
- QA：真机 Gate 待执行

## 7. Gate 后增量

| 提交 | 内容 |
|---|---|
| `6ad87df3` | 运行时权限桥（`OHOSRequestPermission` + `request_permission`/`permission_result`）；`GIO_OHOS_SYNC_FRAME`、`GIO_OHOS_NO_A11Y` 开关 |

新增自动化覆盖：权限投递/重复结果/不支持降级、flags 解析、同步帧回退。真机权限弹窗仍需 shim 对接。

仍待 ABI/平台支持：多窗口（需 window id，ABI v4）、Vulkan surface、完整无障碍虚拟树、v1 shim 尺寸安全、`GIO_OHOS_IME_LEGACY`。
