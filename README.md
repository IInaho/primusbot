# NekoCode

<p align="center">
  <b>你的猫娘 AI 编程助手 &nbsp;·&nbsp; MIT 开源 &nbsp;·&nbsp; 一个文件就能跑</b>
</p>

<p align="center">
  <sub>双界面 · 多模型自由切换 · 读代码 / 改文件 / 跑命令，全靠聊</sub>
</p>

<p align="center">
  <img src="docs/images/demo.gif" width="90%" alt="NekoCode 演示">
</p>

<p align="center">
  <b>简体中文</b> · <a href="README.en.md">English</a>
</p>

---

## 这是什么？

NekoCode 目前的定位是 **AI 编程助手**，你只需要 **像聊天一样说出你的需求**，它就能帮你：

- **看懂项目**：快速理解陌生代码库，讲清楚某段逻辑在做什么
- **改代码**：加功能、修 bug、重构，并检查格式和语法
- **跑命令**：执行构建、测试等命令，持续返回运行状态
- **查资料**：联网搜索、抓取网页，补充需要实时确认的信息
- **生成图片**：通过已配置的图片模型生成示意图或封面图

**为什么选它？**

- **远程交互**：支持 Telegram、飞书和 QQ，可在离开电脑时提交任务并接收结果
- **单二进制分发**：TUI 以单个可执行文件发布；Linux 版本需要兼容的 glibc 环境
- **Claude Code 风格扩展**：支持其插件和 Skill 格式的兼容子集，具体能力以文档和测试为准
- **二次开发**：`bot → runtime → interaction` 分层，可嵌入其他应用

它不锁定某一家模型厂商。Anthropic、DeepSeek、Kimi、GLM 等模型可以通过已实现的 OpenAI 或 Anthropic 协议能力接入。


## 快速开始

### 第一步：一键安装

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/lznauy/NekoCode/master/scripts/install.sh | sh
```

> 脚本自动识别系统（Linux / macOS）和架构，安装到 `~/.local/bin`。
> 脚本会用 Release 中的 `SHA256SUMS` 校验下载的二进制；v0.4.2 使用安装器内置的已核验 SHA-256。
> Windows 用户请通过 [WSL](https://learn.microsoft.com/windows/wsl/install) 运行。

<details>
<summary>其他安装方式（手动下载 / 源码编译）</summary>

从 [Releases 页面](https://github.com/lznauy/NekoCode/releases) 下载对应平台的二进制，加执行权限直接运行。

或源码编译（需要 [Go 1.25.12+](https://go.dev/dl/)）：

```bash
git clone https://github.com/lznauy/NekoCode.git
cd NekoCode
go build -o nekocode-tui ./cmd/tui
```

</details>

### 第二步：配置 API Key

```bash
mkdir -p ~/.nekocode
cat > ~/.nekocode/config.json << 'EOF'
{
  "active": "deepseek",
  "models": [
    {
      "name": "deepseek",
      "provider": "deepseek",
      "api_key": "sk-你的Key写这里",
      "model": "deepseek-v4-flash",
      "base_url": "https://api.deepseek.com/v1",
      "protocol": "openai"
    }
  ]
}
EOF
```

### 第三步：开聊

```bash
nekocode-tui
```

配置完成后即可开始使用。

## 了解更多

- 详细使用、配置指南，可以参考文档 [用户使用指南](docs/USER_GUIDE.md)
- 桌面窗口版（GUI）暂未随 Release 发布，目前尚不完善，还在开发中

NekoCode 仍处于测试阶段，尚未经过大规模生产验证。工具设计、前缀缓存、上下文压缩和治理策略记录在开发者文档中。

## 给开发者

- [ARCHITECTURE.md](docs/ARCHITECTURE.md)：整体架构、Agent 循环、工具系统、技术细节
- [RUNTIME_APP_GUIDE.md](docs/RUNTIME_APP_GUIDE.md)：基于底座组装上层 AI 应用
- [RUNTIME_HTTP_API.md](docs/RUNTIME_HTTP_API.md)：Runtime HTTP/SSE 协议

## 开源

- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)
- [支持范围](SUPPORT.md)

NekoCode 使用 [MIT License](LICENSE)，可以按许可证条款使用、修改和分发。
