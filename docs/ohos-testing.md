# OHOS 测试指南

> 当前分支 `fix/ohos-m0`，已实现 S0–S10（生命周期/帧/ABI/窗口/IME/输入/环境/文件/文档/无障碍）。Gate 结果见 [ohos-gate-m2.md](ohos-gate-m2.md)。
> 真机 Gate 仍需在 OHOS 设备上执行（本机无设备）。

## 1. Host 测试（无需 OHOS SDK）

```bash
export PATH=/opt/homebrew/Cellar/go/1.27.1/libexec/bin:$PATH
go test ./app -tags gio_ohos_test
go test -race ./app -tags gio_ohos_test
go test ./...
```

覆盖：

- 队列合并/限容/回收（`ohos_queue_test.go`）
- 帧调度/Invalidate/Size（`ohos_pump_test.go`）
- 配置/动作/fakeHost（`ohos_config_test.go`）
- IME 坐标/去重（`ohos_ime_bridge_test.go`）
- 输入表/鼠标/捏合/去重（`ohos_input_test.go`）
- 文件/剪贴板（`ohos_file_test.go`）

## 2. C ABI 契约测试

```bash
./script/test-ohos-abi.sh
```

- 用 host cc 编译 `ohos_glue.c`（`GIO_OHOS_HOST_TEST` 跳过 NDK 头）。
- 有 ASan 时自动启用，检查 v2 尺寸经 v3 入口安装时的越界读。
- 覆盖：v2/v3 安装、版本查询、NULL、offset 检查。

## 3. OHOS 交叉编译

需要 DevEco Studio 的 native SDK：

```bash
export PATH=/opt/homebrew/Cellar/go/1.27.1/libexec/bin:$PATH
SDK=/Applications/DevEco-Studio.app/Contents/sdk/default/openharmony/native
export GOOS=linux GOARCH=arm64 CGO_ENABLED=1
export CC=$SDK/llvm/bin/aarch64-unknown-linux-ohos-clang
export CXX=$SDK/llvm/bin/aarch64-unknown-linux-ohos-clang++
export CGO_CFLAGS="--sysroot=$SDK/sysroot -I$SDK/sysroot/usr/include"
export CGO_LDFLAGS="--sysroot=$SDK/sysroot -L$SDK/sysroot/usr/lib/aarch64-linux-ohos"
go build -tags gio_ohos ./app
```

> 本机默认 `go` 可能是 1.24.5 驱动 + 1.27.1 标准库的混合体，必须显式使用 1.27.1。

## 4. 构建标签检查

```bash
./script/check-ohos-tags.sh
```

检查 `gio_ohos`/`openharmony` 正负标签成对出现，避免新增平台时漏改。

## 5. 真机矩阵（Gate 必跑）

| 类别 | 场景 |
|---|---|
| 启动/销毁 | 冷启动、后台/前台、销毁、再次创建 |
| 帧 | 空闲 10s 0 帧；Invalidate 1 帧；动画连续；后台 0 帧 |
| 输入 | 触摸、鼠标、触控板捏合、外接键盘、Tab 焦点 |
| IME | 拼音、候选、上屏、删除、移动、选区、emoji、撤销 |
| 窗口 | 全屏/窗口化、标题、系统栏颜色、方向、置顶、关闭 |
| 环境 | 旋转、字体缩放、软键盘 Insets |
| 文件 | picker 单选/多选、拖放、含空格/中文路径 |
| 剪贴板 | 文本、含 NUL 二进制 |
| 稳定性 | 1 小时输入洪泛、反复旋转/前后台、无泄漏 |

## 6. 当前验证结果

| 项 | 结果 |
|---|---|
| `go test ./app -tags gio_ohos_test` | 通过 |
| `-race` 同上 | 通过 |
| `go test ./...` | 通过 |
| `script/test-ohos-abi.sh` | 通过 |
| `script/check-ohos-tags.sh` | 通过 |
| OHOS arm64 交叉编译 | 通过 |
| 真机 Gate | **部分执行**：VYG-AL00 启动/输入/文件选择器/ABI v4 通过；见 [ohos-device-2026-10-27.md](ohos-device-2026-10-27.md) |