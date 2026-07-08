# NekoCode Website

NekoCode 的官网 — 一个终端里的猫娘 AI 助手。

## Tech Stack

- Next.js 16 (App Router)
- React 19 + TypeScript
- Tailwind CSS 4
- Motion ( animations )
- Geist Font

## Development

```bash
npm install
npm run dev
```

打开 [http://localhost:3000](http://localhost:3000) 查看页面。

## Build

```bash
npm run build
npm start
```

## Project Structure

```
src/
├── app/
│   ├── globals.css      # 全局样式 + Tailwind 主题变量
│   ├── layout.tsx       # 根布局 + 字体 + SEO
│   └── page.tsx         # 首页组装
└── components/
    ├── Nav.tsx          # 顶部导航
    ├── Hero.tsx         # 首屏 + 终端动画
    ├── Features.tsx     # 六大特性卡片
    ├── Architecture.tsx # 架构分层展示
    ├── Ecosystem.tsx    # 工具/ Hook / 上下文
    ├── Footer.tsx       # 底部链接
    └── GitHubIcon.tsx   # GitHub SVG 图标
```

## License

[MIT](../../LICENSE)
