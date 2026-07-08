"use client";

import { motion, useReducedMotion } from "motion/react";
import { useState, useEffect } from "react";
import { GitHubIcon } from "./GitHubIcon";

export function Nav() {
  const reduce = useReducedMotion();
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 20);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <motion.nav
      initial={reduce ? false : { opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, delay: 0.1 }}
      className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${
        scrolled
          ? "bg-[var(--background)]/80 backdrop-blur-xl border-b border-[var(--border)]"
          : "bg-transparent"
      }`}
    >
      <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
        <a href="#" className="flex items-center gap-2.5">
          <span className="text-xl">🐱</span>
          <span className="font-bold tracking-tight">NekoCode</span>
        </a>

        <div className="hidden md:flex items-center gap-8 text-sm text-[var(--muted)]">
          <a href="#features" className="hover:text-[var(--foreground)] transition-colors">
            Features
          </a>
          <a href="#architecture" className="hover:text-[var(--foreground)] transition-colors">
            Architecture
          </a>
          <a href="#ecosystem" className="hover:text-[var(--foreground)] transition-colors">
            Ecosystem
          </a>
        </div>

        <a
          href="https://github.com/lznauy/NekoCode"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-[var(--border)] bg-[var(--surface)] text-sm font-medium hover:bg-[var(--surface-elevated)] transition-colors"
        >
          <GitHubIcon />
          Star on GitHub
        </a>
      </div>
    </motion.nav>
  );
}
