"use client";

import { useState } from "react";
import { findings, stats, type Severity, type Finding } from "@/lib/findings";
import {
  ShieldCheck,
  ShieldAlert,
  AlertTriangle,
  CheckCircle,
  InfoCircle,
  ArrowDown,
  ArrowUp,
  Code,
  Calendar,
  Layers,
  Globe,
  ChevronDown,
  Shield,
  Bolt,
  Lock,
  Eye,
  FileText,
  Target,
} from "reicon-react";

const severityConfig: Record<
  Severity,
  { color: string; bg: string; border: string; icon: React.ReactNode; label: string }
> = {
  HIGH: {
    color: "#e3354d",
    bg: "#fde8eb",
    border: "#f5b3bb",
    icon: <ShieldAlert size={16} color="#e3354d" />,
    label: "Critical",
  },
  MEDIUM: {
    color: "#c99a00",
    bg: "#fef9e0",
    border: "#f5e08a",
    icon: <AlertTriangle size={16} color="#c99a00" />,
    label: "Medium",
  },
  LOW: {
    color: "#3a7bd5",
    bg: "#e8f1fd",
    border: "#a8c8f0",
    icon: <InfoCircle size={16} color="#3a7bd5" />,
    label: "Low",
  },
  INFO: {
    color: "#6b6f7e",
    bg: "#f4f5f7",
    border: "#d5d8de",
    icon: <InfoCircle size={16} color="#6b6f7e" />,
    label: "Info",
  },
};

const severityOrder: Severity[] = ["HIGH", "MEDIUM", "LOW", "INFO"];

function Navbar() {
  return (
    <nav className="sticky top-0 z-50 border-b border-[var(--hairline)] bg-white/95 backdrop-blur-md">
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <div className="flex h-14 items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2.5">
              <div className="h-8 w-8 rounded-full bg-[var(--brand-yellow)] flex items-center justify-center">
                <Shield size={16} color="#1c1c1e" weight="Filled" />
              </div>
              <span className="text-sm font-semibold text-[var(--ink)] tracking-tight">
                VeiloVault Sentinel
              </span>
            </div>
            <div className="hidden sm:flex items-center gap-1">
              <a
                href="#findings"
                className="px-3 py-1.5 text-sm text-[var(--steel)] hover:text-[var(--ink)] transition-colors rounded-full hover:bg-[var(--surface)]"
              >
                Findings
              </a>
              <a
                href="#methodology"
                className="px-3 py-1.5 text-sm text-[var(--steel)] hover:text-[var(--ink)] transition-colors rounded-full hover:bg-[var(--surface)]"
              >
                Methodology
              </a>
              <a
                href="https://github.com/popololo229099-svg/VeiloVault-Sentinel"
                target="_blank"
                rel="noopener noreferrer"
                className="px-3 py-1.5 text-sm text-[var(--steel)] hover:text-[var(--ink)] transition-colors rounded-full hover:bg-[var(--surface)] flex items-center gap-1"
              >
                <Globe size={14} />
                GitHub
              </a>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="hidden sm:flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-[var(--success)]/10 border border-[var(--success)]/20">
              <span className="h-1.5 w-1.5 rounded-full bg-[var(--success)] animate-pulse" />
              <span className="text-xs font-medium text-[var(--success)]">Mainnet</span>
            </div>
            <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-[var(--surface)] border border-[var(--hairline)]">
              <Layers size={12} className="text-[var(--muted)]" />
              <span className="text-xs text-[var(--steel)]">Slot</span>
              <span className="text-xs font-mono text-[var(--ink)]">432,860,998</span>
            </div>
          </div>
        </div>
      </div>
    </nav>
  );
}

