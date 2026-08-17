# VV-02: swap_data_hash not bound by ZK proof

**Severity:** HIGH  
**Module:** `swap.rs`, `positions.rs`  
**Lines:** swap.rs:46-59, swap.rs:101-127, positions.rs:2000-2001  
**Category:** MEV / Relayer Trust  
**Date:** 2026-08-17

---

## Description

The `SwapParams::hash()` function explicitly excludes `swap_data_hash` from the Poseidon hash used as a ZK proof public input. The code comments document this:

```rust
// swap.rs:55-57
// NOTE: `swap_data_hash` is not proof-bound until the circuit and verifying key
// are upgraded to include it in `swap_params_hash`.  For now the on-chain check
// `SHA256(swap_data) == swap_data_hash` is internal consistency only.
```

Since the relayer provides both `swap_params` and `swap_data`, the on-chain SHA256 check at `swap.rs:827-828` is trivially self-consistent. A relayer can substitute any swap route.

## Vulnerable Code

```rust
// swap.rs:115-127 — hash() excludes swap_data_hash
pub fn hash(&self, source_mint: &Pubkey, dest_mint: &Pubkey) -> Result<[u8; 32]> {
    let mut h = PoseidonHasher::new();
    h.write_field(&source_mint.to_bytes())?;
    h.write_field(&dest_mint.to_bytes())?;
    h.write_u64(self.min_amount_out)?;
    h.write_u64(self.deadline)?;
    h.write_u64(self.dest_amount)?;
    // swap_data_hash is NOT hashed here
    Ok(h.finalize())
}

// swap.rs:827-828 — self-consistent check
let computed: [u8; 32] = solana_sha256_hasher::hash(&swap_data).to_bytes();
require!(computed == swap_params.swap_data_hash, ...);
```

## Attack Flow

```
1. User generates valid ZK proof with swap_params_hash = H(source_mint, dest_mint, min_amount_out, deadline, dest_amount)
2. User sends proof + swap_params to relayer
3. Relayer replaces swap_params.swap_data with a route that provides MEV/kickbacks
4. Relayer sets swap_params.swap_data_hash = SHA256(new_swap_data)
5. On-chain: SHA256(new_swap_data) == swap_params.swap_data_hash ✓ (self-consistent)
6. On-chain: verify_swap_groth16(proof, swap_params_hash) ✓ (unchanged)
7. User gets at least min_amount_out (proof-bound), but relayer captures all price improvement
```

## Impact

- **Relayer MEV extraction:** Captures all surplus between optimal route and suboptimal route
- **User receives suboptimal execution:** Gets minimum guaranteed amount, not best available price
- **No direct fund loss:** User's `min_amount_out` and `dest_amount` are proof-bound

## Recommended Fix

Include `swap_data_hash` in `SwapParams::hash()`:

```rust
pub fn hash(&self, source_mint: &Pubkey, dest_mint: &Pubkey) -> Result<[u8; 32]> {
    let mut h = PoseidonHasher::new();
    h.write_field(&source_mint.to_bytes())?;
    h.write_field(&dest_mint.to_bytes())?;
    h.write_u64(self.min_amount_out)?;
    h.write_u64(self.deadline)?;
    h.write_u64(self.dest_amount)?;
    h.write_field(&self.swap_data_hash)?;  // FIX: include swap_data_hash
    Ok(h.finalize())
}
```

**Prerequisite:** ZK circuit and verifying key must be updated to include `swap_data_hash` in public inputs.

## References

- `swap.rs:46-59` — Comment documenting the exclusion
- `swap.rs:101-127` — `SwapParams::hash()` implementation
- `swap.rs:827-828` — Self-consistent SHA256 check
- `positions.rs:2000-2001` — Same hash function used in position swaps
