# Regression Real CI Evidence Manifest

本目录仅保存 Topic 3 Regression 真实 CI 的局部证据，不作为最终验收总结。最终汇报口径以仓库根目录 `TOPIC3-README.md` 为准。

## Normal PASS

- Branch: `feat/topic3-evaluation`
- Commit: `2a7607098c05f32843c4bcd59766ee3d5ddf2eff`
- PR: [#1](https://github.com/qyc060615/WeKnora/pull/1)
- Workflow run: [34198435447](https://github.com/qyc060615/WeKnora/actions/runs/34198435447)
- Job: `Benchmark quality regression gate`
- Result: `PASS`
- Machine-readable evidence: [`normal_pass/github-evidence.json`](normal_pass/github-evidence.json)
- Screenshot: [`docs/images/topic3-ci-pass.png`](../../../../docs/images/topic3-ci-pass.png)

## Retrieval Degradation FAIL

- Temporary branch: `test/recall-regression-gate`
- Parent commit: `2a7607098c05f32843c4bcd59766ee3d5ddf2eff`
- Degradation commit: `b901fd309c2824cb38202e331c4eedc132f7a063`
- PR: [#2](https://github.com/qyc060615/WeKnora/pull/2)
- Workflow run: [34222435194](https://github.com/qyc060615/WeKnora/actions/runs/34222435194)
- Failed job: [Benchmark quality regression gate](https://github.com/qyc060615/WeKnora/actions/runs/34222435194/job/102048492139)
- Result: `FAIL` (`exit status 1`)
- Machine-readable run evidence: [`retrieval_degradation_fail/github-evidence.json`](retrieval_degradation_fail/github-evidence.json)
- Machine-readable comparator report: [`retrieval_degradation_fail/reported-regression-report.json`](retrieval_degradation_fail/reported-regression-report.json)
- Screenshot: [`docs/images/topic3-ci-regression-fail.png`](../../../../docs/images/topic3-ci-regression-fail.png)

The temporary degradation branch and commit are evidence inputs only and are not part of the final feature branch.