function HeroSection() {
  return (
    <section className="relative border-b border-[var(--hairline)]">
      <div className="mx-auto max-w-6xl px-4 sm:px-6 py-10 sm:py-14">
        <div className="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-6">
          <div>
            <div className="flex items-center gap-2 mb-4">
              <span className="px-3 py-1 rounded-full text-xs font-semibold bg-[var(--brand-yellow)] text-[var(--primary)]">
                SECURITY AUDIT
              </span>
              <span className="px-3 py-1 rounded-full text-xs font-medium bg-[var(--success)]/10 text-[var(--success)] border border-[var(--success)]/20">
                COMPLETED
              </span>
            </div>
            <h1 className="text-3xl sm:text-4xl font-bold text-[var(--ink)] tracking-tight mb-2">
              Veilo Privacy Pool
            </h1>
            <p className="text-base text-[var(--slate)] max-w-xl leading-relaxed">
              Comprehensive security analysis of the Groth16 ZK-SNARK privacy
              protocol on Solana — covering swaps, perpetuals, predictions, and
              cross-mint position trading.
            </p>
          </div>
          <div className="flex flex-col gap-2 text-right shrink-0">
            <div className="flex items-center gap-2 justify-end">
              <span className="text-xs text-[var(--muted)]">Program</span>
              <code className="text-xs font-mono text-[var(--steel)] bg-[var(--surface)] px-2 py-1 rounded-md border border-[var(--hairline)]">
                GYy4k...7fFU
              </code>
            </div>
            <div className="flex items-center gap-2 justify-end">
              <span className="text-xs text-[var(--muted)]">Framework</span>
              <span className="text-xs text-[var(--steel)]">
                Anchor 0.32.1 / Solana 2.3.0
              </span>
            </div>
            <div className="flex items-center gap-2 justify-end">
              <Calendar size={12} className="text-[var(--muted)]" />
              <span className="text-xs text-[var(--steel)]">
                August 17, 2026
              </span>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function StatsBar() {
  const statItems = [
    {
      label: "Total Findings",
      value: stats.total,
      color: "var(--ink)",
      icon: <FileText size={16} color="var(--ink)" />,
    },
    {
      label: "Critical",
      value: stats.high,
      color: "var(--high)",
      icon: <ShieldAlert size={16} color="var(--high)" />,
    },
    {
      label: "Medium",
      value: stats.medium,
      color: "var(--medium)",
      icon: <AlertTriangle size={16} color="var(--medium)" />,
    },
    {
      label: "Low",
      value: stats.low,
      color: "var(--low)",
      icon: <InfoCircle size={16} color="var(--low)" />,
    },
    {
      label: "Info",
      value: stats.info,
      color: "var(--info)",
      icon: <InfoCircle size={16} color="var(--info)" />,
    },
  ];

  return (
    <section className="border-b border-[var(--hairline)] bg-[var(--surface)]">
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <div className="grid grid-cols-5 divide-x divide-[var(--hairline)]">
          {statItems.map((item, i) => (
            <div
              key={item.label}
              className={`py-4 sm:py-5 px-2 sm:px-4 animate-slide-up delay-${i + 1}`}
            >
              <div className="flex items-center gap-2 mb-1.5">
                {item.icon}
                <span className="text-xs text-[var(--steel)] hidden sm:block">
                  {item.label}
                </span>
              </div>
              <div
                className="text-xl sm:text-2xl font-bold"
                style={{ color: item.color }}
              >
                {item.value}
              </div>
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
    { label: "HIGH", count: stats.high, color: "var(--high)" },
    { label: "MEDIUM", count: stats.medium, color: "var(--medium)" },
    { label: "LOW", count: stats.low, color: "var(--low)" },
    { label: "INFO", count: stats.info, color: "var(--info)" },
  ];

  return (
    <div className="mx-auto max-w-6xl px-4 sm:px-6 py-6">
      <div className="flex items-center gap-3 mb-3">
        <div className="flex-1 h-2 rounded-full bg-[var(--hairline)] overflow-hidden flex">
          {segments.map((s) => (
            <div
              key={s.label}
              className="h-full transition-all duration-500"
              style={{
                width: `${(s.count / total) * 100}%`,
                backgroundColor: s.color,
              }}
            />
          ))}
        </div>
      </div>
      <div className="flex items-center gap-5">
        {segments.map((s) => (
          <div key={s.label} className="flex items-center gap-1.5">
            <span
              className="h-2 w-2 rounded-full"
              style={{ backgroundColor: s.color }}
            />
            <span className="text-xs text-[var(--steel)]">{s.label}</span>
            <span
              className="text-xs font-semibold"
              style={{ color: s.color }}
            >
              {s.count}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function FilterTabs({
  active,
  onChange,
}: {
  active: Severity | "ALL";
  onChange: (s: Severity | "ALL") => void;
}) {
  const tabs: { label: string; value: Severity | "ALL"; count: number }[] = [
    { label: "All Findings", value: "ALL", count: stats.total },
    { label: "Critical", value: "HIGH", count: stats.high },
    { label: "Medium", value: "MEDIUM", count: stats.medium },
    { label: "Low", value: "LOW", count: stats.low },
    { label: "Info", value: "INFO", count: stats.info },
  ];

  return (
    <div className="flex items-center gap-1 p-1 bg-[var(--surface)] rounded-full border border-[var(--hairline)] w-fit">
      {tabs.map((tab) => {
        const isActive = active === tab.value;
        return (
          <button
            key={tab.value}
            onClick={() => onChange(tab.value)}
            className={`px-3 py-1.5 rounded-full text-xs font-medium transition-all ${
              isActive
                ? "bg-[var(--primary)] text-[var(--on-primary)] shadow-sm"
                : "text-[var(--steel)] hover:text-[var(--ink)] hover:bg-white"
            }`}
          >
            {tab.label}
            <span
              className={`ml-1.5 text-[10px] ${
                isActive ? "text-[var(--muted)]" : "text-[var(--stone)]"
              }`}
            >
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
      className={`rounded-xl border transition-all duration-200 animate-slide-up delay-${Math.min(
        (index % 6) + 1,
        6
      )}`}
      style={{
        borderColor: open ? cfg.border : "var(--hairline-soft)",
        backgroundColor: open ? cfg.bg : "white",
      }}
    >
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-3 p-4 text-left"
      >
        <span
          className="inline-flex items-center justify-center gap-1 px-2.5 py-1 rounded-full text-[11px] font-semibold shrink-0"
          style={{
            backgroundColor: cfg.bg,
            color: cfg.color,
            border: `1px solid ${cfg.border}`,
          }}
        >
          {cfg.icon}
          {finding.severity}
        </span>

        <span className="text-xs font-mono text-[var(--muted)] shrink-0 w-12">
          {finding.id}
        </span>

        <span className="flex-1 text-sm font-medium text-[var(--ink)] leading-snug">
          {finding.title}
        </span>

        <span className="hidden sm:flex items-center gap-1 text-xs font-mono text-[var(--muted)] shrink-0">
          <Code size={12} />
          {finding.module}:{finding.lines}
        </span>

        <ChevronDown
          size={16}
          className={`text-[var(--muted)] shrink-0 transition-transform duration-200 ${
            open ? "rotate-180" : ""
          }`}
        />
      </button>

      {open && (
        <div
          className="border-t px-4 pb-4 pt-3 space-y-3"
          style={{ borderColor: cfg.border }}
        >
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="rounded-lg bg-white p-3 border border-[var(--hairline)]">
              <h4 className="text-[10px] font-bold uppercase tracking-widest text-[var(--muted)] mb-1.5">
                Category
              </h4>
              <p className="text-xs text-[var(--steel)]">{finding.category}</p>
            </div>
            <div className="rounded-lg bg-white p-3 border border-[var(--hairline)]">
              <h4 className="text-[10px] font-bold uppercase tracking-widest text-[var(--muted)] mb-1.5">
                Location
              </h4>
              <p className="text-xs font-mono text-[var(--steel)]">
                {finding.module} : lines {finding.lines}
              </p>
            </div>
          </div>

          <div className="rounded-lg bg-white p-3 border border-[var(--hairline)]">
            <h4 className="text-[10px] font-bold uppercase tracking-widest text-[var(--muted)] mb-1.5">
              Description
            </h4>
            <p className="text-xs text-[var(--steel)] leading-relaxed">
              {finding.description}
            </p>
          </div>

          <div className="rounded-lg bg-white p-3 border border-[var(--hairline)]">
            <h4 className="text-[10px] font-bold uppercase tracking-widest text-[var(--muted)] mb-1.5">
              Impact
            </h4>
            <p className="text-xs text-[var(--steel)] leading-relaxed">
              {finding.impact}
            </p>
          </div>

          <div
            className="rounded-lg p-3 border"
            style={{
              backgroundColor: `${cfg.color}08`,
              borderColor: `${cfg.color}20`,
            }}
          >
            <h4
              className="text-[10px] font-bold uppercase tracking-widest mb-1.5"
              style={{ color: cfg.color }}
            >
              Recommendation
            </h4>
            <p
              className="text-xs leading-relaxed"
              style={{ color: `${cfg.color}CC` }}
            >
              {finding.recommendation}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

function CriticalBanner() {
  return (
    <div className="rounded-xl border border-[var(--high-border)] bg-[var(--high-bg)] p-4">
      <div className="flex items-center gap-2 mb-3">
        <ShieldAlert size={16} color="var(--high)" />
        <span className="text-sm font-semibold text-[var(--high)]">
          Critical Findings Requiring Immediate Attention
        </span>
      </div>
      <div className="space-y-2.5">
        {findings
          .filter((f) => f.severity === "HIGH")
          .map((f) => (
            <div key={f.id} className="flex items-start gap-2.5">
              <span className="shrink-0 mt-0.5 h-1.5 w-1.5 rounded-full bg-[var(--high)]" />
              <div>
                <span className="text-xs font-mono text-[var(--high)]/70">
                  {f.id}
                </span>
                <span className="text-xs text-[var(--ink)] ml-2">
                  {f.title}
                </span>
                <span className="text-xs text-[var(--muted)] ml-2">
                  — {f.module}:{f.lines}
                </span>
              </div>
            </div>
          ))}
      </div>
    </div>
  );
}

function StrengthsSection() {
  const strengths = [
    {
      title: "ZK Proof Binding",
      desc: "All fund movements bound by Groth16 proofs with public inputs for amounts, recipients, fees, and nullifiers",
      color: "var(--success)",
      bg: "var(--surface-teal)",
      icon: <ShieldCheck size={18} color="var(--moss-dark)" />,
    },
    {
      title: "Double-Spend Prevention",
      desc: "PDA-based init constraints + is_spent flags provide defense-in-depth nullifier protection",
      color: "var(--moss-dark)",
      bg: "var(--surface-teal)",
      icon: <Lock size={18} color="var(--moss-dark)" />,
    },
    {
      title: "Claimant Co-Signature",
      desc: "Position close operations require ephemeral keypair holder signature, preventing relayer theft",
      color: "var(--brand-blue)",
      bg: "var(--surface-pricing)",
      icon: <ShieldCheck size={18} color="var(--brand-blue)" />,
    },
    {
      title: "Fee Bounds",
      desc: "Layered fee protection via fee_bps, min_withdrawal_fee, and fee_error_margin_bps",
      color: "var(--yellow-dark)",
      bg: "var(--surface-yellow)",
      icon: <Target size={18} color="var(--yellow-dark)" />,
    },
    {
      title: "Pairing Guards",
      desc: "Multi-instruction atomicity enforced via instructions_sysvar checks in fund_native_source",
      color: "var(--coral-dark)",
      bg: "var(--surface-coral)",
      icon: <Bolt size={18} color="var(--coral-dark)" />,
    },
    {
      title: "Canonical Field Elements",
      desc: "BN254 field element validation prevents non-canonical PDA manipulation",
      color: "var(--moss-dark)",
      bg: "var(--surface-rose)",
      icon: <Eye size={18} color="var(--moss-dark)" />,
    },
  ];

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {strengths.map((s, i) => (
        <div
          key={i}
          className="rounded-2xl p-4 border border-[var(--hairline-soft)] animate-slide-up"
          style={{ backgroundColor: s.bg, animationDelay: `${i * 0.05}s` }}
        >
          <div className="flex items-center gap-2 mb-2">
            {s.icon}
            <h4 className="text-xs font-semibold" style={{ color: s.color }}>
              {s.title}
            </h4>
          </div>
          <p className="text-[11px] text-[var(--slate)] leading-relaxed">
            {s.desc}
          </p>
        </div>
      ))}
    </div>
  );
}

function MethodologySection() {
  const steps = [
    {
      num: "01",
      title: "Architecture Review",
      desc: "Mapped all 10 source files, 44 instruction handlers, 16 account structs, and 6 fund-moving paths",
      color: "var(--brand-yellow)",
      bg: "var(--surface-yellow)",
    },
    {
      num: "02",
      title: "Code Review",
      desc: "Line-by-line review of every source file focusing on authorization, PDA derivations, token transfers, and arithmetic",
      color: "var(--brand-blue)",
      bg: "var(--surface-pricing)",
    },
    {
      num: "03",
      title: "Deep Analysis",
      desc: "Parallel deep-dive audits of perps.rs, positions.rs, swap.rs, and phoenix.rs with specific vulnerability hypotheses",
      color: "var(--brand-teal)",
      bg: "var(--surface-teal)",
    },
    {
      num: "04",
      title: "Verification",
      desc: "Manual verification of all critical/high findings against actual code with confirmed PoC sketches",
      color: "var(--success)",
      bg: "var(--surface-teal)",
    },
  ];

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
      {steps.map((s) => (
        <div
          key={s.num}
          className="rounded-2xl p-4 border border-[var(--hairline-soft)]"
          style={{ backgroundColor: s.bg }}
        >
          <div
            className="text-2xl font-bold font-mono mb-2"
            style={{ color: `${s.color}40` }}
          >
            {s.num}
          </div>
          <h4 className="text-sm font-semibold text-[var(--ink)] mb-1">
            {s.title}
          </h4>
          <p className="text-[11px] text-[var(--slate)] leading-relaxed">
            {s.desc}
          </p>
        </div>
      ))}
    </div>
  );
}

function Footer() {
  return (
    <footer className="border-t border-[var(--hairline)] bg-[var(--primary)] mt-12">
      <div className="mx-auto max-w-6xl px-4 sm:px-6 py-8">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="h-7 w-7 rounded-full bg-[var(--brand-yellow)] flex items-center justify-center">
              <Shield size={14} color="#1c1c1e" weight="Filled" />
            </div>
            <div>
              <span className="text-sm font-semibold text-[var(--on-primary)]">
                VeiloVault Sentinel
              </span>
              <span className="text-xs text-[var(--on-dark-muted)] ml-2">
                Automated Security Audit
              </span>
            </div>
          </div>
          <div className="flex items-center gap-4 text-xs text-[var(--on-dark-muted)]">
            <span>August 2026</span>
            <span className="text-[var(--charcoal)]">|</span>
            <a
              href="https://github.com/popololo229099-svg/VeiloVault-Sentinel"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-[var(--on-primary)] transition-colors flex items-center gap-1"
            >
              <Globe size={12} />
              GitHub
            </a>
            <span className="text-[var(--charcoal)]">|</span>
            <span>Solana / Anchor</span>
          </div>
        </div>
      </div>
    </footer>
  );
}

export default function Home() {
  const [filter, setFilter] = useState<Severity | "ALL">("ALL");

  const filteredFindings =
    filter === "ALL" ? findings : findings.filter((f) => f.severity === filter);

  return (
    <div className="min-h-screen flex flex-col bg-[var(--canvas)]">
      <Navbar />
      <HeroSection />
      <StatsBar />
      <SeverityBar />

      <main className="flex-1">
        <div className="mx-auto max-w-6xl px-4 sm:px-6 mb-8">
          <CriticalBanner />
        </div>

        <div
          id="findings"
          className="mx-auto max-w-6xl px-4 sm:px-6 mb-10"
        >
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-5">
            <h2 className="text-lg font-bold text-[var(--ink)]">All Findings</h2>
            <FilterTabs active={filter} onChange={setFilter} />
          </div>
          <div className="space-y-2">
            {filteredFindings.map((f, i) => (
              <FindingCard key={f.id} finding={f} index={i} />
            ))}
          </div>
          {filteredFindings.length === 0 && (
            <div className="text-center py-12 text-sm text-[var(--muted)]">
              No findings match the selected filter.
            </div>
          )}
        </div>

        <div className="mx-auto max-w-6xl px-4 sm:px-6 mb-10">
          <h2 className="text-lg font-bold text-[var(--ink)] mb-4">
            Strengths Observed
          </h2>
          <StrengthsSection />
        </div>

        <div
          id="methodology"
          className="mx-auto max-w-6xl px-4 sm:px-6 mb-10"
        >
          <h2 className="text-lg font-bold text-[var(--ink)] mb-4">
            Audit Methodology
          </h2>
          <MethodologySection />
        </div>

        <div className="mx-auto max-w-6xl px-4 sm:px-6 mb-10">
          <h2 className="text-lg font-bold text-[var(--ink)] mb-4">
            Scope & Limitations
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="rounded-2xl bg-[var(--surface-teal)] border border-[var(--hairline-soft)] p-4">
              <h4 className="text-xs font-bold uppercase tracking-widest text-[var(--moss-dark)] mb-2">
                In Scope
              </h4>
              <ul className="space-y-1.5">
                {[
                  "On-chain Anchor program source (10 Rust files, 5388 lines)",
                  "All 44 fund-moving instruction handlers",
                  "ZK proof verification and public input binding",
                  "PDA derivation and authorization checks",
                  "Token transfer logic (SOL, SPL, Token-2022)",
                  "Jupiter, Phoenix, JPerps, Predictions integrations",
                ].map((item, i) => (
                  <li
                    key={i}
                    className="flex items-start gap-2 text-xs text-[var(--slate)]"
                  >
                    <CheckCircle
                      size={12}
                      className="mt-0.5 shrink-0"
                      color="var(--moss-dark)"
                    />
                    {item}
                  </li>
                ))}
              </ul>
            </div>
            <div className="rounded-2xl bg-[var(--high-bg)] border border-[var(--hairline-soft)] p-4">
              <h4 className="text-xs font-bold uppercase tracking-widest text-[var(--high)] mb-2">
                Out of Scope
              </h4>
              <ul className="space-y-1.5">
                {[
                  "Circom circuits and proving artifacts",
                  "Off-chain relayer implementation",
                  "Client-side key management",
                  "Third-party program internals",
                  "Mainnet binary comparison (no CLI available)",
                  "Formal verification of ZK soundness",
                ].map((item, i) => (
                  <li
                    key={i}
                    className="flex items-start gap-2 text-xs text-[var(--slate)]"
                  >
                    <AlertTriangle
                      size={12}
                      className="mt-0.5 shrink-0"
                      color="var(--high)"
                    />
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
