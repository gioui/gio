# OHOS 无障碍最小闭环

> 实现位置：[app/ohos_semantics.go](../app/ohos_semantics.go)、[app/os_ohos.go](../app/os_ohos.go)、[app/window.go](../app/window.go)。
> 状态：Go 侧已完成；shim 侧需把 JSON 映射到 ArkUI 可访问节点或播报 API。

## 1. 数据流

```
Window.processFrame -> updateState -> semanticsDriver.SemanticsChanged
  -> SemanticRoot / AppendSemanticDiffs / LookupSemantic
  -> ohosSemanticsJSON(root, nodes, removed)
  -> host.semantics_update(json, len)
```

- 首帧发送整棵树（root + 所有后代）。
- 之后只发送 diff 节点与 removed ID。
- 无变化时不调用 host。

## 2. JSON schema

```json
{
  "root": 1,
  "nodes": [
    {
      "id": 2,
      "parent": 1,
      "class": "button",
      "label": "OK",
      "desc": "",
      "selected": false,
      "disabled": false,
      "bounds": [10, 20, 90, 50],
      "actions": ["click"]
    }
  ],
  "removed": [3]
}
```

- `bounds`：surface 像素，[minX,minY,maxX,maxY]。
- `class`：button/checkbox/editor/radio/switch，未知为空。
- `actions`：click/scroll。

## 3. 读屏动作

shim 调：

```c
void gio_ohos_semantic_action(int nodeID, int action);
```

- 0 click：在节点中心合成 Press/Release。
- 1 scroll forward / 2 scroll backward：合成 Scroll。

## 4. 播报

Go API：

```go
app.OHOSAnnounce("已保存")
```

底层调用 host `semantics_announce(text)`。

## 5. 平台限制与验证

ArkUI 对自绘 XComponent 的虚拟子节点支持需要按目标 API 版本验证。若不可用：

- 至少保留 `semantics_announce` 与命中区域 click；
- 在发布说明标注"完整虚拟树待平台支持"；
- 不要把无法映射的节点静默丢弃，shim 应记录一次诊断。

验证：

- 单测：JSON schema/golden、diff/removed、action 路由。
- 真机：读屏焦点顺序、按钮播报与点击、编辑器标签、列表项。
