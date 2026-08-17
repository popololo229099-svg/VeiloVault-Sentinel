<p align="center">
  <img src="https://img.shields.io/badge/VEILOVAULT-SENTINEL-FCD535?style=for-the-badge&labelColor=0B0E11&color=FCD535" alt="VeiloVault Sentinel" />
</p>

<h1 align="center">Security Audit Report</h1>

<p align="center">
  <strong>Veilo Privacy Pool Program</strong><br/>
  <sub>Groth16 ZK-SNARK Privacy Protocol on Solana</sub>
</p>

<p align="center">
  <a href="https://audit-dashboard-psi-three.vercel.app">
    <img src="https://img.shields.io/badge/LIVE_DASHBOARD-0ECB81?style=for-the-badge&logo=vercel&logoColor=0ECB81&labelColor=0B0E11" alt="Live Dashboard" />
  </a>
  <a href="https://github.com/popololo229099-svg/VeiloVault-Sentinel/findings">
    <img src="https://img.shields.io/badge/FINDINGS-12-F6465D?style=for-the-badge&labelColor=0B0E11" alt="Findings" />
  </a>
  <a href="https://github.com/popololo229099-svg/VeiloVault-Sentinel/blob/main/SECURITY_AUDIT.md">
    <img src="https://img.shields.io/badge/FULL_REPORT-1E90FF?style=for-the-badge&labelColor=0B0E11" alt="Full Report" />
  </a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/SOLANA-2.3.0-9945FF?style=flat-square&logo=solana&logoColor=white" alt="Solana" />
  <img src="https://img.shields.io/badge/ANCHOR-0.32.1-FCFFFC?style=flat-square&logo=anchor&logoColor=black" alt="Anchor" />
  <img src="https://img.shields.io/badge/RUST-2021-000000?style=flat-square&logo=rust&logoColor=white" alt="Rust" />
  <img src="https://img.shields.io/badge/AUDIT_DATE-AUG_2026-848E9C?style=flat-square&labelColor=1E2329" alt="Audit Date" />
  <img src="https://img.shields.io/badge/STATUS-COMPLETED-0ECB81?style=flat-square&labelColor=1E2329" alt="Status" />
</p>

<br/>

---

## Table of Contents

