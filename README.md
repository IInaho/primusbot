# NekoCode 🐱

<p align="center">
  <b>你的猫娘 AI 编程助手 &nbsp;·&nbsp; MIT 开源 &nbsp;·&nbsp; 一个文件就能跑</b>
</p>

<p align="center">
  <sub>双界面 · 多模型自由切换 · 读代码 / 改文件 / 跑命令，全靠聊</sub>
</p>

<p align="center">
  <img src="docs/images/demo.gif" width="90%" alt="NekoCode 演示">
</p>

---

## 这是什么？

NekoCode 目前的定位是 **AI 编程助手**，你只需要 **像聊天一样说出你的需求**，它就能帮你：

- 📖 **看懂项目** — 快速理解陌生代码库，讲清楚某段逻辑在干嘛
- ✏️ **改代码** — 加功能、修 bug、重构，改完自动检查格式和语法
- 🖥️ **跑命令** — 执行构建、测试等命令，替你盯着输出
- 🔍 **查资料** — 联网搜索、抓取网页，不懂的知识现查现用
- 🎨 **生成图片** — 需要示意图、封面图，直接让 AI 画一张

**为什么选它？**

- **手机上也能干活** — 连接 Telegram / 飞书 / QQ，未来可支持更多IM平台，不在电脑前也能派任务、收结果
- **一个文件，零负担** — GO单二进制零依赖，静态编译，天然支持Mac与各linux发行版
- **兼容 Claude Code 生态** — 插件和技能格式与 Claude Code 兼容，不需要额外适配成本
- **二次开发** — 精简的bot核心， bot->runtime->ui层层解耦， 可以集成到任意系统中

它不锁死任何一家大模型——Anthropic、DeepSeek、Kimi、GLM 等，只要是 OpenAI 或 Anthropic 协议都能用。


## 快速开始

### 第一步：一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/lznauy/NekoCode/master/scripts/install.sh | sh
```

> 脚本自动识别系统（Linux / macOS）和架构，安装到 `~/.local/bin`。
> Windows 用户：请通过 [WSL](https://learn.microsoft.com/windows/wsl/install) 运行

<details>
<summary>其他安装方式（手动下载 / 源码编译）</summary>

从 [Releases 页面](https://github.com/lznauy/NekoCode/releases) 下载对应平台的二进制，加执行权限直接运行。

或源码编译（需要 [Go 1.25.8+](https://go.dev/dl/)）：

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

然后你就可以愉快的使用了~

## 了解更多

- 详细使用、配置指南，可以参考文档 [用户使用指南](docs/USER_GUIDE.md)
- 桌面窗口版（GUI）暂未随 Release 发布，目前尚不完善，还在开发中

目前nekocode项目还处于测试阶段，功能尚不完善，没有经过大规模测试和推广，如果您有幸看到这个项目，可以给个star支持一下~
如果您想了解具体的技术细节，包括工具设计、prefix-cache设计、上下文压缩、治理策略等等，可以看下面的开发者文档。

## 给开发者

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) — 整体架构、Agent 循环、工具系统、技术细节
- [RUNTIME_APP_GUIDE.md](docs/RUNTIME_APP_GUIDE.md) — 基于底座组装上层 AI 应用
- [RUNTIME_HTTP_API.md](docs/RUNTIME_HTTP_API.md) — Runtime HTTP/SSE 协议

## 开源

NekoCode 使用 **MIT 许可证**，完全开源，随意使用、修改、分发。
