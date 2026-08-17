import { findings, stats, type Severity } from "@/lib/findings";

function severityColor(s: Severity) {
  switch (s) {
    case "HIGH":
      return "bg-red-500/15 text-red-400 border-red-500/30";
    case "MEDIUM":
      return "bg-amber-500/15 text-amber-400 border-amber-500/30";
    case "LOW":
      return "bg-blue-500/15 text-blue-400 border-blue-500/30";
    case "INFO":
      return "bg-zinc-500/15 text-zinc-400 border-zinc-500/30";
  }
}

function severityDot(s: Severity) {
  switch (s) {
    case "HIGH":
      return "bg-red-500";
    case "MEDIUM":
      return "bg-amber-500";
    case "LOW":
      return "bg-blue-500";
    case "INFO":
      return "bg-zinc-500";
  }
}

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="rounded-xl border border-zinc-800 bg-zinc-900/50 p-5">
      <div className={`text-3xl font-bold ${color}`}>{value}</div>
      <div className="mt-1 text-sm text-zinc-400">{label}</div>
    </div>
  );
}

function FindingCard({ finding, index }: { finding: (typeof findings)[0]; index: number }) {
  return (
    <details className={`group rounded-xl border border-zinc-800 bg-zinc-900/50 transition-colors hover:border-zinc-700 animate-fade-in opacity-0 stagger-${Math.min(index % 5 + 1, 5)}`}>
      <summary className="flex cursor-pointer items-center gap-3 p-5 select-none">
        <span className={`h-2.5 w-2.5 rounded-full ${severityDot(finding.severity)} shrink-0`} />
        <span className={`inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium ${severityColor(finding.severity)}`}>
          {finding.severity}
        </span>
        <span className="font-mono text-xs text-zinc-500">{finding.id}</span>
        <span className="flex-1 text-sm font-medium text-zinc-200">{finding.title}</span>
        <span className="text-xs text-zinc-500 font-mono">{finding.module}:{finding.lines}</span>
        <svg className="h-4 w-4 text-zinc-500 transition-transform group-open:rotate-180 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" /></svg>
      </summary>
      <div className="border-t border-zinc-800 px-5 pb-5 pt-4 space-y-4">
        <div>
          <h4 className="text-xs font-semibold uppercase tracking-wider text-zinc-500 mb-1.5">Category</h4>
          <p className="text-sm text-zinc-300">{finding.category}</p>
        </div>
        <div>
          <h4 className="text-xs font-semibold uppercase tracking-wider text-zinc-500 mb-1.5">Description</h4>
          <p className="text-sm leading-relaxed text-zinc-300">{finding.description}</p>
        </div>
        <div>
          <h4 className="text-xs font-semibold uppercase tracking-wider text-zinc-500 mb-1.5">Impact</h4>
          <p className="text-sm leading-relaxed text-zinc-300">{finding.impact}</p>
        </div>
        <div>
          <h4 className="text-xs font-semibold uppercase tracking-wider text-zinc-500 mb-1.5">Recommendation</h4>
          <p className="text-sm leading-relaxed text-zinc-300">{finding.recommendation}</p>
        </div>
      </div>
    </details>
  );
}

export default function Home() {
  return (
    <div className="min-h-screen">
      {/* Hero */}
      <header className="relative overflow-hidden border-b border-zinc-800">
        <div className="absolute inset-0 bg-gradient-to-br from-emerald-500/5 via-transparent to-blue-500/5" />
        <div className="relative mx-auto max-w-5xl px-6 py-16">
          <div className="flex items-center gap-3 mb-4">
            <div className="h-10 w-10 rounded-lg bg-emerald-500/20 border border-emerald-500/30 flex items-center justify-center">
              <svg className="h-5 w-5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
            </div>
            <h1 className="text-2xl font-bold tracking-tight">VeiloVault Sentinel</h1>
          </div>
          <p className="text-lg text-zinc-400 max-w-2xl mb-2">
            Security Audit of the Veilo Privacy Pool Program
          </p>
          <p className="text-sm text-zinc-500 font-mono">
            GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU &middot; Slot 432860998 &middot; Commit e1b3bd0
          </p>
        </div>
      </header>

      {/* Stats */}
      <section className="mx-auto max-w-5xl px-6 py-10">
        <div className="grid grid-cols-2 sm:grid-cols-5 gap-4">
          <StatCard label="Total Findings" value={stats.total} color="text-zinc-100" />
          <StatCard label="High Severity" value={stats.high} color="text-red-400" />
          <StatCard label="Medium Severity" value={stats.medium} color="text-amber-400" />
          <StatCard label="Low Severity" value={stats.low} color="text-blue-400" />
          <StatCard label="Informational" value={stats.info} color="text-zinc-400" />
        </div>
      </section>

      {/* Key Findings */}
      <section className="mx-auto max-w-5xl px-6 pb-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <span className="h-2 w-2 rounded-full bg-red-500" />
          Critical Findings
        </h2>
        <div className="rounded-xl border border-red-500/20 bg-red-500/5 p-5 space-y-3">
          <div className="flex items-start gap-3">
            <span className="inline-flex items-center rounded-md border border-red-500/30 bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-400 shrink-0">HIGH</span>
            <div>
              <p className="text-sm font-medium text-red-300">VV-01: Relayer fee silently skipped on native-SOL jperp_reissue_notes</p>
              <p className="text-xs text-zinc-400 mt-1">perps.rs:1186-1214 &mdash; The native-SOL reissue path validates the fee but never transfers it to the relayer. Vault absorbs the surplus.</p>
            </div>
          </div>
          <div className="flex items-start gap-3">
            <span className="inline-flex items-center rounded-md border border-red-500/30 bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-400 shrink-0">HIGH</span>
            <div>
              <p className="text-sm font-medium text-red-300">VV-02: swap_data_hash not bound by ZK proof</p>
              <p className="text-xs text-zinc-400 mt-1">swap.rs:46-59 &mdash; Relayer can substitute Jupiter swap routes for MEV extraction. User gets minimum guaranteed amount but not best execution.</p>
            </div>
          </div>
        </div>
      </section>

      {/* All Findings */}
      <section className="mx-auto max-w-5xl px-6 py-10">
        <h2 className="text-lg font-semibold mb-6">All Findings</h2>
        <div className="space-y-3">
          {findings.map((f, i) => (
            <FindingCard key={f.id} finding={f} index={i} />
          ))}
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-zinc-800 py-8 mt-10">
        <div className="mx-auto max-w-5xl px-6 text-center text-xs text-zinc-600">
          VeiloVault Sentinel &middot; Automated Security Audit &middot; August 2026
        </div>
      </footer>
    </div>
  );
}
