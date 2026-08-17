# VV-01: Relayer fee silently skipped on native-SOL jperp_reissue_notes

**Severity:** HIGH  
**Module:** `perps.rs`  
**Lines:** 1186-1214  
**Category:** Logic Error / Fee Bypass  
**Date:** 2026-08-17

---

## Description

In `jperp_reissue_notes`, the native-SOL pool path validates `ext_data.fee` against the pool's fee config and ZK proof, but never actually transfers the fee to the relayer. The entire WSOL ATA balance (including the fee portion) is swept into the vault via `token::close_account`.

## Vulnerable Code

```rust
// perps.rs:1187-1214 — NATIVE SOL PATH (BUGGY)
let is_native_sol_pool = mint_address == Pubkey::default();
if is_native_sol_pool {
    // ... validation ...
    require!(
        executor_wsol_data.amount >= reissue_amount,  // BUG: should be >= gross_outflow
        PrivacyError::InsufficientFundsForWithdrawal
    );
    // close_account sends ALL WSOL to vault — no fee to relayer
    token::close_account(CpiContext::new_with_signer(
        ctx.accounts.token_program.to_account_info(),
        token::CloseAccount {
            account: ctx.accounts.executor_token_account.to_account_info(),
            destination: ctx.accounts.vault.to_account_info(),  // vault gets everything
            authority: ctx.accounts.executor.to_account_info(),
        },
        &[executor_seeds],
    ))?;
}
```

## Comparison with SPL Path (Correct)

```rust
// perps.rs:1215-1282 — SPL PATH (CORRECT)
} else {
    require!(
        executor_ata_data.amount >= gross_outflow,  // checks full amount
        PrivacyError::InsufficientFundsForWithdrawal
    );
    // Fee: executor → relayer ATA (correctly transfers fee)
    if ext_data.fee > 0 {
        token::transfer(/* executor → relayer */, ext_data.fee)?;
    }
    // Then: executor → vault (only reissue_amount)
    token::transfer(/* executor → vault */, reissue_amount)?;
    // Close ATA if fully drained
}
```

## Impact

1. **Relayer never compensated:** The relayer fronts capital for SOL-pool perp operations but never receives fees
2. **Untracked vault surplus:** Vault absorbs `reissue_amount + fee` but TVL only increases by `reissue_amount` (`perps.rs:1304-1306`), creating a growing discrepancy
3. **Economic invariant violation:** `ext_data.fee` is bound in the ZK proof but never moves on-chain

## PoC Sketch

```
1. User has SOL pool with fee_bps=10 (0.1%), min_withdrawal_fee=1_000_000
2. Open jperp position: deposit 1 SOL, position profits 0.1 SOL
3. Reissue: reissue_amount=1.1 SOL, ext_data.fee=1_100 (0.0000011 SOL)
4. ZK proof validates (fee is within bounds)
5. Executor WSOL ATA holds ~1.1 SOL
6. close_account sends entire 1.1 SOL to vault
7. Relayer receives 0 fee
8. TVL increases by 1.1 SOL, but pool accounting only tracks 1.1 SOL
9. Fee surplus of 0.0000011 SOL silently absorbed by vault
```

## Recommended Fix

```rust
if is_native_sol_pool {
    // ... existing validation ...
    require!(
        executor_wsol_data.amount >= gross_outflow,  // FIX: check full amount
        PrivacyError::InsufficientFundsForWithdrawal
    );

    // FIX: Transfer fee to relayer before closing
    if ext_data.fee > 0 {
        let expected_relayer_wsol =
            get_associated_token_address(&ctx.accounts.relayer.key(), &WSOL_MINT);
        require!(
            ctx.accounts.relayer_token_account.key() == expected_relayer_wsol,
            PrivacyError::RelayerTokenAccountMismatch
        );
        token::transfer(
            CpiContext::new_with_signer(
                ctx.accounts.token_program.to_account_info(),
                token::Transfer {
                    from: ctx.accounts.executor_token_account.to_account_info(),
                    to: ctx.accounts.relayer_token_account.to_account_info(),
                    authority: ctx.accounts.executor.to_account_info(),
                },
                &[executor_seeds],
            ),
            ext_data.fee,
        )?;
    }

    // Then close to vault (remaining balance = reissue_amount)
    token::close_account(/* ... destination: vault ... */)?;
}
```

## References

- `perps.rs:1186-1214` — Vulnerable native SOL path
- `perps.rs:1215-1282` — Correct SPL path for comparison
- `perps.rs:1304-1306` — TVL update (only adds `reissue_amount`)
- `predictions.rs:243-262` — Similar pattern (correct fee transfer for predictions)
- `phoenix.rs:1690+` — Phoenix reissue (needs similar check)
