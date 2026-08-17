# VV-08 through VV-12: Low and Info Findings

## VV-08: Executor ATA token surplus stranded on partial SPL reissues (LOW)

**File:** `perps.rs:1271`

The executor ATA is only closed when `executor_ata_data.amount == gross_outflow` (exact match). If a winning position's ATA holds more than gross_outflow, surplus tokens remain stranded permanently. No admin recovery path exists.

**Recommendation:** Implement an admin recovery instruction for stranded executor ATA balances after a timeout period.

---

## VV-09: relayer_token_account missing mint/owner constraints in TransactSwap (LOW)

**File:** `lib.rs:1012-1014`

`relayer_token_account` is typed as `Account<'info, TokenAccount>` but lacks `token::mint` and `token::authority` constraints. Self-inflicted only (relayer passes wrong account to themselves).

**Recommendation:** Add `token::mint = dest_mint` constraint.

---

## VV-10: Cosigner's matching accounts marked as CPI signers (LOW)

**File:** `positions.rs:1866`

```rust
let is_signer = acc.key() == exec_key || acc.key() == cosigner;
```

Any account matching the cosigner's key is marked as a signer. A malicious program-cosigner could behave arbitrarily within Jupiter legs. Practical impact is low since all funds are swept post-swap.

---

## VV-11: reduce_to_field returns modulus instead of 0 at boundary (INFO)

**File:** `swap.rs:63-99`

When input bytes exactly equal the BN254 Fr modulus, the function returns the modulus itself instead of `[0u8; 32]`. Probability of occurrence with a Solana public key: ~2^-254 (negligible).

---

## VV-12: Hardcoded byte offsets in fund_native_open_position (INFO)

**File:** `positions.rs:2362-2390`

Paired instruction validation uses hardcoded byte offsets (`OPEN_POSITION_EXECUTOR_IDX = 15`, `OPEN_SOURCE_MINT_OFFSET = 10`, `OPEN_SWAP_AMOUNT_OFFSET = 580`). If the `OpenPosition` account layout changes, these become silently wrong.

**Recommendation:** Add comment documenting the expected layout, or use dynamic parsing.
