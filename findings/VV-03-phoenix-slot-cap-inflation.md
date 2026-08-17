# VV-03: phoenix_queue_withdraw max_slot_amount allows arbitrary cap inflation

**Severity:** MEDIUM  
**Module:** `phoenix.rs`  
**Lines:** 1140-1143  
**Category:** Accounting Manipulation  
**Date:** 2026-08-17

---

## Description

The `phoenix_queue_withdraw` instruction accepts an optional `max_slot_amount: Option<u64>` parameter. When provided and greater than the current `slot.amount`, it raises the slot cap without any on-chain proof of profitability:

```rust
// phoenix.rs:1140-1143
if let Some(max) = max_slot_amount {
    if max > slot.amount {
        slot.amount = max;  // arbitrary inflation
    }
}
```

The slot cap is the protocol's bookkeeping guarantee preventing reissuance of more than deposited. The only enforcement of actual funds is Phoenix's external `withdrawFunds` CPI.

## Impact

- If Phoenix Eternal has a bug (stale margin calculation, rounding error, or oracle manipulation), the inflated cap enables minting notes for funds that were never deposited
- Breaks the accounting invariant that `reissue_amount <= deposit_amount + bounded_PnL`

## Recommended Fix

Either remove `max_slot_amount` entirely, or cap it:
```rust
if let Some(max) = max_slot_amount {
    let max_allowed = slot.amount
        .checked_add(slot.amount / 10)  // 10% max profit
        .ok_or(PrivacyError::ArithmeticOverflow)?;
    require!(max <= max_allowed, PrivacyError::SlotOverdraft);
    slot.amount = max;
}
```

## References

- `phoenix.rs:1129-1154` — Slot cap enforcement block
- `phoenix.rs:1140-1143` — Arbitrary inflation code
