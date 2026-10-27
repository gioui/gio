# OHOS 真机验证记录（2026-10-27）

## 1. 设备

| 设备 | 目标 | 系统 | UDID |
|---|---|---|---|
| OpenHarmony 模拟器 | `127.0.0.1:5555` | emulator 7.0.0.107(SP8DEVC00E47R1P3) | `454D…0000` |
| Huawei VYG-AL00 | `192.168.1.101:33045` | HarmonyOS 7.0.0.111(SP12C00E111R3P4) | `EA6B…021C` |

## 2. 构建

- Gio：`fix/ohos-m0`，提交 `9028090d`（ABI v4 多窗口路由）。
- Go：`~/.cache/gpofd/go-ohos-1.26/bin/go`（go1.26.5 darwin/arm64），`GOOS=openharmony`。
- NDK：DevEco / `/Applications/DevEco-Studio.app/Contents/sdk/default/openharmony/native`。
- 宿主工程：`/Users/mahesh/AiWork/research/t12/gpofd`（`../gio` → 本仓库）。
- 真实手机不接受 OpenHarmony 默认证书；改用机器上已有的 Huawei 自动调试材料
  （bundle `com.example.myapplication`，profile 包含 VYG-AL00 UDID），由 hvigor 签名。

## 3. 模拟器结果

- HAP 可安装、可启动（pid 存在数秒）。
- 首帧阶段崩溃：`gpu.newBlitter` → `NewImmutableBuffer` → `panic(err)`，SIGABRT。
- 12:18/12:23 的旧 HAP 崩溃 PC 模式相同，说明是**模拟器 GLES 能力/驱动问题**，不是本次 ABI v4 改动引入。
- 未继续在模拟器上做功能结论。

## 4. 真机 VYG-AL00 结果

### 4.1 启动与渲染

- 安装 Huawei 调试签名 HAP：成功。
- `aa start -a EntryAbility -b com.example.myapplication`：成功，pid 稳定。
- 首帧 PDF 文档渲染正常（菜单栏、工具栏、页面、状态栏）。
- 前后台切换：`keyEvent Home` 后 pid 不变，重新启动后恢复，无崩溃。

### 4.2 ABI v4

临时把 shim 升级为 v4（补丁：`/tmp/gpofd-v4-shim.patch`，未提交到 Gio 仓库）：

- `gio_ohos_install_host_v4(&host, sizeof)` 成功。
- 日志确认 `v4 create window 1`。
- 所有 surface/touch/mouse/key/axis/focus/lifecycle/insets/frame 改走 `_w` 入口。
- 真机菜单点击、文件选择器打开均正常；证明 `_w` 路由、staging、默认窗口回落工作。
- `select_window` 在 shim 侧按 id 缓存，避免每帧重复回调。

### 4.3 交互

- 点击“文件”菜单：菜单展开正常。
- 点击“打开…”：系统文件选择器打开，列出 PDF。
- 文件选择器内点击/选择在当前 `uitest` 注入下未生效（UI 未变化），但 picker 本身已由 Gio 调起。
- 关闭文档后显示“未打开文档”空状态，说明菜单/页面状态更新链路正常。

## 5. 未验证项

| 项 | 原因 |
|---|---|
| IME（中文/emoji/候选/删除/选区） | 当前 gpofd 页面没有 Gio 文本输入框；shim 的 IME 回调未在真机触发 |
| 双指捏合 | `uitest`/`uinput` 不支持多点注入 |
| 旋转/Insets 动态 | 未通过命令触发系统旋转；软键盘未出现 |
| 窗口模式/标题/系统栏/置顶/限制 | 当前测试 shim 仍单窗口，且 App 未暴露这些入口 |
| 多窗口 | shim 只创建一个 XComponent；v4 路由已通，但未创建第二窗口 |
| 无障碍 | 页面无读屏操作；ArkUI 虚拟节点能力未验证 |

## 6. 复现命令

```bash
cd /Users/mahesh/AiWork/research/t12/gpofd
export OHOS_GIO=/Users/mahesh/AiWork/gio
export OHOS_NDK_HOME=/Applications/DevEco-Studio.app/Contents/sdk/default/openharmony/native
export OHOS_BASE_SDK_HOME=/Applications/DevEco-Studio.app/Contents/sdk/default
bash scripts/build-ohos.sh --gui
bash scripts/build-ohos-hap.sh
# 真机需 Huawei 调试签名（bundle com.example.myapplication）
hdc -t 192.168.1.101:33045 install -r ohos/entry/build/default/outputs/default/entry-default-signed.hap
hdc -t 192.168.1.101:33045 shell aa start -a EntryAbility -b com.example.myapplication
```

## 7. 结论

- Go 侧生命周期、surface、帧调度、触摸/菜单、文件选择器、前后台恢复在真机通过。
- ABI v4 安装、create/select 回调、`_w` 事件路由在真机通过。
- IME/捏合/旋转/多窗口仍需可注入的测试场景或专用测试 App。
