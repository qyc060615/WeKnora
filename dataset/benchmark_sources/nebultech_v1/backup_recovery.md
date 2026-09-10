# NebulaTech 星云科技备份与恢复制度

## 适用范围

本制度覆盖核心 SaaS Production 数据库、Staging 数据库以及长期合规归档。Development 环境不提供灾难恢复承诺，开发者应依赖代码仓库和可重建测试数据。

## Production Backup

### 备份策略

Production 数据库每天 02:00 执行一次全量 Backup，并持续采集增量日志。增量日志上传间隔不得超过 15 分钟。全量 Backup 与增量日志存放在与生产集群不同的存储账号中。

### RPO

Production 数据库的恢复点目标 RPO 为 15 分钟。任何灾难恢复方案都必须能够把可接受的数据丢失窗口控制在 15 分钟以内。

### RTO

Production 数据库的恢复时间目标 RTO 为 2 小时，从 Incident Commander 宣布启动灾难恢复流程时开始计时，到核心业务完成读写验证时结束。

## Staging Snapshot

### 快照策略

Staging 数据库每天 03:00 创建一次 Snapshot。Staging 的 RPO 为 24 小时，RTO 为 8 小时。Staging Snapshot 仅用于测试环境恢复，不得替代 Production Backup。

### Snapshot 保留周期

Staging Snapshot 保留 7 个自然日，到期自动删除。因测试需要保留更久时，应导出为 Archive，而不是延长 Snapshot 生命周期。

## Archive

### 月度归档

每月 1 日生成上一个自然月最后一份 Production 全量 Backup 的只读 Archive。Archive 用于合规与历史取证，不用于承诺 2 小时内的在线恢复。

### Archive 保留周期

月度 Archive 保留 365 天。到期后由归档系统自动删除，存在法律保全标记的 Archive 除外。

## Backup 保留周期

Production 每日全量 Backup 保留 35 个自然日；与其关联的增量日志至少保留到对应全量 Backup 到期。Backup、Snapshot 和 Archive 是三种不同对象，保留周期不得混用。

## 恢复流程

### Production 恢复步骤

Production 灾难恢复依次执行：第一，Incident Commander 宣布进入恢复模式；第二，恢复最近可用全量 Backup；第三，重放增量日志到目标恢复点；第四，由 DBA 执行数据一致性校验；第五，由当周值班 SRE 执行业务读写冒烟测试；第六，Incident Commander 宣布恢复完成。

### 恢复演练

Production 恢复演练每季度至少执行 1 次。演练必须记录实际 RPO、实际 RTO、失败步骤和改进项，记录保留 365 天。

## 备份对象边界

Backup、Snapshot 和 Archive 的名称相似，但目标不同。Production Backup 服务于生产灾难恢复；Staging Snapshot 服务于测试环境快速回退；Archive 服务于较长期的合规和历史取证。三类对象不能因为都保存了数据副本就互相替代对应的恢复目标。

RPO 描述可接受的数据丢失时间窗口，RTO 描述从启动恢复到业务恢复验证所允许的时间。Production 的增量日志用于缩短恢复点距离，因此全量 Backup 的生成时间不等于最终可恢复时间点。恢复时需要把基础全量数据与后续增量日志组合起来。

Staging 的恢复承诺明显弱于 Production，因此测试环境快照不能作为生产灾难恢复证据。Archive 虽然保存时间更长，但其设计目标也不是快速在线恢复。评估恢复能力时应使用与目标环境相匹配的备份对象。

恢复演练用于验证备份文件“真的能恢复”，而不只是确认备份任务显示成功。演练记录应能够反映恢复流程中的失败位置和实际达成的恢复目标。
