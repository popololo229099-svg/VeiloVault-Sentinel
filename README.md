<p align="center">
  <img src="https://img.shields.io/badge/VEILOVAULT-SENTINEL-FCD535?style=for-the-badge&labelColor=0B0E11&color=FCD535" alt="VeiloVault Sentinel" />
</p>

<h1 align="center">VeiloVault Sentinel</h1>

<p align="center">
  <strong>Security Audit + Go SDK + Relayer Backend for Veilo Privacy Pool</strong><br/>
  <sub>Groth16 ZK-SNARK Privacy Protocol on Solana</sub>
</p>

<p align="center">
  <a href="https://audit-dashboard-psi-three.vercel.app">
    <img src="https://img.shields.io/badge/LIVE_DASHBOARD-0ECB81?style=for-the-badge&logo=vercel&logoColor=0ECB81&labelColor=0B0E11" alt="Live Dashboard" />
  </a>
  <a href="https://veilo-relayer.onrender.com">
    <img src="https://img.shields.io/badge/RELAYER_API-0ECB81?style=for-the-badge&logo=render&logoColor=0ECB81&labelColor=0B0E11" alt="Relayer API" />
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
  <img src="https://img.shields.io/badge/GO-1.22-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/NEXT.JS-14-000000?style=flat-square&logo=next.js&logoColor=white" alt="Next.js" />
  <img src="https://img.shields.io/badge/AUDIT_DATE-AUG_2026-848E9C?style=flat-square&labelColor=1E2329" alt="Audit Date" />
  <img src="https://img.shields.io/badge/STATUS-COMPLETED-0ECB81?style=flat-square&labelColor=1E2329" alt="Status" />
</p>

<br/>

---

## Table of Contents