- [Overview](#overview)
- [Severity Summary](#severity-summary)
- [Critical Findings](#critical-findings)
- [All Findings](#all-findings)
- [Recommendations](#recommendations)
- [Security Strengths](#security-strengths)
- [Audit Methodology](#audit-methodology)
- [Scope & Limitations](#scope--limitations)
- [Repository Structure](#repository-structure)
- [References](#references)

---

## Overview

<table>
<tr>
<td width="100%">

**VeiloVault Sentinel** is an unauthorized defensive security audit of the **Veilo Privacy Pool** — a Groth16 ZK-SNARK-based privacy protocol deployed on Solana. The program enables private transactions across native SOL, SPL tokens, Jupiter swaps, cross-mint position trading, Phoenix Eternal perpetuals, Jupiter Perpetuals, and Jupiter Prediction Markets.

</td>
</tr>
</table>

<br/>

<table>
<tr><th colspan="2">Program Details</th></tr>
<tr><td><strong>Program ID</strong></td><td><code>GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU</code></td></tr>
<tr><td><strong>Repository</strong></td><td><a href="https://github.com/VeiloSolana/privacy-program">VeiloSolana/privacy-program</a></td></tr>
<tr><td><strong>Framework</strong></td><td>Anchor 0.32.1 · Solana 2.3.0</td></tr>
<tr><td><strong>Commit</strong></td><td><code>e1b3bd0</code></td></tr>
<tr><td><strong>Deployed Slot</strong></td><td><code>432,860,998</code></td></tr>
<tr><td><strong>Upgrade Authority</strong></td><td>Squads v4 3-of-4 multisig</td></tr>
<tr><td><strong>Audit Date</strong></td><td>August 17, 2026</td></tr>
</table>

<br/>

<table>
<tr><th colspan="2">Codebase Statistics</th></tr>
<tr><td>Source Files</td><td><strong>10</strong> Rust modules</td></tr>
<tr><td>Total Lines (lib.rs)</td><td><strong>5,388</strong></td></tr>
<tr><td>Instruction Handlers</td><td><strong>44</strong></td></tr>
<tr><td>Account Structs</td><td><strong>16</strong></td></tr>
<tr><td>Error Variants</td><td><strong>53</strong></td></tr>
<tr><td>Events</td><td><strong>6</strong></td></tr>
<tr><td>Integration Modules</td><td>Jupiter · Phoenix · JPerps · Predictions</td></tr>
</table>

<br/>

---

## Severity Summary

<p align="center">
  <table>
  <tr>
    <th>Severity</th>
    <th>Count</th>
    <th>Description</th>
    <th>Priority</th>
  </tr>
  <tr>
    <td align="center"><img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /></td>
    <td align="center"><strong>2</strong></td>
    <td>Requires immediate attention</td>
    <td align="center">P0 — Fix before next deploy</td>
  </tr>
  <tr>
    <td align="center"><img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /></td>
    <td align="center"><strong>5</strong></td>
    <td>Should be addressed</td>
    <td align="center">P1 — Fix within 2 weeks</td>
  </tr>
  <tr>
    <td align="center"><img src="https://img.shields.io/badge/LOW-1E90FF?style=flat-square" /></td>
    <td align="center"><strong>2</strong></td>
    <td>Defense-in-depth improvements</td>
    <td align="center">P2 — Next sprint</td>
  </tr>
  <tr>
    <td align="center"><img src="https://img.shields.io/badge/INFO-848E9C?style=flat-square" /></td>
    <td align="center"><strong>2</strong></td>
    <td>Informational observations</td>
    <td align="center">P3 — Backlog</td>
  </tr>
  <tr>
    <td colspan="2" align="right"><strong>Total</strong></td>
    <td colspan="2"><strong>11 findings</strong></td>
  </tr>
  </table>
</p>

> **Key Finding:** No direct fund-theft vulnerabilities were identified. All fund movements are cryptographically protected by Groth16 ZK-SNARK proofs. Two HIGH-severity logic errors were found that require immediate remediation.

<br/>

---

## Critical Findings

<table>
<tr>
<td width="20" valign="top"><img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /></td>
<td>

### VV-01 — Relayer fee silently skipped on native-SOL `jperp_reissue_notes`

**Module:** `perps.rs:1186-1214` · **Category:** Logic Error / Fee Bypass

The native-SOL pool path in `jperp_reissue_notes` validates `ext_data.fee` against the pool config and ZK proof, but **never transfers the fee to the relayer**. The entire WSOL ATA balance is swept into the vault via `token::close_account`. The SPL path (lines 1215-1282) correctly transfers the fee first.

**Impact:** The relayer is never compensated for SOL-pool perp reissues. The vault absorbs `reissue_amount + fee` but TVL only increases by `reissue_amount`, creating untracked vault surplus. The ZK proof binds a fee that never moves, breaking the economic invariant.

**Fix:** Add fee transfer to relayer in the native-SOL path, mirroring the SPL path: check `gross_outflow`, transfer fee to relayer ATA, then close remaining balance to vault.

</td>
</tr>
</table>

<br/>

<table>
<tr>
<td width="20" valign="top"><img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /></td>
<td>

### VV-02 — `swap_data_hash` not bound by ZK proof

**Module:** `swap.rs:46-59, 101-127` · **Category:** MEV / Relayer Trust

`SwapParams::hash()` explicitly excludes `swap_data_hash` from the Poseidon hash used as a ZK proof public input. The relayer provides both `swap_params` and `swap_data`, making the on-chain SHA256 consistency check trivially satisfiable. The same valid proof works for **any** Jupiter route.

**Impact:** A malicious relayer can substitute the Jupiter swap route for MEV extraction, routing through pools that provide kickbacks. User receives minimum guaranteed amount (proof-bound) but not optimal execution. All surplus goes to relayer.

**Fix:** Include `swap_data_hash` in `SwapParams::hash()` and re-circuit the ZK proof to include it in public inputs.

</td>
</tr>
</table>

<br/>

---

## All Findings

<table>
<thead>
<tr>
<th>ID</th>
<th>Severity</th>
<th>Title</th>
<th>Module</th>
<th>Category</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>VV-01</code></td>
<td><img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /></td>
<td>Relayer fee skipped on native-SOL jperp reissue</td>
<td><code>perps.rs</code></td>
<td>Logic Error</td>
</tr>
<tr>
<td><code>VV-02</code></td>
<td><img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /></td>
<td>swap_data_hash not proof-bound</td>
<td><code>swap.rs</code></td>
<td>MEV</td>
</tr>
<tr>
<td><code>VV-03</code></td>
<td><img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /></td>
<td>Phoenix slot cap inflation via max_slot_amount</td>
<td><code>phoenix.rs</code></td>
<td>Accounting</td>
</tr>
<tr>
<td><code>VV-04</code></td>
<td><img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /></td>
<td>ember_unwrap has no claimant signer</td>
<td><code>phoenix.rs</code></td>
<td>Front-Running</td>
</tr>
<tr>
<td><code>VV-05</code></td>
<td><img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /></td>
<td>Vault accounts unconstrained for native SOL</td>
<td><code>lib.rs</code></td>
<td>Defense-in-Depth</td>
</tr>
<tr>
<td><code>VV-06</code></td>
<td><img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /></td>
<td>JPerps remaining_accounts unvalidated</td>
<td><code>perps.rs</code></td>
<td>Account Substitution</td>
</tr>
<tr>
<td><code>VV-07</code></td>
<td><img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /></td>
<td>Perps slot.reissued unbounded</td>
<td><code>perps.rs</code></td>
<td>Missing Guard</td>
</tr>
<tr>
<td><code>VV-08</code></td>
<td><img src="https://img.shields.io/badge/LOW-1E90FF?style=flat-square" /></td>
<td>Executor ATA surplus stranded</td>
<td><code>perps.rs</code></td>
<td>Fund Stranding</td>
</tr>
<tr>
<td><code>VV-09</code></td>
<td><img src="https://img.shields.io/badge/LOW-1E90FF?style=flat-square" /></td>
<td>relayer_token_account missing constraints</td>
<td><code>lib.rs</code></td>
<td>Defense-in-Depth</td>
</tr>
<tr>
<td><code>VV-10</code></td>
<td><img src="https://img.shields.io/badge/LOW-1E90FF?style=flat-square" /></td>
<td>Cosigner accounts as CPI signers</td>
<td><code>positions.rs</code></td>
<td>CPI Auth</td>
</tr>
<tr>
<td><code>VV-11</code></td>
<td><img src="https://img.shields.io/badge/INFO-848E9C?style=flat-square" /></td>
<td>reduce_to_field boundary case</td>
<td><code>swap.rs</code></td>
<td>Edge Case</td>
</tr>
<tr>
<td><code>VV-12</code></td>
<td><img src="https://img.shields.io/badge/INFO-848E9C?style=flat-square" /></td>
<td>Hardcoded byte offsets</td>
<td><code>positions.rs</code></td>
<td>Maintainability</td>
</tr>
</tbody>
</table>

<br/>

---

## Recommendations

<table>
<tr><th colspan="3">Prioritized Remediation Plan</th></tr>
<tr>
<td><img src="https://img.shields.io/badge/P0_CRITICAL-F6465D?style=flat-square" /></td>
<td><strong>VV-01</strong></td>
<td>Add fee transfer to relayer in the native-SOL <code>jperp_reissue_notes</code> path</td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/P0_CRITICAL-F6465D?style=flat-square" /></td>
<td><strong>VV-02</strong></td>
<td>Include <code>swap_data_hash</code> in <code>SwapParams::hash()</code> and re-circuit the ZK proof</td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/P1_HIGH-FCD535?style=flat-square" /></td>
<td><strong>VV-03</strong></td>
<td>Remove or cap <code>max_slot_amount</code> to a bounded profit threshold</td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/P1_HIGH-FCD535?style=flat-square" /></td>
<td><strong>VV-04</strong></td>
<td>Add <code>claimant: Signer</code> to <code>PhoenixEmberUnwrap</code></td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/P1_HIGH-FCD535?style=flat-square" /></td>
<td><strong>VV-07</strong></td>
<td>Add <code>slot.reissued &lt;= slot.amount</code> overdraft guard</td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/P2_MEDIUM-1E90FF?style=flat-square" /></td>
<td><strong>VV-05</strong></td>
<td>Add address constraints for vault token accounts in native SOL contexts</td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/P2_MEDIUM-1E90FF?style=flat-square" /></td>
<td><strong>VV-06</strong></td>
<td>Validate critical <code>remaining_accounts</code> positions in Jupiter Perps CPI</td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/P2_MEDIUM-1E90FF?style=flat-square" /></td>
<td><strong>VV-09</strong></td>
<td>Add <code>token::mint</code> / <code>token::authority</code> constraints to <code>relayer_token_account</code></td>
</tr>
</table>

<br/>

---

## Security Strengths

<table>
<tr>
<td width="50%">

#### ZK Proof Binding
All fund movements are bound by Groth16 proofs with public inputs for amounts, recipients, fees, and nullifiers.

#### Double-Spend Prevention
PDA-based `init` constraints + `is_spent` flags provide defense-in-depth nullifier protection.

#### Claimant Co-Signature
Position close operations require the ephemeral keypair holder to sign, preventing relayer theft.

</td>
<td width="50%">

#### Fee Bounds
Layered protection via `fee_bps`, `min_withdrawal_fee`, and `fee_error_margin_bps`.

#### Pairing Guards
Multi-instruction atomicity enforced via `instructions_sysvar` checks in `fund_native_source`.

#### Canonical Field Elements
BN254 field element validation prevents non-canonical PDA manipulation.

</td>
</tr>
</table>

<br/>

---

## Audit Methodology

<table>
<tr>
<th width="60">Phase</th>
<th width="180">Name</th>
<th>Description</th>
</tr>
<tr>
<td align="center"><strong>01</strong></td>
<td><strong>Architecture Review</strong></td>
<td>Mapped 10 source files, 44 instruction handlers, 16 account structs, and 6 fund-moving paths</td>
</tr>
<tr>
<td align="center"><strong>02</strong></td>
<td><strong>Code Review</strong></td>
<td>Line-by-line review focusing on authorization, PDA derivations, token transfers, and arithmetic</td>
</tr>
<tr>
<td align="center"><strong>03</strong></td>
<td><strong>Deep Analysis</strong></td>
<td>Parallel deep-dive audits of perps, positions, swap, and phoenix modules</td>
</tr>
<tr>
<td align="center"><strong>04</strong></td>
<td><strong>Verification</strong></td>
<td>Manual verification of all critical/high findings with confirmed PoC sketches</td>
</tr>
</table>

<br/>

---

## Scope & Limitations

<table>
<tr>
<th width="50%">In Scope</th>
<th width="50%">Out of Scope</th>
</tr>
<tr>
<td>

- On-chain Anchor program source (10 Rust files)
- All 44 fund-moving instruction handlers
- ZK proof verification and public input binding
- PDA derivation and authorization checks
- Token transfer logic (SOL, SPL, Token-2022)
- Jupiter / Phoenix / JPerps / Predictions integrations

</td>
<td>

- Circom circuits and proving artifacts (maintained separately)
- Off-chain relayer implementation
- Client-side key management and note database
- Third-party program internals (Jupiter, Phoenix, Ember)
- Mainnet binary comparison (no Solana CLI available)
- Formal verification of ZK soundness

</td>
</tr>
</table>

<br/>

---

## Repository Structure

```
VeiloVault-Sentinel/
├── README.md                  ← You are here
├── SECURITY_AUDIT.md          Full audit report (detailed findings + PoC)
├── findings/
│   ├── VV-01-*.md             HIGH — Relayer fee skip on native-SOL reissue
│   ├── VV-02-*.md             HIGH — swap_data_hash not proof-bound
│   ├── VV-03-*.md             MED  — Phoenix slot cap inflation
│   ├── VV-04-*.md             MED  — ember_unwrap no claimant signer
│   ├── VV-05-*.md             MED  — Vault accounts unconstrained
│   ├── VV-06-*.md             MED  — JPerps remaining_accounts
│   ├── VV-07-*.md             MED  — Perps slot.reissued unbounded
│   └── VV-08-through-VV-12.md LOW/INFO — Additional findings
├── tests/
│   └── SECURITY_TESTS.md      Conceptual PoC test outlines
└── audit-dashboard/           Next.js dashboard (Binance-inspired UI)
    └── src/
        ├── app/page.tsx       Dashboard page component
        ├── app/layout.tsx     Root layout
        ├── app/globals.css    Binance color palette
        └── lib/findings.ts    Finding data definitions
```

<br/>

---

## References

| Resource | Link |
|----------|------|
| Live Dashboard | [audit-dashboard-psi-three.vercel.app](https://audit-dashboard-psi-three.vercel.app) |
| Full Audit Report | [SECURITY_AUDIT.md](SECURITY_AUDIT.md) |
| Target Repository | [VeiloSolana/privacy-program](https://github.com/VeiloSolana/privacy-program) |
| Program Explorer | [Solscan](https://solscan.io/account/GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU) |

<br/>

---

<p align="center">
  <img src="https://img.shields.io/badge/VEILOVAULT_SENTINEL-AUTOMATED_SECURITY_AUDIT-848E9C?style=for-the-badge&labelColor=0B0E11" alt="VeiloVault Sentinel" />
</p>

<p align="center">
  <sub>Built with <a href="https://solana.com">Solana</a> · <a href="https://www.anchor-lang.com">Anchor</a> · <a href="https://nextjs.org">Next.js</a></sub>
</p>

<p align="center">
  <sub>August 2026 · All rights reserved</sub>
</p>
