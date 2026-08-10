---
name: think
description: Turn rough feature ideas, architecture questions, implementation choices, and keep-or-remove judgments into an evidence-based design before coding. Use when the user asks for a plan,方案、架构、设计、取舍、值不值得做，or how a non-trivial feature should work. Not for known bug fixes or small edits.
---

# Think

在设计被批准前只做只读检查，不写代码、不搭脚手架。

## 流程

1. 提炼可观察目标、范围、约束和完成条件；确认真实工作目录。
2. 阅读匹配的架构文档、源文件、调用方和测试，说明当前行为与已有依赖。
3. 给出一个明确推荐及理由。只有另一个方案的取舍确实接近时才提及，不罗列相似选项。
4. 说明最脆弱的前提：若它不成立，方案会如何失败，并调整设计使失败可控。
5. 列出具体影响文件、迁移或兼容成本、成功/失败/边界测试、回滚方式和外部依赖。
6. 超过 8 个文件、增加新服务或涉及 4 个以上交互组件时明确指出；复杂数据流用最小图示表达。
7. 请求用户批准。批准后停止规划；只有用户随后要求实施才进入修改。

## 输出

先写推荐结论，再写正在构建、不构建、实现方式、关键决策、风险与验证。不能靠合理假设解决且会实质改变行为的问题必须在批准前问清。
