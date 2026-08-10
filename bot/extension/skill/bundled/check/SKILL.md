---
name: check
description: Review actual code changes, diffs, issues, or pull requests; find regressions, safely fix in-scope problems after implementation, and verify before delivery. Use when the user asks for review、检查代码、看看改动、PR/issue triage、合并前检查，or after a non-trivial implementation. Not for initial debugging or feature design.
---

# Check

以实际 diff 和验证结果为准，不以实施者的总结代替证据。

## 流程

1. 读取工作区状态和完整相关 diff，保留用户已有的无关改动；对照原始目标判断范围是准确、遗漏还是漂移。
2. 检查可观察行为、错误路径、权限与信任边界、并发、持久化、退出路径、生成产物和意外依赖变化。
3. 将问题分为：明确缺陷、具体风险、可选改进。每项给出位置、触发条件和影响，不把风格偏好升级成缺陷。
4. 纯 review 请求保持只读。原任务已授权实现时，可直接修复拼写、导入、格式等无歧义问题；行为、架构或安全取舍必须先征求用户意见。
5. 改动超过 5 个文件，或触及认证、权限、沙箱、并发、持久化、数据修改时，主 Agent 使用 `task(profile="explore", skills=["check"])` 做独立架构或安全复核并核验返回结论；若当前已经是被委托的子 Agent，则直接完成复核，不再委托。
6. 运行与风险匹配的测试、静态检查、构建或真实场景，检查真实退出状态。Bug 修复应覆盖原复现路径。
7. 修复后重新读取 diff 并复跑受影响验证。未运行、环境受限和既有失败不能写成通过。

## 输出

先列按严重程度排序的发现；没有发现时说明检查范围和未覆盖风险。最后报告改动文件数、范围判断、独立复核、实际验证命令及 PASS、FAIL 或 PARTIAL。
