"use client";

import { motion, useReducedMotion } from "motion/react";

const ecosystem = [
  {
    title: "16+ Built-in Tools",
    items: ["read / write / edit", "glob / grep / diff", "bash / web_search", "task / todo / index"],
  },
  {
    title: "Hook System",
    items: ["8 event points", "12 built-in hooks", "JS runner", "Plugin declaration"],
  },
  {
    title: "Context Engine",
    items: ["5-level compression", "Auto discovery", "Memory injection", "Token budget"],
  },
];

export function Ecosystem() {
  const reduce = useReducedMotion();

  return (
    <section id="ecosystem" className="relative z-10 py-32 px-6">
      <div className="max-w-7xl mx-auto">
        <motion.div
          initial={reduce ? false : { opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, amount: 0.3 }}
          transition={{ duration: 0.6 }}
          className="text-center mb-16"
        >
          <h2 className="text-3xl md:text-4xl font-bold tracking-tight">
            Extensible <span className="text-[var(--accent-warm)]">ecosystem</span>
          </h2>
          <p className="mt-4 text-[var(--muted)] text-lg max-w-2xl mx-auto">
            工具、Hook、上下文，每一层都可以扩展。
          </p>
        </motion.div>

        <div className="grid md:grid-cols-3 gap-6">
          {ecosystem.map((group, i) => (
            <motion.div
              key={group.title}
              initial={reduce ? false : { opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, amount: 0.3 }}
              transition={{ duration: 0.6, delay: i * 0.12 }}
              className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6"
            >
              <h3 className="font-semibold text-lg mb-4 text-[var(--foreground)]">
                {group.title}
              </h3>
              <div className="space-y-2.5">
                {group.items.map((item) => (
                  <div
                    key={item}
                    className="flex items-center gap-2 text-sm text-[var(--muted)] font-mono"
                  >
                    <span className="w-1.5 h-1.5 rounded-full bg-[var(--accent)]/60 shrink-0" />
                    {item}
                  </div>
                ))}
              </div>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