- [Overview](#overview)
- [Live Endpoints](#live-endpoints)
- [Security Audit](#security-audit)
  - [Severity Summary](#severity-summary)
  - [Critical Findings](#critical-findings)
  - [All Findings](#all-findings)
  - [Recommendations](#recommendations)
  - [Security Strengths](#security-strengths)
  - [Audit Methodology](#audit-methodology)
  - [Scope & Limitations](#scope--limitations)
- [Go SDK](#go-sdk)
  - [Installation](#installation)
  - [Usage](#usage)
  - [SDK Features](#sdk-features)
- [Relayer Backend](#relayer-backend)
  - [Architecture](#architecture)
  - [API Endpoints](#api-endpoints)
  - [Configuration](#configuration)
  - [Deployment](#deployment)
- [Repository Structure](#repository-structure)
- [References](#references)

---

## Overview

**VeiloVault Sentinel** is a comprehensive security audit and developer tooling suite for the **Veilo Privacy Pool** — a Groth16 ZK-SNARK-based privacy protocol deployed on Solana. It includes:

<table>
<tr>
<td width="50%">

**Security Audit**
Unauthorized defensive audit of 10 Rust source files (~5,388 lines in lib.rs), covering 44 instruction handlers across 6 fund-moving paths.

**12 findings** identified: 2 HIGH, 5 MEDIUM, 2 LOW, 2 INFO

</td>
<td width="50%">

**Go SDK**
Type-safe Go client with PDA helpers, Anchor instruction builders, and Solana RPC integration for building relayers and tooling around Veilo.

**Relayer Backend**
Production-grade Go monolith with Clean Architecture, PostgreSQL, Redis, and REST API — deployed on Render.

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

## Live Endpoints

<table>
<tr><th>Service</th><th>URL</th><th>Status</th><th>Description</th></tr>
<tr>
  <td><strong>Audit Dashboard</strong></td>
  <td><a href="https://audit-dashboard-psi-three.vercel.app"><code>audit-dashboard-psi-three.vercel.app</code></a></td>
  <td><img src="https://img.shields.io/badge/LIVE-0ECB81?style=flat-square" /></td>
  <td>Interactive Next.js dashboard with Miro-inspired UI</td>
</tr>
<tr>
  <td><strong>Relayer API</strong></td>
  <td><a href="https://veilo-relayer.onrender.com"><code>veilo-relayer.onrender.com</code></a></td>
  <td><img src="https://img.shields.io/badge/DEPLOYING-FCD535?style=flat-square" /></td>
  <td>Go monolith relayer with REST API</td>
</tr>
<tr>
  <td><strong>Relayer Dashboard</strong></td>
  <td><a href="https://dashboard.render.com/web/srv-da1c277qj5pc73cmoagg"><code>Render Dashboard</code></a></td>
  <td><img src="https://img.shields.io/badge/MANAGE-1E90FF?style=flat-square" /></td>
  <td>Render service management panel</td>
</tr>
</table>

<br/>

**Quick Test:**

```bash
# Health check
curl https://veilo-relayer.onrender.com/api/v1/health

# Get recent transactions
curl https://veilo-relayer.onrender.com/api/v1/transactions?limit=10

# Get stats
curl https://veilo-relayer.onrender.com/api/v1/stats
```

<br/>

---

## Security Audit

### Severity Summary

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
    <td align="center"><strong>3</strong></td>
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
    <td colspan="2"><strong>12 findings</strong></td>
  </tr>
  </table>
</p>

> **Key Finding:** No direct fund-theft vulnerabilities were identified. All fund movements are cryptographically protected by Groth16 ZK-SNARK proofs. Two HIGH-severity logic errors were found that require immediate remediation.

<br/>

### Critical Findings

<table>
<tr>
<td width="20" valign="top"><img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /></td>
<td>

### VV-01 — Relayer fee silently skipped on native-SOL `jperp_reissue_notes`

**Module:** `perps.rs:1186-1214` · **Category:** Logic Error / Fee Bypass

The native-SOL pool path in `jperp_reissue_notes` validates `ext_data.fee` against the pool config and ZK proof, but **never transfers the fee to the relayer**. The entire WSOL ATA balance is swept into the vault via `token::close_account`. The SPL path (lines 1215-1282) correctly transfers the fee first.

**Impact:** The relayer is never compensated for SOL-pool perp reissues. The vault absorbs `reissue_amount + fee` but TVL only increases by `reissue_amount`, creating untracked vault surplus.

**Fix:** Add fee transfer to relayer in the native-SOL path, mirroring the SPL path.

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

**Impact:** A malicious relayer can substitute the Jupiter swap route for MEV extraction. User receives minimum guaranteed amount but not optimal execution.

**Fix:** Include `swap_data_hash` in `SwapParams::hash()` and re-circuit the ZK proof.

</td>
</tr>
</table>

<br/>

### All Findings

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

### Recommendations

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

### Security Strengths

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

### Audit Methodology

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

### Scope & Limitations

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

## Go SDK

<p align="center">
  <img src="https://img.shields.io/badge/GO_SDK-1.0.0-00ADD8?style=for-the-badge&logo=go&logoColor=white&labelColor=0B0E11" alt="Go SDK" />
  <a href="https://github.com/popololo229099-svg/VeiloVault-Sentinel/tree/main/veilo-sdk">
    <img src="https://img.shields.io/badge/SOURCE_CODE-848E9C?style=for-the-badge&labelColor=0B0E11" alt="Source" />
  </a>
</p>

Type-safe Go client for interacting with the Veilo Privacy Pool program on Solana.

### Installation

```bash
go get github.com/popolo229099-svg/veilo-sdk
```

### Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/gagliardetto/solana-go/rpc"
    veilo "github.com/popolo229099-svg/veilo-sdk/pkg/client"
)

func main() {
    client := veilo.NewClient("https://api.mainnet-beta.solana.com")
    
    // Get pool config
    pool, err := client.GetPoolConfig(rpc.CommitmentFinalized)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Pool Authority: %s\n", pool.Authority)
    fmt.Printf("Fee BPS: %d\n", pool.FeeBps)
    
    // Get vault balance
    balance, err := client.GetVaultBalance(rpc.CommitmentFinalized)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Vault Balance: %d lamports\n", balance)
    
    // Check nullifier
    spent, err := client.IsNullifierSpent([32]byte{}, rpc.CommitmentFinalized)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Nullifier spent: %v\n", spent)
}
```

### SDK Features

<table>
<tr>
<td width="50%">

- **Type-safe PDA helpers** for all on-chain accounts
- **Anchor instruction builders** for deposit, withdraw, swap
- **Transaction simulation** before submission
- **Fee estimation** with configurable BPS

</td>
<td width="50%">

- **Solana RPC client** with commitment level support
- **Account deserialization** for pool configs
- **Nullifier tracking** via on-chain state
- **Health check** endpoint integration

</td>
</tr>
</table>

<br/>

---

## Relayer Backend

<p align="center">
  <img src="https://img.shields.io/badge/RELAYER-v1.0.0-00ADD8?style=for-the-badge&logo=go&logoColor=white&labelColor=0B0E11" alt="Relayer" />
  <a href="https://veilo-relayer.onrender.com">
    <img src="https://img.shields.io/badge/LIVE_API-0ECB81?style=for-the-badge&logo=render&logoColor=0ECB81&labelColor=0B0E11" alt="Live API" />
  </a>
  <a href="https://dashboard.render.com/web/srv-da1c277qj5pc73cmoagg">
    <img src="https://img.shields.io/badge/RENDER_DASHBOARD-1E90FF?style=for-the-badge&labelColor=0B0E11" alt="Render Dashboard" />
  </a>
</p>

Production-grade monolith relayer backend built with Go, Clean Architecture, and battle-tested infrastructure.

### Architecture

<table>
<tr><th colspan="2">Tech Stack</th></tr>
<tr><td><strong>Language</strong></td><td>Go 1.22</td></tr>
<tr><td><strong>HTTP Framework</strong></td><td>Gin</td></tr>
<tr><td><strong>Database</strong></td><td>PostgreSQL (sqlx)</td></tr>
<tr><td><strong>Cache</strong></td><td>Redis (go-redis)</td></tr>
<tr><td><strong>Solana Client</strong></td><td>gagliardetto/solana-go</td></tr>
<tr><td><strong>Config</strong></td><td>Viper (YAML + env vars)</td></tr>
<tr><td><strong>Logging</strong></td><td>zerolog</td></tr>
<tr><td><strong>Deployment</strong></td><td>Render (Docker-free Go build)</td></tr>
</table>

<table>
<tr><th colspan="2">Clean Architecture Layers</th></tr>
<tr><td><code>internal/domain/</code></td><td>Entities, repository interfaces, service contracts</td></tr>
<tr><td><code>internal/usecase/</code></td><td>Business logic: relay validation, fee calculation, tx building</td></tr>
<tr><td><code>internal/infrastructure/</code></td><td>Solana RPC, PostgreSQL repos, Redis cache</td></tr>
<tr><td><code>internal/interfaces/</code></td><td>Gin HTTP handlers, middleware, route registration</td></tr>
<tr><td><code>cmd/server/</code></td><td>Entry point, DI wiring, graceful shutdown</td></tr>
</table>

### API Endpoints

<table>
<tr><th>Method</th><th>Endpoint</th><th>Description</th><th>Status</th></tr>
<tr>
  <td><img src="https://img.shields.io/badge/GET-0ECB81?style=flat-square" /></td>
  <td><code>/</code></td>
  <td>Service info</td>
  <td><img src="https://img.shields.io/badge/AVAILABLE-0ECB81?style=flat-square" /></td>
</tr>
<tr>
  <td><img src="https://img.shields.io/badge/GET-0ECB81?style=flat-square" /></td>
  <td><code>/api/v1/health</code></td>
  <td>System health + Solana slot + pool status</td>
  <td><img src="https://img.shields.io/badge/AVAILABLE-0ECB81?style=flat-square" /></td>
</tr>
<tr>
  <td><img src="https://img.shields.io/badge/POST-FCD535?style=flat-square" /></td>
  <td><code>/api/v1/relay</code></td>
  <td>Submit a privacy transaction for relay</td>
  <td><img src="https://img.shields.io/badge/AVAILABLE-0ECB81?style=flat-square" /></td>
</tr>
<tr>
  <td><img src="https://img.shields.io/badge/GET-0ECB81?style=flat-square" /></td>
  <td><code>/api/v1/transactions</code></td>
  <td>List recent relay transactions</td>
  <td><img src="https://img.shields.io/badge/AVAILABLE-0ECB81?style=flat-square" /></td>
</tr>
<tr>
  <td><img src="https://img.shields.io/badge/GET-0ECB81?style=flat-square" /></td>
  <td><code>/api/v1/transactions/:id</code></td>
  <td>Get transaction by ID</td>
  <td><img src="https://img.shields.io/badge/AVAILABLE-0ECB81?style=flat-square" /></td>
</tr>
<tr>
  <td><img src="https://img.shields.io/badge/GET-0ECB81?style=flat-square" /></td>
  <td><code>/api/v1/stats</code></td>
  <td>Relayer statistics (24h volume, fees, success rate)</td>
  <td><img src="https://img.shields.io/badge/AVAILABLE-0ECB81?style=flat-square" /></td>
</tr>
<tr>
  <td><img src="https://img.shields.io/badge/GET-0ECB81?style=flat-square" /></td>
  <td><code>/api/v1/pools</code></td>
  <td>List active privacy pools</td>
  <td><img src="https://img.shields.io/badge/AVAILABLE-0ECB81?style=flat-square" /></td>
</tr>
</table>

### Configuration

Configuration is managed via environment variables (Render Dashboard) or `configs/config.yaml`:

| Variable | Default | Description |
|----------|---------|-------------|
| `SOLANA_RPC` | `https://api.mainnet-beta.solana.com` | Solana RPC endpoint |
| `SOLANA_WS` | `wss://api.mainnet-beta.solana.com` | WebSocket endpoint |
| `PORT` | `10000` | HTTP server port |
| `GIN_MODE` | `release` | Gin framework mode |
| `DATABASE_HOST` | `localhost` | PostgreSQL host |
| `REDIS_HOST` | `localhost` | Redis host |
| `RELAYER_FEE_BPS` | `50` | Relayer fee in basis points |
| `RELAYER_MIN_FEE` | `1000000` | Minimum fee (lamports) |

### Deployment

The relayer is deployed on Render as a **Go web service** (free tier):

```
Build:  cd relayer && go mod download && go build -tags netgo -ldflags '-s -w' -o ../bin/app ./cmd/server/main.go
Start:  ../bin/app
Region: Oregon (US West)
Plan:   Free
```

<br/>

---

## Repository Structure

```
VeiloVault-Sentinel/
├── README.md                     ← You are here
├── SECURITY_AUDIT.md             Full audit report (12 findings + PoC)
│
├── findings/                     Detailed finding documents
│   ├── VV-01-*.md                HIGH — Relayer fee skip on native-SOL reissue
│   ├── VV-02-*.md                HIGH — swap_data_hash not proof-bound
│   ├── VV-03-*.md                MED  — Phoenix slot cap inflation
│   ├── VV-04-*.md                MED  — ember_unwrap no claimant signer
│   ├── VV-05-*.md                MED  — Vault accounts unconstrained
│   ├── VV-06-*.md                MED  — JPerps remaining_accounts
│   ├── VV-07-*.md                MED  — Perps slot.reissued unbounded
│   └── VV-08-through-VV-12.md   LOW/INFO — Additional findings
│
├── tests/
│   └── SECURITY_TESTS.md         Conceptual PoC test outlines
│
├── audit-dashboard/              Next.js dashboard (Miro + Reicon)
│   ├── DESIGN.md                 Miro design tokens
│   └── src/
│       ├── app/page.tsx          Dashboard page component
│       ├── app/layout.tsx        Root layout (Inter + JetBrains Mono)
│       ├── app/globals.css       Miro color palette (light canvas)
│       └── lib/findings.ts       Finding data definitions
│
├── veilo-sdk/                    Go SDK for Veilo Privacy Pool
│   ├── go.mod
│   ├── go.sum
│   └── pkg/
│       ├── types/types.go        Program ID, PoolConfig, PDA helpers
│       ├── client/client.go      SDK client (GetPool, GetBalance, Health)
│       └── anchor/instructions.go Instruction builders (deposit/withdraw/swap)
│
├── relayer/                      Go monolith relayer backend
│   ├── go.mod
│   ├── go.sum
│   ├── configs/config.yaml       Default configuration
│   ├── cmd/server/main.go        Entry point (DI, graceful shutdown)
│   └── internal/
│       ├── domain/entities.go    Entities + repository interfaces
│       ├── usecase/relay.go      Business logic (validate, fee calc, build tx)
│       ├── infrastructure/
│       │   ├── solana/client.go  Solana RPC client
│       │   ├── database/postgres.go  PostgreSQL repos (sqlx)
│       │   └── cache/redis.go    Redis cache
│       └── interfaces/api/handlers.go  Gin HTTP handlers
│
└── programs/                     Original Anchor program source (read-only)
    └── src/lib.rs
```

<br/>

---

## References

| Resource | Link |
|----------|------|
| Live Dashboard | [audit-dashboard-psi-three.vercel.app](https://audit-dashboard-psi-three.vercel.app) |
| Relayer API | [veilo-relayer.onrender.com](https://veilo-relayer.onrender.com) |
| Render Dashboard | [dashboard.render.com](https://dashboard.render.com/web/srv-da1c277qj5pc73cmoagg) |
| Full Audit Report | [SECURITY_AUDIT.md](SECURITY_AUDIT.md) |
| Target Repository | [VeiloSolana/privacy-program](https://github.com/VeiloSolana/privacy-program) |
| Fork Repository | [VeiloVault-Sentinel](https://github.com/popololo229099-svg/VeiloVault-Sentinel) |
| Program Explorer | [Solscan](https://solscan.io/account/GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU) |

<br/>

---

<p align="center">
  <img src="https://img.shields.io/badge/VEILOVAULT_SENTINEL-AUDIT_+_SDK_+_RELAYER-848E9C?style=for-the-badge&labelColor=0B0E11" alt="VeiloVault Sentinel" />
</p>

<p align="center">
  <sub>Built with <a href="https://solana.com">Solana</a> · <a href="https://www.anchor-lang.com">Anchor</a> · <a href="https://go.dev">Go</a> · <a href="https://nextjs.org">Next.js</a></sub>
</p>

<p align="center">
  <sub>August 2026 · All rights reserved</sub>
</p>
