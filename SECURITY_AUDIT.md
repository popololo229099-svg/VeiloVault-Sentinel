# VeiloVault Sentinel — Security Audit Report

**Program:** `GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU`  
**Repository:** https://github.com/VeiloSolana/privacy-program  
**Framework:** Anchor 0.32.1 / Solana 2.3.0  
**Audit Date:** 2026-08-17  
**Audit Scope:** On-chain program source (10 Rust source files, 5388 lines in lib.rs)  
**Commit Reviewed:** `e1b3bd0`, deployed slot `432860998`  
**Auditor:** VeiloVault Sentinel (automated + manual review)

---

## Executive Summary

This audit covers the Veilo privacy pool program — a Groth16 ZK-SNARK-based privacy protocol on Solana supporting native SOL, SPL tokens, Jupiter swaps, cross-mint position trading, Phoenix Eternal perpetuals, Jupiter Perpetuals, and Jupiter Prediction Markets.

**The program demonstrates strong security fundamentals:** ZK proof binding of all fund movements, PDA-based nullifier double-spend prevention, claimant co-signature enforcement on position closes, and defense-in-depth fee validation. No direct fund-theft vulnerabilities were identified that would allow an attacker to drain the vault without a valid ZK proof.

However, several design-level issues and implementation gaps were found that could lead to economic exploitation, relayer MEV extraction, fee bypass, or fund stranding under specific conditions.

---

## Findings Summary

| ID | Severity | Title | Module | Impact |
|----|----------|-------|--------|--------|
| VV-01 | **HIGH** | Relayer fee silently skipped on native-SOL `jperp_reissue_notes` | perps.rs | Relayer never compensated; vault absorbs fee surplus |
| VV-02 | **HIGH** | `swap_data_hash` not bound by ZK proof — relayer can substitute Jupiter routes | swap.rs | Relayer MEV extraction from route substitution |
| VV-03 | **MEDIUM** | `phoenix_queue_withdraw` `max_slot_amount` allows arbitrary slot cap inflation | phoenix.rs | Weakened accounting if Phoenix CPI has bugs |
| VV-04 | **MEDIUM** | `phoenix_ember_unwrap` has no claimant signer — relayer front-running | phoenix.rs | Relayer controls exit timing unilaterally |
| VV-05 | **MEDIUM** | Vault token accounts unconstrained for native SOL pools in `TransactSwap` | lib.rs | Future code changes could be exploitable |
| VV-06 | **MEDIUM** | Jupiter Perps `remaining_accounts` beyond `[0]` not validated | perps.rs | Wrong account substitution in Jupiter CPI |
| VV-07 | **MEDIUM** | `slot.reissued` is unbounded audit counter — no overdraft guard on cumulative reissues | perps.rs | No on-chain reissue cap enforcement |
| VV-08 | **LOW** | Executor ATA token surplus stranded on partial SPL reissues | perps.rs | Funds permanently locked if claimant key lost |
| VV-09 | **LOW** | `relayer_token_account` missing mint/owner constraints in `TransactSwap` | lib.rs | Defense-in-depth gap |
| VV-10 | **LOW** | Cosigner's matching accounts marked as CPI signers in `execute_jup_legs` | positions.rs | Potential griefing via malicious program-cosigner |
| VV-11 | **INFO** | `reduce_to_field` returns modulus instead of 0 at boundary | swap.rs | Negligible probability in practice |
| VV-12 | **INFO** | Hardcoded byte offsets in `fund_native_open_position` pairing guard | positions.rs | Fragile to future account layout changes |

---

## Detailed Findings

### VV-01: Relayer fee silently skipped on native-SOL `jperp_reissue_notes`

**Severity:** HIGH  
**File:** `perps.rs:1186-1214`  
**Category:** Logic Error / Fee Bypass

