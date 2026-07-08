"use client";

import { motion, useReducedMotion } from "motion/react";

const features = [
  {
    icon: "🧠",
    title: "Agent Loop",
    description:
      "Reason, Execute, Feedback 三轮循环。并行工具调度，自动判断依赖，子 Agent 委派。",
  },
  {
    icon: "🛡️",
    title: "Sandbox Security",
    description:
      "Linux namespace 六重隔离 + Landlock 文件写保护。sudo、ssh、dd 直接拒绝。",
  },
  {
    icon: "🔌",
    title: "Plugin / MCP / Skill",
    description:
      "GitHub 一键安装插件，JSON-RPC 2.0 MCP 服务器接入，YAML 技能包一键加载。",
  },
  {
    icon: "💾",
    title: "Session Memory",
    description:
      "五级智能压缩，项目上下文自动发现，手动维护记忆自动注入，对话永不丢失。",
  },
  {
    icon: "🎨",
    title: "Dual Frontend",
    description:
      "TUI 终端原生体验 (Bubble Tea) + GUI 桌面窗口 (Wails v2 + React)，共享核心引擎。",
  },
  {
    icon: "🔀",
    title: "Multi-Model",
    description:
      "Anthropic 原生 + OpenAI 兼容协议。DeepSeek、MiniMax 等，一个 /model 切换。",
  },
];

const containerVariants = {
  hidden: {},
  visible: {
    transition: { staggerChildren: 0.08 },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 24 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.6, ease: [0.16, 1, 0.3, 1] as const },
  },
};

export function Features() {
  const reduce = useReducedMotion();

  return (
    <section id="features" className="relative z-10 py-32 px-6">
      <div className="max-w-7xl mx-auto">
        <motion.div
          initial={reduce ? false : { opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, amount: 0.3 }}
          transition={{ duration: 0.6 }}
          className="text-center mb-16"
        >
          <h2 className="text-3xl md:text-4xl font-bold tracking-tight">
            Built for <span className="text-[var(--accent)]">real work</span>
          </h2>
          <p className="mt-4 text-[var(--muted)] text-lg max-w-2xl mx-auto">
            不是 demo，是每天写代码都会用到的工具。
          </p>
        </motion.div>

        <motion.div
          variants={containerVariants}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, amount: 0.1 }}
          className="grid md:grid-cols-2 lg:grid-cols-3 gap-5"
        >
          {features.map((feature) => (
            <motion.div
              key={feature.title}
              variants={itemVariants}
              className="group relative rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 hover:border-[var(--accent)]/40 transition-colors"
            >
              <div className="text-3xl mb-4">{feature.icon}</div>
              <h3 className="text-lg font-semibold mb-2">{feature.title}</h3>
              <p className="text-sm text-[var(--muted)] leading-relaxed">
                {feature.description}
              </p>
              <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-[var(--accent)]/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
            </motion.div>
          ))}
        </motion.div>
      </div>
    </section>
  );
}
