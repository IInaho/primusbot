import { Nav } from "@/components/Nav";
import { Hero } from "@/components/Hero";
import { Features } from "@/components/Features";
import { Architecture } from "@/components/Architecture";
import { Ecosystem } from "@/components/Ecosystem";
import { Footer } from "@/components/Footer";

export default function Home() {
  return (
    <>
      <div className="mesh-bg" />
      <div className="noise-overlay" />
      <Nav />
      <main className="relative z-10">
        <Hero />
        <div className="glow-line" />
        <Features />
        <div className="glow-line" />
        <Architecture />
        <div className="glow-line" />
        <Ecosystem />
      </main>
      <Footer />
    </>
  );
}
