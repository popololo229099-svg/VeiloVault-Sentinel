# VV-04: phoenix_ember_unwrap has no claimant signer

**Severity:** MEDIUM  
**Module:** `phoenix.rs`  
**Lines:** phoenix.rs:1459-1465, lib.rs:2234-2297  
**Category:** Front-Running / Griefing  
**Date:** 2026-08-17

---

## Description

`PhoenixEmberUnwrap` does not require the `claimant` to be a `Signer`. The claimant is an instruction parameter validated only against `slot.claimant_pubkey` (a data comparison, not a cryptographic check). Only the `relayer` signs the transaction.

This means any whitelisted relayer can call `ember_unwrap` for any slot, wrapping PhUSD → USDC without the claimant's consent.

## Impact

- **Front-running:** Relayer can unwrap at unfavorable market conditions
- **Timing control:** Relayer unilaterally determines when proceeds become available for reissue
- **No fund theft:** USDC goes to vault (not relayer), but user loses control of exit timing

## Recommended Fix

Add `claimant: Signer<'info>` to `PhoenixEmberUnwrap` context, similar to `phoenix_close_position`.

## References

- `phoenix.rs:1459-1465` — Handler with no claimant signer
- `lib.rs:2234-2297` — Context struct (no `claimant: Signer`)
- `phoenix.rs:1004-1097` — `phoenix_close_position` (correctly requires `claimant: Signer`)