**Description:**  
In `jperp_reissue_notes`, the native-SOL pool path (lines 1187-1214) validates `ext_data.fee` (lines 1121-1129), computes `gross_outflow = reissue_amount + fee`, and requires `executor_wsol_data.amount >= reissue_amount` (NOT `>= gross_outflow`). It then calls `token::close_account` which sends the **entire** WSOL ATA balance (including the fee portion) to the vault. No fee is transferred to the relayer.

Compare with the SPL path (lines 1215-1282) which correctly:
1. Checks `executor_ata_data.amount >= gross_outflow`
2. Transfers `ext_data.fee` from executor → relayer
3. Then transfers `reissue_amount` from executor → vault
4. Closes the ATA

**Impact:**  
- The relayer is never compensated for SOL-pool perp reissues
- The vault absorbs `reissue_amount + fee` but TVL only increases by `reissue_amount` (line 1304-1306)
- Creates untracked vault surplus equal to the cumulative fees
- The ZK proof binds a fee that never moves, breaking the economic invariant

**PoC Sketch:**  
1. Open a JPerp position via SOL pool
2. Position profits, reissue proceeds
3. Call `jperp_reissue_notes` with `ext_data.fee = min_withdrawal_fee`
4. The fee is validated by ZK proof but never transferred — vault absorbs it

---

### VV-02: `swap_data_hash` not bound by ZK proof

**Severity:** HIGH  
**File:** `swap.rs:46-59`, `swap.rs:101-127`, `positions.rs:2000-2001`  
**Category:** MEV / Relayer Trust

**Description:**  
`SwapParams::hash()` explicitly excludes `swap_data_hash` from the Poseidon hash (documented at `swap.rs:55-57`: *"`swap_data_hash` is not proof-bound until the circuit and verifying key are upgraded"*). The on-chain check at `swap.rs:827-828` verifies `SHA256(swap_data) == swap_params.swap_data_hash`, but since the relayer provides both, this is trivially satisfiable.

The ZK proof's `swap_params_hash` public input does not constrain `swap_data_hash`, so the same valid proof works for **any** Jupiter route.

**Impact:**  
- A malicious relayer can substitute the Jupiter swap route for MEV extraction
- Routes through pools that provide kickbacks or MEV to the relayer
- User is protected by `min_amount_out` and `dest_amount` (proof-bound), but receives suboptimal execution
- All surplus (price improvement) goes to relayer

**Recommendation:** Include `swap_data_hash` in `SwapParams::hash()` and the ZK circuit's `swap_params_hash` computation.

---

### VV-03: `phoenix_queue_withdraw` `max_slot_amount` allows arbitrary slot cap inflation

**Severity:** MEDIUM  
**File:** `phoenix.rs:1140-1143`  
**Category:** Accounting Manipulation

**Description:**  
```rust
if let Some(max) = max_slot_amount {
    if max > slot.amount {
        slot.amount = max;
    }
}
```

Any signer who matches `slot.claimant_pubkey` can raise `slot.amount` to an arbitrary value (e.g., `u64::MAX`). The slot cap is the protocol's bookkeeping guarantee that a user cannot reissue more than they deposited. Enforcement relies on Phoenix's external `withdrawFunds` balance check.

**Impact:**  
- If Phoenix has a bug (stale margin, rounding error), the inflated cap lets users mint notes for funds they never deposited
- Weakened accounting invariant

**Recommendation:** Cap `max_slot_amount` to a bounded profit threshold, or remove the parameter.

---

### VV-04: `phoenix_ember_unwrap` has no claimant signer

**Severity:** MEDIUM  
**File:** `phoenix.rs:1459-1465` / `lib.rs:2234-2297`  
**Category:** Front-Running / Griefing

**Description:**  
`PhoenixEmberUnwrap` has no `claimant: Signer` field. Only the `relayer` signs. The `claimant` is an instruction parameter validated against `slot.claimant_pubkey`, but this is not a cryptographic signature check — a relayer can call this for any slot.

**Impact:**  
- Relayer can front-run any user's exit by calling `ember_unwrap` at an unfavorable time
- Relayer controls the timing of when proceeds become available for reissue
- No fund theft (USDC goes to vault), but undermines user sovereignty

