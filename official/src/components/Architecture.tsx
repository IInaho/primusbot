"use client";

import { motion, useReducedMotion } from "motion/react";

const layers = [
  { name: "TUI / GUI", desc: "Bubble Tea · Wails v2 · React 18", color: "var(--accent-cyan)" },
  { name: "Bot Engine", desc: "Agent Loop · Tools · Hooks · Policy", color: "var(--accent)" },
  { name: "Extension", desc: "Plugin · MCP · Skill · Sub-Agent", color: "var(--accent-warm)" },
  { name: "Infrastructure", desc: "SQLite · Tree-sitter · LRU · Config", color: "var(--muted)" },
];

export function Architecture() {
  const reduce = useReducedMotion();

  return (
    <section id="architecture" className="relative z-10 py-32 px-6">
      <div className="max-w-7xl mx-auto">
        <motion.div
          initial={reduce ? false : { opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, amount: 0.3 }}
          transition={{ duration: 0.6 }}
          className="text-center mb-16"
        >
          <h2 className="text-3xl md:text-4xl font-bold tracking-tight">
            Clean <span className="text-[var(--accent-cyan)]">architecture</span>
          </h2>
          <p className="mt-4 text-[var(--muted)] text-lg max-w-2xl mx-auto">
            纯 Go 单二进制，零 CGO 依赖。模块化分层，窄接口通信。
          </p>
        </motion.div>

        <div className="max-w-2xl mx-auto space-y-3">
          {layers.map((layer, i) => (
            <motion.div
              key={layer.name}
              initial={reduce ? false : { opacity: 0, x: i % 2 === 0 ? -30 : 30 }}
              whileInView={{ opacity: 1, x: 0 }}
              viewport={{ once: true, amount: 0.5 }}
              transition={{ duration: 0.6, delay: i * 0.1, ease: [0.16, 1, 0.3, 1] }}
              className="relative rounded-xl border border-[var(--border)] bg-[var(--surface)] p-5 overflow-hidden"
            >
              <div className="flex items-center justify-between gap-4">
                <div>
                  <div className="flex items-center gap-3">
                    <div
                      className="w-3 h-3 rounded-full"
                      style={{ backgroundColor: layer.color }}
                    />
                    <h3 className="font-semibold text-[var(--foreground)]">
                      {layer.name}
                    </h3>
                  </div>
                  <p className="mt-1 text-sm text-[var(--muted)] font-mono">
                    {layer.desc}
                  </p>
                </div>
              </div>
              <div
                className="absolute left-0 top-0 bottom-0 w-1"
                style={{ backgroundColor: layer.color }}
              />
            </motion.div>
          ))}
        </div>

        {/* Tech badges */}
        <motion.div
          initial={reduce ? false : { opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.6, delay: 0.4 }}
          className="mt-12 flex flex-wrap justify-center gap-3"
        >
          {["Go 1.25+", "SQLite (zero CGO)", "Tree-sitter", "Bubble Tea", "Wails v2", "React 18", "Tailwind CSS 4"].map(
            (tech) => (
              <span
                key={tech}
                className="px-3 py-1.5 rounded-full text-xs font-mono border border-[var(--border)] bg-[var(--surface)] text-[var(--muted)]"
              >
                {tech}
              </span>
            )
          )}
        </motion.div>
      </div>
    </section>
  );
}
