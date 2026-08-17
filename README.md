<div align="center">

# VeiloVault Sentinel

### Security Audit — Veilo Privacy Pool Program

[![Audit Complete](https://img.shields.io/badge/Status-Audit%20Complete-0ECB81?style=for-the-badge&logo=security&logoColor=0ECB81)]()
[![Solana](https://img.shields.io/badge/Solana-2.3.0-9945FF?style=for-the-badge&logo=solana&logoColor=9945FF)]()
[![Anchor](https://img.shields.io/badge/Anchor-0.32.1-FCFFFC?style=for-the-badge&logo=anchor&logoColor=FCFFFC)]()
[![ findings](https://img.shields.io/badge/Findings-12-FCD535?style=for-the-badge&logo=bugcrowd&logoColor=FCD535)]()

**Live Dashboard →** [audit-dashboard-psi-three.vercel.app](https://audit-dashboard-psi-three.vercel.app)

</div>

---

## Overview

Unauthorized defensive security audit of the **Veilo Privacy Pool** — a Groth16 ZK-SNARK-based privacy protocol on Solana supporting native SOL, SPL tokens, Jupiter swaps, cross-mint position trading, Phoenix Eternal perpetuals, Jupiter Perpetuals, and Jupiter Prediction Markets.

| | |
|---|---|
| **Program** | `GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU` |
| **Repository** | [VeiloSolana/privacy-program](https://github.com/VeiloSolana/privacy-program) |
| **Framework** | Anchor 0.32.1 / Solana 2.3.0 |
| **Audit Date** | August 17, 2026 |
| **Commit** | `e1b3bd0` |
| **Deployed Slot** | `432860998` |
| **Codebase** | 10 Rust files · 5,388 lines (lib.rs) · 44 instructions · 16 account structs |

---

## Severity Distribution

<div align="center">

| Severity | Count | Status |
|:--------:|:-----:|:------:|
| <img src="https://img.shields.io/badge/HIGH-2-F6465D?style=flat-square" /> | **2** | Requires immediate attention |
| <img src="https://img.shields.io/badge/MEDIUM-5-FCD535?style=flat-square" /> | **5** | Should be addressed |
| <img src="https://img.shields.io/badge/LOW-2-1E90FF?style=flat-square" /> | **2** | Defense-in-depth |
| <img src="https://img.shields.io/badge/INFO-2-848E9C?style=flat-square" /> | **2** | Informational |
| **Total** | **11** | |

</div>

> **No direct fund-theft vulnerabilities found.** All fund movements are protected by Groth16 ZK-SNARK proofs. Two HIGH-severity issues were identified that require remediation.

---

## Critical Findings

### VV-01 — Relayer fee silently skipped on native-SOL `jperp_reissue_notes`

| | |
|---|---|
| **Severity** | <img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /> |
| **Module** | `perps.rs:1186-1214` |
| **Category** | Logic Error / Fee Bypass |

The native-SOL reissue path validates `ext_data.fee` against the pool config and ZK proof, but **never transfers the fee to the relayer**. The entire WSOL ATA balance is swept into the vault via `token::close_account`. The SPL path (lines 1215-1282) correctly transfers the fee first.

**Impact:** Relayer is never compensated for SOL-pool perp reissues. Vault absorbs fee surplus, creating untracked TVL discrepancy.

---

### VV-02 — `swap_data_hash` not bound by ZK proof

| | |
|---|---|
| **Severity** | <img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /> |
| **Module** | `swap.rs:46-59, 101-127` |
| **Category** | MEV / Relayer Trust |

`SwapParams::hash()` explicitly excludes `swap_data_hash` from the Poseidon hash used as a ZK proof public input. The relayer provides both `swap_params` and `swap_data`, making the SHA256 consistency check trivially satisfiable. The same valid proof works for **any** Jupiter route.

**Impact:** Relayer can substitute swap routes for MEV extraction. User gets minimum guaranteed amount but not best execution.

---

## All Findings

| ID | Severity | Title | Module | Category |
|----|:--------:|-------|--------|----------|
| VV-01 | <img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /> | Relayer fee skipped on native-SOL jperp reissue | `perps.rs` | Logic Error |
| VV-02 | <img src="https://img.shields.io/badge/HIGH-F6465D?style=flat-square" /> | swap_data_hash not proof-bound | `swap.rs` | MEV |
| VV-03 | <img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /> | Phoenix slot cap inflation | `phoenix.rs` | Accounting |
| VV-04 | <img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /> | ember_unwrap no claimant signer | `phoenix.rs` | Front-Running |
| VV-05 | <img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /> | Vault accounts unconstrained (native SOL) | `lib.rs` | Defense-in-Depth |
| VV-06 | <img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /> | JPerps remaining_accounts unvalidated | `perps.rs` | Account Substitution |
| VV-07 | <img src="https://img.shields.io/badge/MEDIUM-FCD535?style=flat-square" /> | Perps slot.reissued unbounded | `perps.rs` | Missing Guard |
| VV-08 | <img src="https://img.shields.io/badge/LOW-1E90FF?style=flat-square" /> | Executor ATA surplus stranded | `perps.rs` | Fund Stranding |
| VV-09 | <img src="https://img.shields.io/badge/LOW-1E90FF?style=flat-square" /> | relayer_token_account missing constraints | `lib.rs` | Defense-in-Depth |
| VV-10 | <img src="https://img.shields.io/badge/LOW-1E90FF?style=flat-square" /> | Cosigner accounts as CPI signers | `positions.rs` | CPI Auth |
| VV-11 | <img src="https://img.shields.io/badge/INFO-848E9C?style=flat-square" /> | reduce_to_field boundary case | `swap.rs` | Edge Case |
| VV-12 | <img src="https://img.shields.io/badge/INFO-848E9C?style=flat-square" /> | Hardcoded byte offsets | `positions.rs` | Maintainability |

---

## Recommendations

### Critical Priority
1. **Fix VV-01** — Add fee transfer to relayer in the native-SOL `jperp_reissue_notes` path
2. **Fix VV-02** — Include `swap_data_hash` in `SwapParams::hash()` and re-circuit the ZK proof

### High Priority
3. **Fix VV-03** — Remove or cap `max_slot_amount` to a bounded profit threshold
4. **Fix VV-04** — Add `claimant: Signer` to `PhoenixEmberUnwrap`
5. **Fix VV-07** — Add `slot.reissued <= slot.amount` overdraft guard

### Medium Priority
6. **Fix VV-05** — Add address constraints for vault token accounts in native SOL contexts
7. **Fix VV-06** — Validate critical `remaining_accounts` positions
8. **Fix VV-09** — Add `token::mint` / `token::authority` constraints

---

## Strengths

- **ZK Proof Binding** — All fund movements bound by Groth16 proofs with public inputs for amounts, recipients, fees, and nullifiers
- **Double-Spend Prevention** — PDA-based `init` constraints + `is_spent` flags provide defense-in-depth
- **Claimant Co-Signature** — Position close operations require ephemeral keypair holder signature
- **Fee Bounds** — Layered protection via `fee_bps`, `min_withdrawal_fee`, and `fee_error_margin_bps`
- **Pairing Guards** — Multi-instruction atomicity enforced via `instructions_sysvar` checks
- **Canonical Field Elements** — BN254 field element validation prevents non-canonical PDA manipulation
- **Cross-Tree Transactions** — Input and output trees can differ, enabling withdrawals when input tree is full

---

## Repository Structure

```
VeiloVault-Sentinel/
├── SECURITY_AUDIT.md          # Full audit report
├── README.md                  # This file
├── findings/                  # Individual finding documents
│   ├── VV-01-perps-native-reissue-fee-skip.md
│   ├── VV-02-swap-data-hash-not-proof-bound.md
│   ├── VV-03-phoenix-slot-cap-inflation.md
│   ├── VV-04-phoenix-ember-unwrap-no-claimant-signer.md
│   ├── VV-05-vault-token-unconstrained-native-sol.md
│   ├── VV-06-perps-remaining-accounts-unvalidated.md
│   ├── VV-07-perps-slot-reissued-unbounded.md
│   └── VV-08-through-VV-12.md
├── tests/
│   └── SECURITY_TESTS.md      # Conceptual PoC test outlines
└── audit-dashboard/           # Next.js dashboard (deployed to Vercel)
    └── src/
        ├── app/
        │   ├── page.tsx       # Dashboard UI
        │   ├── layout.tsx
        │   └── globals.css
        └── lib/
            └── findings.ts    # Finding data
```

---

## Methodology

| Phase | Description |
|-------|-------------|
| **01 — Architecture** | Mapped 10 source files, 44 instruction handlers, 16 account structs, 6 fund-moving paths |
| **02 — Code Review** | Line-by-line review focusing on authorization, PDA derivations, token transfers, arithmetic |
| **03 — Deep Analysis** | Parallel deep-dive audits of perps, positions, swap, and phoenix modules |
| **04 — Verification** | Manual verification of all critical/high findings with confirmed PoC sketches |

### Scope Limitations

- **Circom circuits** — Maintained separately; circuit-level vulnerabilities out of scope
- **Off-chain components** — Relayer, note database, client key management out of scope
- **Mainnet state** — No Solana CLI available for binary comparison
- **Third-party programs** — Jupiter, Phoenix, Ember CPI security assumed

---

<div align="center">

**VeiloVault Sentinel** · Automated Security Audit · August 2026

[Live Dashboard](https://audit-dashboard-psi-three.vercel.app) · [GitHub](https://github.com/popololo229099-svg/VeiloVault-Sentinel)

</div>
