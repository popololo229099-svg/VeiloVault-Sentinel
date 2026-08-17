# VV-05: Vault token accounts unconstrained for native SOL pools

**Severity:** MEDIUM  
**Module:** `lib.rs`  
**Lines:** lib.rs:918-923, lib.rs:958-960  
**Category:** Defense-in-Depth  
**Date:** 2026-08-17

---

## Description

In the `TransactSwap` context, `source_vault_token_account` and `dest_vault_token_account` are `UncheckedAccount` with only `#[account(mut)]` constraints. For native SOL pools (where `mint_address == Pubkey::default()`), these accounts are unused in the handler but can be any arbitrary account:

```rust
/// CHECK: Validated in handler — for SPL: address == ATA(vault, mint); for native SOL: unused.
#[account(mut)]
pub source_vault_token_account: UncheckedAccount<'info>,
```

## Impact

Passing arbitrary accounts increases attack surface. A future code change that references these accounts for native SOL would be immediately exploitable.

## Recommended Fix

Add conditional constraints:
```rust
#[account(
    mut,
    constraint = if is_token_mint(&source_mint) {
        source_vault_token_account.key() == get_associated_token_address(...)
    } else {
        true // unused for native SOL
    } @ PrivacyError::VaultTokenAccountNotATA
)]
```

## References

- `lib.rs:918-923` — source_vault_token_account
- `lib.rs:958-960` — dest_vault_token_account
