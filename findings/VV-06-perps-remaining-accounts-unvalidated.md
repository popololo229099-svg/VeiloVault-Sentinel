# VV-06: Jupiter Perps remaining_accounts beyond [0] not validated

**Severity:** MEDIUM  
**Module:** `perps.rs`  
**Lines:** perps.rs:464-470, perps.rs:680-685  
**Category:** Account Substitution  
**Date:** 2026-08-17

---

## Description

In `jperp_open_position`, `jperp_set_tpsl`, and `jperp_close_position`, only `remaining[0]` is validated against `JUPITER_PERP_PROGRAM_ID`. All other 13+ accounts (perpetuals, pool, position, custody, price accounts, etc.) are passed through to the CPI without any Anchor-level validation.

While Jupiter's own CPI will reject incorrect accounts, a malicious relayer could substitute accounts from a look-alike program that accepts the same instruction layout.

## Recommended Fix

Validate at minimum:
- `remaining[1]` against known `JLP_POOL` (perpetuals pool)
- `remaining[2]` against expected pool PDA

## References

- `perps.rs:464-470` — remaining_accounts length check
- `perps.rs:536-553` — CPI account metas construction
