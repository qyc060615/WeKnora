# NebulaTech 星云科技 On-call 值班手册

## 值班角色

每个核心服务团队每周必须安排 Primary On-call 和 Secondary On-call 各 1 人。两种角色不得由同一人同时承担。Primary 负责第一响应，Secondary 在 Primary 无法响应或需要并行排障时接管或协助。

## On-call 排班

### 周期与交接

On-call 以一周为一个周期，每周一 10:00 完成交接，新一周值班从周一 10:00 生效，到下一周一 10:00 结束。排班表至少提前 14 个自然日在 NebulaPager 中发布。

### 换班

值班人员需要换班时，必须在 NebulaPager 中由原值班人和接班人双方确认。聊天中口头约定不改变系统排班，未在系统确认前仍由原值班人承担响应责任。

## 告警确认

### 确认时限

P0 告警必须在 5 分钟内确认，P1 在 10 分钟内确认，P2 在 30 分钟内确认。P3 不进入 7×24 电话升级链，但仍需在 4 小时内由责任团队确认。

## 告警升级

### Primary 到 Secondary

P0 或 P1 告警发送给 Primary 后，如果 5 分钟内没有确认，NebulaPager 自动升级给 Secondary。该升级规则不会把 P1 的最终确认时限从 10 分钟延长。

### Secondary 到 Incident Manager

P0 告警升级给 Secondary 后，如果再过 5 分钟仍无人确认，系统自动呼叫当日 Incident Manager。P1 在发送给 Secondary 后 10 分钟仍无人确认时，升级给 Incident Manager。

### 服务负责人升级

P0 从首次告警起 15 分钟仍未建立事故频道时，Incident Manager 必须电话联系对应服务负责人。P1 从首次告警起 30 分钟仍未建立事故频道时执行同样升级。

## 联系方式

### 标准渠道

所有自动告警通过 NebulaPager 发送。事故协作统一使用 `#incidents` 工作区中的独立事故频道；值班排班问题使用 `#oncall-ops`。安全事件使用 `#security-incident`，不得只在普通项目群报告。

### 紧急联系人目录

服务负责人、Incident Manager、DBA 和安全负责人的电话号码维护在 NebulaPager 的 Emergency Contacts 目录中。值班人员应使用目录中的当前号码，不得依赖个人保存的旧通讯录。

## Escalation Policy

升级的目标是确保告警被确认，而不是转移责任。Primary 在 Secondary 或 Incident Manager 加入后仍应继续提供上下文，除非 Incident Manager 明确完成责任交接。任何手工静音超过 30 分钟的 P0/P1 告警都必须记录原因和批准人。

## 值班交接记录

周一交接必须记录未关闭事故、已知风险、临时告警静音和计划中的 Production 变更。交接记录保留 90 天。

## 值班边界说明

Primary、Secondary 和 Incident Manager 构成逐级升级链，但升级并不等于前一角色自动退出。Primary 最了解最初告警上下文，Secondary 可以并行排障，Incident Manager 负责在无人响应或事故组织不足时补位。只有明确完成责任交接后，原响应人才可以停止承担当前值班上下文。

告警确认时限是事故响应目标的一部分，NebulaPager 的自动升级用于减少无人响应风险。系统把 P0 与 P1 都先升级给 Secondary，但后续升级节奏不同，因此值班人员不能把某一个等级的规则机械套用到另一个等级。

排班变更必须在系统中留下双方确认，是为了确保自动告警始终发送给真实承担职责的人。个人日历、聊天备注或口头换班都不会改变 NebulaPager 的路由对象。交接记录则用于把尚未结束的风险从上一班传递给下一班。

紧急联系人目录是统一联系方式来源。人员调整或号码变更后，应更新目录而不是要求所有值班人员手工同步私人通讯录。
