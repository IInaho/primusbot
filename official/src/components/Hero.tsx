"use client";

import { motion, useReducedMotion } from "motion/react";
import { useState, useEffect } from "react";
import { GitHubIcon } from "./GitHubIcon";

const terminalLines = [
  { prefix: "$", text: "nekocode", suffix: "help" },
  { prefix: "", text: "  Meow! I'm NekoCode, your terminal AI assistant." },
  { prefix: "", text: "  What would you like me to do today?" },
];

export function Hero() {
  const reduce = useReducedMotion();
  const [typedText, setTypedText] = useState("");
  const fullText = "refactor auth module to use JWT";

  useEffect(() => {
    if (reduce) {
      setTypedText(fullText);
      return;
    }
    let i = 0;
    const interval = setInterval(() => {
      if (i <= fullText.length) {
        setTypedText(fullText.slice(0, i));
        i++;
      } else {
        clearInterval(interval);
      }
    }, 50);
    return () => clearInterval(interval);
  }, [reduce]);

  return (
    <section className="relative z-10 min-h-[100dvh] flex items-center px-6 pt-24 pb-16">
      <div className="max-w-7xl mx-auto w-full grid lg:grid-cols-2 gap-12 lg:gap-20 items-center">
        {/* Left: Copy */}
        <motion.div
          initial={reduce ? false : { opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1] }}
        >
          <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border border-[var(--border)] bg-[var(--surface)] text-xs font-mono text-[var(--muted)] mb-6">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
            MIT Open Source · Go Single Binary
          </div>

          <h1 className="text-4xl md:text-5xl lg:text-6xl font-bold tracking-tight leading-[1.1]">
            <span className="bg-gradient-to-r from-[var(--accent)] via-[var(--accent-warm)] to-[var(--accent-cyan)] bg-clip-text text-transparent">
              NekoCode
            </span>
            <br />
            <span className="text-[var(--foreground)]">终端里的猫娘 AI</span>
          </h1>

          <p className="mt-6 text-lg text-[var(--muted)] leading-relaxed max-w-[54ch]">
            像聊天一样交代任务，它读代码、改文件、跑命令、搜资料。
            <br className="hidden md:block" />
            多模型自由切换，Agent 循环自主执行，一个 Go 二进制文件搞定一切。
          </p>

          <div className="mt-8 flex flex-wrap gap-4">
            <a
              href="https://github.com/lznauy/NekoCode"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-lg bg-[var(--accent)] text-white font-medium hover:bg-[var(--accent)]/90 transition-colors"
            >
              <GitHubIcon className="w-5 h-5" />
              GitHub
            </a>
            <a
              href="#features"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-lg border border-[var(--border)] bg-[var(--surface)] text-[var(--foreground)] font-medium hover:bg-[var(--surface-elevated)] transition-colors"
            >
              Learn more
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M7 17L17 7M17 7H7M17 7v10" />
              </svg>
            </a>
          </div>
        </motion.div>

        {/* Right: Terminal Window */}
        <motion.div
          initial={reduce ? false : { opacity: 0, y: 40, scale: 0.95 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          transition={{ duration: 0.8, delay: 0.2, ease: [0.16, 1, 0.3, 1] }}
          className="relative"
        >
          <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] overflow-hidden shadow-2xl shadow-[var(--accent)]/5">
            {/* Title bar */}
            <div className="flex items-center gap-2 px-4 py-3 border-b border-[var(--border)] bg-[var(--surface-elevated)]">
              <div className="w-3 h-3 rounded-full bg-red-500/80" />
              <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
              <div className="w-3 h-3 rounded-full bg-green-500/80" />
              <span className="ml-3 text-xs font-mono text-[var(--muted)]">
                ~/projects/myapp
              </span>
            </div>

            {/* Terminal content */}
            <div className="p-5 font-mono text-sm leading-relaxed min-h-[220px]">
              {terminalLines.map((line, i) => (
                <div key={i} className="flex gap-2">
                  {line.prefix && (
                    <span className="text-[var(--accent-cyan)]">{line.prefix}</span>
                  )}
                  <span className="text-[var(--foreground)]">{line.text}</span>
                  {line.suffix && (
                    <span className="text-[var(--accent)]">{line.suffix}</span>
                  )}
                </div>
              ))}
              <div className="flex gap-2 mt-1">
                <span className="text-[var(--accent-cyan)]">$</span>
                <span className="text-[var(--foreground)]">{typedText}</span>
                <span className="cursor-blink text-[var(--accent)]">▊</span>
              </div>
            </div>
          </div>

          {/* Decorative glow */}
          <div className="absolute -inset-4 -z-10 rounded-2xl bg-gradient-to-r from-[var(--accent)]/10 via-transparent to-[var(--accent-cyan)]/10 blur-xl" />
        </motion.div>
      </div>
    </section>
  );
}
