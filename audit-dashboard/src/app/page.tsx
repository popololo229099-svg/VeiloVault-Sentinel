"use client";

import { useState } from "react";
import { findings, stats, type Severity, type Finding } from "@/lib/findings";

const severityConfig: Record<Severity, { color: string; bg: string; border: string; dot: string }> = {
  HIGH: { color: "#F6465D", bg: "rgba(246,70,93,0.1)", border: "rgba(246,70,93,0.25)", dot: "#F6465D" },
  MEDIUM: { color: "#FCD535", bg: "rgba(252,213,53,0.1)", border: "rgba(252,213,53,0.25)", dot: "#FCD535" },
  LOW: { color: "#1E90FF", bg: "rgba(30,144,255,0.1)", border: "rgba(30,144,255,0.25)", dot: "#1E90FF" },
  INFO: { color: "#848E9C", bg: "rgba(132,142,156,0.08)", border: "rgba(132,142,156,0.2)", dot: "#848E9C" },
};

const severityOrder: Severity[] = ["HIGH", "MEDIUM", "LOW", "INFO"];

function Navbar() {
  return (
    <nav className="sticky top-0 z-50 border-b border-[#2B3139] bg-[#0B0E11]/95 backdrop-blur-md">
      <div className="mx-auto max-w-7xl px-4 sm:px-6">
        <div className="flex h-14 items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <div className="h-8 w-8 rounded-lg bg-[#FCD535] flex items-center justify-center">
                <svg className="h-4.5 w-4.5 text-black" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M12 2L3 7v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-9-5zm-1 15l-4-4 1.41-1.41L11 14.17l6.59-6.59L19 9l-8 8z"/>
                </svg>
              </div>
              <span className="text-base font-semibold tracking-tight">VeiloVault Sentinel</span>
            </div>
            <div className="hidden sm:flex items-center ml-6 gap-1">
              <a href="#findings" className="px-3 py-1.5 text-sm text-[#848E9C] hover:text-[#EAECEF] transition-colors rounded-md hover:bg-[#1E2329]">Findings</a>
              <a href="#methodology" className="px-3 py-1.5 text-sm text-[#848E9C] hover:text-[#EAECEF] transition-colors rounded-md hover:bg-[#1E2329]">Methodology</a>
              <a href="https://github.com/popololo229099-svg/VeiloVault-Sentinel" target="_blank" rel="noopener noreferrer" className="px-3 py-1.5 text-sm text-[#848E9C] hover:text-[#EAECEF] transition-colors rounded-md hover:bg-[#1E2329]">GitHub</a>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-md bg-[#1E2329] border border-[#2B3139]">
              <span className="h-2 w-2 rounded-full bg-[#0ECB81] animate-pulse" />
              <span className="text-xs text-[#848E9C]">Mainnet</span>
            </div>
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-[#1E2329] border border-[#2B3139]">
              <span className="text-xs text-[#848E9C]">Slot</span>
              <span className="text-xs font-mono text-[#EAECEF]">432,860,998</span>
            </div>
          </div>
        </div>
      </div>
    </nav>
  );
}

function HeroSection() {
  return (
    <section className="relative overflow-hidden border-b border-[#2B3139]">
      {/* Background gradient */}
      <div className="absolute inset-0 bg-gradient-to-br from-[#FCD535]/[0.03] via-transparent to-[#0ECB81]/[0.02]" />
      <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-[300px] bg-[#FCD535]/[0.04] blur-[100px] rounded-full" />

      <div className="relative mx-auto max-w-7xl px-4 sm:px-6 py-10 sm:py-14">
        <div className="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-6">
          <div>
            <div className="flex items-center gap-2 mb-3">
              <span className="px-2.5 py-1 rounded text-xs font-semibold bg-[#FCD535]/15 text-[#FCD535] border border-[#FCD535]/25">
                SECURITY AUDIT
              </span>
              <span className="px-2.5 py-1 rounded text-xs font-medium bg-[#0ECB81]/15 text-[#0ECB81] border border-[#0ECB81]/25">
                COMPLETED
              </span>
            </div>
            <h1 className="text-3xl sm:text-4xl font-bold tracking-tight mb-2">
              Veilo Privacy Pool
            </h1>
            <p className="text-base text-[#848E9C] max-w-xl">
              Comprehensive security analysis of the Groth16 ZK-SNARK privacy protocol on Solana — covering swaps, perpetuals, predictions, and cross-mint position trading.
            </p>
          </div>
          <div className="flex flex-col gap-2 text-right shrink-0">
            <div className="flex items-center gap-2 justify-end">
              <span className="text-xs text-[#5E6673]">Program</span>
              <code className="text-xs font-mono text-[#848E9C] bg-[#1E2329] px-2 py-1 rounded border border-[#2B3139]">
                GYy4k...7fFU
              </code>
            </div>
            <div className="flex items-center gap-2 justify-end">
              <span className="text-xs text-[#5E6673]">Framework</span>
              <span className="text-xs text-[#848E9C]">Anchor 0.32.1 / Solana 2.3.0</span>
            </div>
            <div className="flex items-center gap-2 justify-end">
              <span className="text-xs text-[#5E6673]">Audit Date</span>
              <span className="text-xs text-[#848E9C]">August 17, 2026</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function StatsBar() {
  const statItems = [
    { label: "Total Findings", value: stats.total, color: "#EAECEF", icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" },
    { label: "Critical", value: stats.high, color: "#F6465D", icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4.5c-.77-.833-2.694-.833-3.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" },
    { label: "Medium", value: stats.medium, color: "#FCD535", icon: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" },
    { label: "Low", value: stats.low, color: "#1E90FF", icon: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" },
    { label: "Info", value: stats.info, color: "#848E9C", icon: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" },
  ];

  return (
    <section className="border-b border-[#2B3139] bg-[#1E2329]/50">
      <div className="mx-auto max-w-7xl px-4 sm:px-6">
        <div className="grid grid-cols-5 divide-x divide-[#2B3139]">
          {statItems.map((item, i) => (
            <div key={item.label} className={`py-4 sm:py-5 px-2 sm:px-4 animate-slide-up delay-${i + 1}`}>
              <div className="flex items-center gap-2 mb-1.5">
                <svg className="h-3.5 w-3.5 hidden sm:block" style={{ color: item.color }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d={item.icon} />
                </svg>
                <span className="text-xs text-[#5E6673]">{item.label}</span>
              </div>
              <div className="text-xl sm:text-2xl font-bold" style={{ color: item.color }}>{item.value}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function SeverityBar() {
  const total = stats.total || 1;
  const segments = [
    { label: "HIGH", count: stats.high, color: "#F6465D" },
    { label: "MEDIUM", count: stats.medium, color: "#FCD535" },
    { label: "LOW", count: stats.low, color: "#1E90FF" },
    { label: "INFO", count: stats.info, color: "#848E9C" },
  ];

  return (
    <div className="mx-auto max-w-7xl px-4 sm:px-6 py-6">
      <div className="flex items-center gap-3 mb-3">
        <div className="flex-1 h-2 rounded-full bg-[#1E2329] overflow-hidden flex">
          {segments.map((s) => (
            <div
              key={s.label}
              className="h-full transition-all duration-500"
              style={{ width: `${(s.count / total) * 100}%`, backgroundColor: s.color }}
            />
          ))}
        </div>
      </div>
      <div className="flex items-center gap-5">
        {segments.map((s) => (
          <div key={s.label} className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-sm" style={{ backgroundColor: s.color }} />
            <span className="text-xs text-[#848E9C]">{s.label}</span>
            <span className="text-xs font-semibold" style={{ color: s.color }}>{s.count}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function FilterTabs({ active, onChange }: { active: Severity | "ALL"; onChange: (s: Severity | "ALL") => void }) {
  const tabs: { label: string; value: Severity | "ALL"; count: number }[] = [
    { label: "All Findings", value: "ALL", count: stats.total },
    { label: "Critical", value: "HIGH", count: stats.high },
    { label: "Medium", value: "MEDIUM", count: stats.medium },
    { label: "Low", value: "LOW", count: stats.low },
    { label: "Info", value: "INFO", count: stats.info },
  ];

  return (
    <div className="flex items-center gap-1 p-1 bg-[#1E2329] rounded-lg border border-[#2B3139] w-fit">
      {tabs.map((tab) => {
        const isActive = active === tab.value;
        const cfg = tab.value === "ALL" ? null : severityConfig[tab.value as Severity];
        return (
          <button
            key={tab.value}
            onClick={() => onChange(tab.value)}
            className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
              isActive
                ? "bg-[#2B3139] text-[#EAECEF] shadow-sm"
                : "text-[#5E6673] hover:text-[#848E9C] hover:bg-[#2B3139]/50"
            }`}
          >
            {tab.label}
            <span className={`ml-1.5 text-[10px] ${isActive ? "text-[#848E9C]" : "text-[#5E6673]"}`}>
              {tab.count}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function FindingCard({ finding, index }: { finding: Finding; index: number }) {
  const cfg = severityConfig[finding.severity];
  const [open, setOpen] = useState(false);

  return (
    <div
      className={`rounded-lg border transition-all duration-200 animate-slide-up delay-${Math.min((index % 6) + 1, 6)}`}
      style={{
        borderColor: open ? cfg.border : "#2B3139",
        backgroundColor: open ? cfg.bg : "#1E2329",
      }}
    >
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-3 p-4 text-left"
      >
        {/* Severity badge */}
        <span
          className="inline-flex items-center justify-center px-2 py-0.5 rounded text-[10px] font-bold tracking-wider shrink-0"
          style={{
            backgroundColor: cfg.bg,
            color: cfg.color,
            border: `1px solid ${cfg.border}`,
          }}
        >
          {finding.severity}
        </span>

        {/* ID */}
        <span className="text-xs font-mono text-[#5E6673] shrink-0 w-12">{finding.id}</span>

        {/* Title */}
        <span className="flex-1 text-sm font-medium text-[#EAECEF] leading-snug">{finding.title}</span>

        {/* Module */}
        <span className="hidden sm:inline text-xs font-mono text-[#5E6673] shrink-0">
          {finding.module}:{finding.lines}
        </span>

        {/* Chevron */}
        <svg
          className={`h-4 w-4 text-[#5E6673] shrink-0 transition-transform duration-200 ${open ? "rotate-180" : ""}`}
          fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {open && (
        <div className="border-t px-4 pb-4 pt-3 space-y-3" style={{ borderColor: cfg.border }}>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="rounded-md bg-[#0B0E11]/60 p-3 border border-[#2B3139]">
              <h4 className="text-[10px] font-bold uppercase tracking-widest text-[#5E6673] mb-1.5">Category</h4>
              <p className="text-xs text-[#848E9C]">{finding.category}</p>
            </div>
            <div className="rounded-md bg-[#0B0E11]/60 p-3 border border-[#2B3139]">
              <h4 className="text-[10px] font-bold uppercase tracking-widest text-[#5E6673] mb-1.5">Location</h4>
              <p className="text-xs font-mono text-[#848E9C]">{finding.module} : lines {finding.lines}</p>
            </div>
          </div>

          <div className="rounded-md bg-[#0B0E11]/60 p-3 border border-[#2B3139]">
            <h4 className="text-[10px] font-bold uppercase tracking-widest text-[#5E6673] mb-1.5">Description</h4>
            <p className="text-xs text-[#848E9C] leading-relaxed">{finding.description}</p>
          </div>

          <div className="rounded-md bg-[#0B0E11]/60 p-3 border border-[#2B3139]">
            <h4 className="text-[10px] font-bold uppercase tracking-widest text-[#5E6673] mb-1.5">Impact</h4>
            <p className="text-xs text-[#848E9C] leading-relaxed">{finding.impact}</p>
          </div>

          <div className="rounded-md p-3 border" style={{ backgroundColor: `${cfg.color}08`, borderColor: `${cfg.color}20` }}>
            <h4 className="text-[10px] font-bold uppercase tracking-widest mb-1.5" style={{ color: cfg.color }}>Recommendation</h4>
            <p className="text-xs leading-relaxed" style={{ color: `${cfg.color}CC` }}>{finding.recommendation}</p>
          </div>
        </div>
      )}
    </div>
  );
}

function CriticalBanner() {
  return (
    <div className="rounded-lg border border-[#F6465D]/25 bg-[#F6465D]/[0.06] p-4">
      <div className="flex items-center gap-2 mb-3">
        <svg className="h-4 w-4 text-[#F6465D]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4.5c-.77-.833-2.694-.833-3.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" />
        </svg>
        <span className="text-sm font-semibold text-[#F6465D]">Critical Findings Requiring Immediate Attention</span>
      </div>
      <div className="space-y-2.5">
        {findings.filter(f => f.severity === "HIGH").map(f => (
          <div key={f.id} className="flex items-start gap-2.5">
            <span className="shrink-0 mt-0.5 h-1.5 w-1.5 rounded-full bg-[#F6465D]" />
            <div>
              <span className="text-xs font-mono text-[#F6465D]/70">{f.id}</span>
              <span className="text-xs text-[#EAECEF] ml-2">{f.title}</span>
              <span className="text-xs text-[#5E6673] ml-2">— {f.module}:{f.lines}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function StrengthsSection() {
  const strengths = [
    { title: "ZK Proof Binding", desc: "All fund movements bound by Groth16 proofs with public inputs for amounts, recipients, fees, and nullifiers" },
    { title: "Double-Spend Prevention", desc: "PDA-based init constraints + is_spent flags provide defense-in-depth nullifier protection" },
    { title: "Claimant Co-Signature", desc: "Position close operations require ephemeral keypair holder signature, preventing relayer theft" },
    { title: "Fee Bounds", desc: "Layered fee protection via fee_bps, min_withdrawal_fee, and fee_error_margin_bps" },
    { title: "Pairing Guards", desc: "Multi-instruction atomicity enforced via instructions_sysvar checks in fund_native_source" },
    { title: "Canonical Field Elements", desc: "BN254 field element validation prevents non-canonical PDA manipulation" },
  ];

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {strengths.map((s, i) => (
        <div key={i} className="rounded-lg bg-[#0ECB81]/[0.04] border border-[#0ECB81]/15 p-3.5 animate-slide-up" style={{ animationDelay: `${i * 0.05}s` }}>
          <div className="flex items-center gap-2 mb-1.5">
            <svg className="h-3.5 w-3.5 text-[#0ECB81]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
            </svg>
            <h4 className="text-xs font-semibold text-[#0ECB81]">{s.title}</h4>
          </div>
          <p className="text-[11px] text-[#848E9C] leading-relaxed">{s.desc}</p>
        </div>
      ))}
    </div>
  );
}

function MethodologySection() {
  const steps = [
    { num: "01", title: "Architecture Review", desc: "Mapped all 10 source files, 44 instruction handlers, 16 account structs, and 6 fund-moving paths" },
    { num: "02", title: "Code Review", desc: "Line-by-line review of every source file focusing on authorization, PDA derivations, token transfers, and arithmetic" },
    { num: "03", title: "Deep Analysis", desc: "Parallel deep-dive audits of perps.rs, positions.rs, swap.rs, and phoenix.rs with specific vulnerability hypotheses" },
    { num: "04", title: "Verification", desc: "Manual verification of all critical/high findings against actual code with confirmed PoC sketches" },
  ];

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
      {steps.map((s) => (
        <div key={s.num} className="rounded-lg bg-[#1E2329] border border-[#2B3139] p-4">
          <div className="text-2xl font-bold text-[#FCD535]/30 font-mono mb-2">{s.num}</div>
          <h4 className="text-sm font-semibold text-[#EAECEF] mb-1">{s.title}</h4>
          <p className="text-[11px] text-[#5E6673] leading-relaxed">{s.desc}</p>
        </div>
      ))}
    </div>
  );
}

function Footer() {
  return (
    <footer className="border-t border-[#2B3139] bg-[#1E2329]/30 mt-12">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 py-8">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="h-7 w-7 rounded-md bg-[#FCD535] flex items-center justify-center">
              <svg className="h-3.5 w-3.5 text-black" fill="currentColor" viewBox="0 0 24 24">
                <path d="M12 2L3 7v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-9-5zm-1 15l-4-4 1.41-1.41L11 14.17l6.59-6.59L19 9l-8 8z"/>
              </svg>
            </div>
            <div>
              <span className="text-sm font-semibold">VeiloVault Sentinel</span>
              <span className="text-xs text-[#5E6673] ml-2">Automated Security Audit</span>
            </div>
          </div>
          <div className="flex items-center gap-4 text-xs text-[#5E6673]">
            <span>August 2026</span>
            <span className="text-[#2B3139]">|</span>
            <a href="https://github.com/popololo229099-svg/VeiloVault-Sentinel" target="_blank" rel="noopener noreferrer" className="hover:text-[#848E9C] transition-colors">GitHub</a>
            <span className="text-[#2B3139]">|</span>
            <span>Solana / Anchor</span>
          </div>
        </div>
      </div>
    </footer>
  );
}

export default function Home() {
  const [filter, setFilter] = useState<Severity | "ALL">("ALL");

  const filteredFindings = filter === "ALL" ? findings : findings.filter((f) => f.severity === filter);

  return (
    <div className="min-h-screen flex flex-col">
      <Navbar />
      <HeroSection />
      <StatsBar />
      <SeverityBar />

      <main className="flex-1">
        {/* Critical Findings */}
        <div className="mx-auto max-w-7xl px-4 sm:px-6 mb-8">
          <CriticalBanner />
        </div>

        {/* All Findings */}
        <div id="findings" className="mx-auto max-w-7xl px-4 sm:px-6 mb-10">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-5">
            <h2 className="text-lg font-bold">All Findings</h2>
            <FilterTabs active={filter} onChange={setFilter} />
          </div>
          <div className="space-y-2">
            {filteredFindings.map((f, i) => (
              <FindingCard key={f.id} finding={f} index={i} />
            ))}
          </div>
          {filteredFindings.length === 0 && (
            <div className="text-center py-12 text-sm text-[#5E6673]">
              No findings match the selected filter.
            </div>
          )}
        </div>

        {/* Strengths */}
        <div className="mx-auto max-w-7xl px-4 sm:px-6 mb-10">
          <h2 className="text-lg font-bold mb-4">Strengths Observed</h2>
          <StrengthsSection />
        </div>

        {/* Methodology */}
        <div id="methodology" className="mx-auto max-w-7xl px-4 sm:px-6 mb-10">
          <h2 className="text-lg font-bold mb-4">Audit Methodology</h2>
          <MethodologySection />
        </div>

        {/* Scope */}
        <div className="mx-auto max-w-7xl px-4 sm:px-6 mb-10">
          <h2 className="text-lg font-bold mb-4">Scope & Limitations</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="rounded-lg bg-[#1E2329] border border-[#2B3139] p-4">
              <h4 className="text-xs font-bold uppercase tracking-widest text-[#0ECB81] mb-2">In Scope</h4>
              <ul className="space-y-1.5">
                {["On-chain Anchor program source (10 Rust files, 5388 lines)", "All 44 fund-moving instruction handlers", "ZK proof verification and public input binding", "PDA derivation and authorization checks", "Token transfer logic (SOL, SPL, Token-2022)", "Jupiter, Phoenix, JPerps, Predictions integrations"].map((item, i) => (
                  <li key={i} className="flex items-start gap-2 text-xs text-[#848E9C]">
                    <span className="text-[#0ECB81] mt-0.5">+</span>
                    {item}
                  </li>
                ))}
              </ul>
            </div>
            <div className="rounded-lg bg-[#1E2329] border border-[#2B3139] p-4">
              <h4 className="text-xs font-bold uppercase tracking-widest text-[#F6465D] mb-2">Out of Scope</h4>
              <ul className="space-y-1.5">
                {["Circom circuits and proving artifacts", "Off-chain relayer implementation", "Client-side key management", "Third-party program internals", "Mainnet binary comparison (no CLI available)", "Formal verification of ZK soundness"].map((item, i) => (
                  <li key={i} className="flex items-start gap-2 text-xs text-[#848E9C]">
                    <span className="text-[#F6465D] mt-0.5">-</span>
                    {item}
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>
      </main>

      <Footer />
    </div>
  );
}
