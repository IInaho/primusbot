---
name: hunt
description: Diagnose errors, crashes, failing tests, regressions, and unexpected behavior before fixing them. Use when the user asks to debug, investigate, find the root cause, explain why something is broken, or fix a failure whose cause is not yet known.
---

# Hunt

先诊断，再修复。不要用试错补丁代替根因。

## 流程

1. 原样读取错误、失败输出和用户描述的入口；能安全复现时，先记录稳定的复现路径。
2. 从调用链、状态边界或最近相关变化提出一个具体、可证伪的根因假设，并指出文件、函数或条件。
3. 用最小的只读检查、日志、断言或测试验证假设；至少再找一条独立证据交叉确认。
4. 证据否定假设时完全放弃它，再提出下一个；不要保留被反证的解释。
5. 只有能用一句话说明“根因是 X，因为证据 Y”后才修改。用户只要求诊断时，到此停止。
6. 用户要求修复时，从根因处做最小改动；用原复现路径验证，并补充能防止复发的测试。

同一症状在修复后仍存在时，停止继续打补丁并重新阅读执行路径。连续三个假设失败后，报告已检查内容、已排除原因、未知项和下一步所需证据。

## 输出

先给根因或当前最强假设，再列关键证据、修复与验证。没有充分证据时明确写“尚未确认”，不要把推断写成事实。
