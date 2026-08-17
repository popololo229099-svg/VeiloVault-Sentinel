# VV-07: slot.reissued is unbounded audit counter

**Severity:** MEDIUM  
**Module:** `perps.rs`  
**Lines:** perps.rs:1100-1104, perps.rs:1475-1478  
**Category:** Missing Overdraft Guard  
**Date:** 2026-08-17

---

## Description

In `jperp_reissue_notes` and `jperp_recover_native`, `slot.reissued` is incremented as a cumulative counter but never checked against `slot.amount`:

```rust
// perps.rs:1475-1478
slot.reissued = slot.reissued
    .checked_add(reissue_amount)
    .ok_or(error!(PrivacyError::ArithmeticOverflow))?;
```

Compare with Phoenix's `phoenix_queue_withdraw` which enforces:
```rust
// phoenix.rs:1149
require!(new_withdrawn <= slot.amount, PrivacyError::SlotOverdraft);
```

## Impact

No on-chain cap on cumulative reissues per slot. While the executor ATA balance provides a practical limit, the accounting gap could allow over-reissuance if the claimant's ephemeral key is compromised.

## Recommended Fix

Add overdraft check before incrementing:
```rust
let new_reissued = slot.reissued
    .checked_add(reissue_amount)
    .ok_or(PrivacyError::ArithmeticOverflow)?;
require!(new_reissued <= slot.amount, PrivacyError::SlotOverdraft);
slot.reissued = new_reissued;
```

## References

- `perps.rs:1475-1478` — Unbounded increment
- `phoenix.rs:1146-1149` — Phoenix's correct overdraft guard
