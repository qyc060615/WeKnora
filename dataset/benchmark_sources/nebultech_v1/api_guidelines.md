# NebulaTech 星云科技 API 设计规范

## REST API

### 资源与格式

对外业务 API 使用 HTTPS REST 风格，资源路径使用复数名词，例如 `/v1/workspaces`。请求和响应正文统一使用 UTF-8 JSON。除文件上传接口外，不接受 XML 作为业务请求格式。

### HTTP 方法

GET 用于读取资源，POST 用于创建资源，PUT 用于完整替换资源，PATCH 用于部分更新，DELETE 用于删除资源。GET 请求不得产生可观察到的业务写入副作用。

## Authentication

### 用户授权

代表最终用户访问资源的 API 使用 OAuth 2.0 Bearer Access Token。Access Token 通过 `Authorization: Bearer <token>` 请求头传递，不允许放在 URL 查询参数中。

### 服务间认证

经批准的外部服务到服务集成可以使用 Workspace API Key。API Key 通过 `Authorization: ApiKey <key>` 请求头传递。API Key 的存储与轮换必须遵守 `security_policy.md`。

## Rate Limit

### 套餐限流

Basic 套餐每个 Workspace 的默认 API 限额为每分钟 120 个请求；Pro 套餐为每分钟 600 个请求；Enterprise 套餐为每分钟 3000 个请求。窗口按自然分钟计算，超限请求返回 HTTP 429。

### 限流响应

HTTP 429 响应必须包含 `Retry-After` 响应头，值为客户端最早可安全重试前的秒数。客户端不得通过并发重试绕过限流。

## Error Code

### 错误响应结构

所有 4xx 和 5xx JSON 错误响应必须包含 `code`、`message` 和 `request_id` 三个字段。`code` 是稳定的机器可读字符串，`message` 面向开发者，`request_id` 用于日志关联。

### 常用状态码

身份凭证缺失或无效返回 401；凭证有效但权限不足返回 403；资源不存在返回 404；请求参数语义校验失败返回 422；达到 Rate Limit 返回 429。

## Versioning

### 主版本

公开 API 主版本写在 URL 中，例如 `/v1/`。破坏向后兼容的字段删除、字段语义改变或资源结构变化必须发布新的主版本，不能直接修改现有主版本行为。

### 废弃周期

公开 API 主版本进入 Deprecated 状态后，至少继续提供 180 个自然日服务。废弃公告必须给出替代版本和停止服务日期。

## 可观测性

所有 API 响应必须携带 `X-Request-Id`。服务端日志中的请求 ID 必须与响应头一致，以便故障追踪。

## 设计边界与兼容性

本规范中的 Authentication、Rate Limit、Error Code 和 Versioning 分别解决不同问题。身份认证回答“调用者是谁”，权限检查回答“调用者能做什么”，限流回答“单位时间可以调用多少次”，错误码则向客户端说明失败类型。客户端不应把 401、403 和 429 当作同一种失败重试。

API Key 与 OAuth Access Token 都通过 HTTP 请求头传输，但它们对应的调用主体不同。面向最终用户的授权流程应使用 OAuth 2.0；外部服务到服务集成在获批后才可以使用 Workspace API Key。具体 Key 的保管和轮换由安全制度统一约束，API 文档不重新定义另一套生命周期。

版本号用于管理破坏性兼容变化，不意味着每次新增字段都要创建新主版本。向现有响应增加可忽略的新字段可以保持当前主版本；删除字段或改变既有字段语义则不能静默完成。进入废弃期后，旧版本仍应按公告中的停止日期继续可用。

`request_id` 同时存在于错误结构和响应头时应保持一致，使研发人员能够从客户端错误直接定位服务端日志。
