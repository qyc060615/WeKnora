# NebulaTech 星云科技产品使用手册

## 产品套餐

### Basic

Basic 套餐每个 Workspace 最多包含 10 名成员，适合小型团队。Basic 单次文件导入上限为 50 MB，并使用基础 API 限额。

### Pro

Pro 套餐每个 Workspace 最多包含 100 名成员。Pro 单次文件导入上限为 200 MB，并提供审计日志查看能力。

### Enterprise

Enterprise 套餐的 Workspace 成员数不设产品内硬上限。Enterprise 单次文件导入上限为 1 GB，并支持 SSO、细粒度权限和更高 API 限额。

## Workspace

### Workspace 隔离

Workspace 是产品中的最高级业务数据隔离边界。一个用户可以加入多个 Workspace，但不同 Workspace 的数据、成员和 API Key 默认互不可见。

### Workspace 创建

Basic 和 Pro 客户由账号 Owner 创建首个 Workspace；Enterprise 客户的首个 Workspace 由实施流程创建。后续 Workspace 只能由当前 Workspace 的 Admin 创建。

## 用户权限

### Viewer

Viewer 可以查看有权限的数据集和仪表盘，但不能修改数据、导入文件、导出数据或管理成员。

### Editor

Editor 可以查看和修改数据集，并可以执行文件导入；Editor 不能管理成员，也不能执行 Workspace 级数据导出。

### Admin

Admin 具有成员管理、权限配置、文件导入和 Workspace 级数据导出权限。删除 Workspace 还需要账号 Owner 二次确认，仅有 Admin 权限不足以单独删除 Workspace。

## 数据导入

### 支持格式与大小

文件导入支持 CSV、XLSX 和 JSON Lines。单个文件大小上限由套餐决定：Basic 50 MB、Pro 200 MB、Enterprise 1 GB。超过套餐上限的文件在上传阶段直接拒绝，不进入解析队列。

### 导入权限

Editor 和 Admin 可以执行数据导入，Viewer 不可以。导入任务生成唯一 Import ID，可在任务页查看成功行数、失败行数和错误原因。

## 数据导出

### 导出权限

Workspace 级数据导出只能由 Admin 发起。导出文件生成后有效下载时间为 24 小时，超过 24 小时链接失效，需要重新发起导出。

## 审计

Pro 和 Enterprise 的成员变更、权限变更、导入和导出操作写入 Workspace 审计日志，日志保留 180 天。

## 套餐与权限边界

套餐能力和 Workspace 角色解决的是两个不同维度的问题。Basic、Pro、Enterprise 决定成员规模、导入容量和平台能力范围；Viewer、Editor、Admin 决定同一 Workspace 内某个用户能执行哪些操作。升级套餐不会自动把普通成员变成 Admin，赋予 Admin 也不会绕过套餐本身的容量限制。

Workspace 隔离意味着跨 Workspace 操作需要分别获得目标 Workspace 的权限。用户在 Workspace A 是 Admin，并不因此在 Workspace B 拥有任何权限。API Key 也遵守所属 Workspace 的边界，不能用一个 Workspace 的 Key 直接读取另一个 Workspace 的数据。

数据导入和数据导出是不同权限动作。Editor 能够导入并修改数据，但不能因为拥有写权限就执行 Workspace 级导出；导出是更高风险的数据离开平台动作，所以由 Admin 发起。Viewer 只承担读取角色，不能通过前端或 API 绕过角色限制执行导入。

导出链接失效只表示已有下载链接不可继续使用，不代表原数据被删除。需要再次下载时必须重新发起一次新的导出任务。

权限判断应始终以“目标 Workspace + 当前角色 + 当前套餐”三个条件共同解释。只看其中一个条件容易产生错误结论：例如用户拥有 Editor 角色，只能说明其角色允许导入，实际文件仍必须满足所属套餐的单文件大小上限。产品文档中的这些边界用于让客户端和服务端采用同一套判定逻辑。