**Recommendation:** Add `claimant: Signer<'info>` to prevent relayer front-running.

---

### VV-05: Vault token accounts unconstrained for native SOL pools in `TransactSwap`

**Severity:** MEDIUM  
**File:** `lib.rs:918-923`, `lib.rs:958-960`  
**Category:** Defense-in-Depth

**Description:**  
```rust
/// For native SOL pools (mint_address == Pubkey::default()), this may be unused — pass any mut account.
#[account(mut)]
pub source_vault_token_account: UncheckedAccount<'info>,
```

For native SOL pools, `source_vault_token_account` and `dest_vault_token_account` can be **any arbitrary mutable account**. While the handler code doesn't use these accounts for native SOL paths, passing arbitrary accounts increases attack surface. A future code change that mistakenly references these would be exploitable.

**Recommendation:** Add `address = Pubkey::default()` constraint for native SOL, or add conditional Anchor-level validation.

---

### VV-06: Jupiter Perps `remaining_accounts` beyond `[0]` not validated

**Severity:** MEDIUM  
**File:** `perps.rs:464-470`, `perps.rs:680-685`  
**Category:** Account Substitution

**Description:**  
Only `remaining[0]` (Jupiter Perps program ID) is validated. All other accounts (perpetuals, pool, position, custody, price accounts) are passed through unchecked. A relayer could substitute accounts belonging to a different Jupiter deployment or look-alike program.

**Impact:**  
- Jupiter's own CPI validates accounts, but a malicious program accepting the same layout could behave differently
- Low practical risk since executor PDA signing limits what can happen, but violates defense-in-depth

---

### VV-07: `slot.reissued` is unbounded audit counter

**Severity:** MEDIUM  
**File:** `perps.rs:1100-1104`, `perps.rs:1475-1478`  
**Category:** Missing Overdraft Guard

**Description:**  
Unlike Phoenix's `phoenix_queue_withdraw` which enforces `require!(new_withdrawn <= slot.amount, SlotOverdraft)`, the perps reissue/recover paths have no upper-bound check on `slot.reissued` against `slot.amount`. The `slot.amount` field is set but never read for validation.

**Impact:**  
- Compromised claimant key can drain proceeds exceeding original deposit through repeated partial reissues
- No on-chain accounting trail to detect this

---

### VV-08: Executor ATA token surplus stranded on partial SPL reissues

**Severity:** LOW  
**File:** `perps.rs:1271`  
**Category:** Fund Stranding

**Description:**  
The executor ATA is only closed when `executor_ata_data.amount == gross_outflow` (exact match). If a winning position's ATA holds `> gross_outflow`, surplus tokens remain stranded. No admin recovery path exists.

---

### VV-09: `relayer_token_account` missing mint/owner constraints in `TransactSwap`

**Severity:** LOW  
**File:** `lib.rs:1012-1014`  
**Category:** Defense-in-Depth

**Description:**  
`relayer_token_account` is typed as `Account<'info, TokenAccount>` but has no `token::mint` or `token::authority` constraints. A relayer could pass any SPL token account as their fee recipient. Self-inflicted only (relayer harms themselves).

---

### VV-10: Cosigner's matching accounts marked as CPI signers

**Severity:** LOW  
**File:** `positions.rs:1866`  
**Category:** CPI Authorization

**Description:**  
```rust
let is_signer = acc.key() == exec_key || acc.key() == cosigner;
```

Any account matching the cosigner's key is marked as a signer in Jupiter CPIs. If the cosigner is a malicious program (not just a wallet), it could behave arbitrarily within Jupiter legs while being marked as a signer.

---

### VV-11: `reduce_to_field` returns modulus instead of 0 at boundary

**Severity:** INFO  
**File:** `swap.rs:63-99`  
**Category:** Edge Case

**Description:**  
When `bytes == FR_MODULUS` exactly (all 32 bytes equal), the function returns the modulus itself instead of `[0u8; 32]`. Probability of a Solana public key matching the BN254 Fr modulus is ~2^-254 (negligible).

---

### VV-12: Hardcoded byte offsets in `fund_native_open_position`

**Severity:** INFO  
**File:** `positions.rs:2362-2390`  
**Category:** Maintainability

**Description:**  
Hardcoded byte offsets (`OPEN_POSITION_EXECUTOR_IDX = 15`, `OPEN_SOURCE_MINT_OFFSET = 10`, `OPEN_SWAP_AMOUNT_OFFSET = 580`) verify the paired open_position instruction. If the `OpenPosition` account struct changes, these silently become wrong.

---

## Strengths Observed

1. **ZK Proof Binding:** All fund movements are bound by Groth16 proofs with public inputs including amounts, recipients, fees, and nullifiers
2. **Nullifier Double-Spend Prevention:** PDA-based `init` constraints + `is_spent` flag provide defense-in-depth
3. **Claimant Co-Signature:** Position close operations require the ephemeral keypair holder to sign, preventing relayer theft
4. **Fee Bounds:** `fee_bps`, `min_withdrawal_fee`, and `fee_error_margin_bps` provide layered fee protection
5. **Cross-Tree Transactions:** Input and output trees can be different, allowing withdrawals even when input tree is full
6. **Canonical Field Element Checks:** BN254 field element validation prevents PDA manipulation
7. **Pairing Guards:** Multi-instruction atomicity enforced via `instructions_sysvar` checks in `fund_native_source` and `fund_native_jperp_open`
8. **Extensive Validation:** Token account ownership, mint matching, ATA derivation, and authority checks throughout

---

## Recommendations

### Critical Priority
1. **Fix VV-01:** Add fee transfer to relayer in the native-SOL `jperp_reissue_notes` path (mirror the SPL path)
2. **Fix VV-02:** Include `swap_data_hash` in `SwapParams::hash()` and re-circuit the ZK proof

### High Priority
3. **Fix VV-03:** Remove or cap `max_slot_amount` to a bounded profit threshold
4. **Fix VV-04:** Add `claimant: Signer` to `PhoenixEmberUnwrap`
5. **Fix VV-07:** Add `slot.reissued <= slot.amount` overdraft guard in perps reissue/recover

### Medium Priority
6. **Fix VV-05:** Add address constraints for vault token accounts in native SOL contexts
7. **Fix VV-06:** Validate critical remaining_accounts positions (globalConfiguration PDA) in perps
8. **Fix VV-09:** Add `token::mint` and `token::authority` constraints to `relayer_token_account` in `TransactSwap`

### Low Priority
9. **Fix VV-08:** Implement admin recovery path for stranded executor ATA surplus
10. **Fix VV-10:** Restrict cosigner signer marking to wallet accounts only
11. **Fix VV-12:** Replace hardcoded offsets with dynamic instruction parsing

---

## Methodology

1. **Phase 1 — Architecture Review:** Mapped all 10 source files, 44 instruction handlers, 16 account structs, and identified 6 fund-moving paths (transact, swap, position open/close, phoenix deposit/reissue, perps open/reissue, prediction open/reissue)
2. **Phase 2 — Code Review:** Read every source file line-by-line, focusing on authorization checks, PDA derivations, token transfers, and arithmetic operations
3. **Phase 3 — Deep Analysis:** Launched parallel deep-dive audits of perps.rs, positions.rs, swap.rs, and phoenix.rs with specific vulnerability hypotheses
4. **Phase 4 — Verification:** Manually verified all critical/high findings against actual code

## Scope Limitations

- **Circuits not in repo:** Circom circuits and proving artifacts are maintained separately; circuit-level vulnerabilities (soundness, completeness) are out of scope
- **Off-chain components:** Relayer implementation, note database, and client-side key management are out of scope
- **No mainnet state verification:** Could not compare source against deployed binary (Solana CLI not available)
- **Third-party programs:** Jupiter, Phoenix, Ember, and Jupiter Perps CPI security is assumed

---

*This audit is provided as-is for informational purposes. It does not constitute a formal security certification.*
