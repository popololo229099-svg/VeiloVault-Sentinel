# Security Audit Tests
# These are conceptual test outlines for verifying the identified vulnerabilities.
# They require a running Solana test validator with the program deployed.

## Test Environment Requirements

1. Solana test validator with the privacy-pool program deployed
2. USDC mint and SOL pool initialized
3. Relayer keypair registered
4. ZK proving key infrastructure (for generating valid proofs)

---

## VV-01 Test: Native-SOL JPerp Reissue Fee Skip

```typescript
// tests/vv-01-perps-native-reissue-fee-skip.ts
import * as anchor from "@coral-xyz/anchor";
import { assert } from "chai";

describe("VV-01: Perps native reissue fee skip", () => {
  it("should transfer fee to relayer on native-SOL reissue (current: does not)", async () => {
    // Setup:
    // 1. Initialize SOL privacy pool with fee_bps=10, min_withdrawal_fee=1_000_000
    // 2. Deposit SOL into pool
    // 3. Open jperp position (mock Jupiter perps CPI or use local program stub)
    // 4. Simulate position profit (executor WSOL ATA has more than deposited)
    //
    // Execute reissue:
    // 5. Generate valid ZK proof with public_amount = -(reissue_amount + fee) as i64
    // 6. Call jperp_reissue_notes with ext_data.fee = min_withdrawal_fee
    //
    // Assert (expected bug behavior):
    // 7. Assert relayer WSOL ATA balance DID NOT increase (fee was skipped)
    // 8. Assert vault balance increased by reissue_amount + fee (vault absorbed fee)
    // 9. Assert TVL only increased by reissue_amount (not gross_outflow)
    //
    // After fix, assertions should be:
    // 7. Assert relayer WSOL ATA balance increased by ext_data.fee
    // 8. Assert vault balance increased by reissue_amount (not fee)
    // 9. Assert TVL increased by reissue_amount
  });
});
```

---

## VV-02 Test: swap_data_hash Route Substitution

```typescript
// tests/vv-02-swap-data-hash-route-substitution.ts
describe("VV-02: swap_data_hash not proof-bound", () => {
  it("relayer can substitute swap route with same valid proof", async () => {
    // Setup:
    // 1. User generates ZK proof with swap_params (min_amount_out=100, dest_amount=95)
    // 2. User sends proof + swap_params to relayer
    //
    // Attack:
    // 3. Relayer creates alternative swap_data routing through a MEV pool
    // 4. Relayer sets swap_params.swap_data_hash = SHA256(alternative_swap_data)
    // 5. Relayer submits transact_swap with:
    //    - Same valid proof (swap_params_hash unchanged)
    //    - Modified swap_params (new swap_data_hash)
    //    - Alternative swap_data
    //
    // Assert:
    // 6. Transaction succeeds (self-consistent swap_data_hash check passes)
    // 7. User receives at least min_amount_out (proof-bound)
    // 8. But actual execution was suboptimal (relayer captured surplus)
  });
});
```

---

## VV-03 Test: Phoenix Slot Cap Inflation

```typescript
// tests/vv-03-phoenix-slot-cap-inflation.ts
describe("VV-03: Phoenix slot cap inflation", () => {
  it("should reject max_slot_amount exceeding deposit + bounded profit", async () => {
    // Setup:
    // 1. User deposits 1000 USDC into Phoenix via phoenix_deposit_from_pool
    // 2. Slot PDA created with amount=1000
    //
    // Attack:
    // 3. Call phoenix_queue_withdraw with max_slot_amount = u64::MAX
    // 4. Verify slot.amount is now u64::MAX (inflated)
    //
    // After fix:
    // 3. Call phoenix_queue_withdraw with max_slot_amount = u64::MAX
    // 4. Transaction should fail with SlotOverdraft or similar error
  });
});
```

---

## VV-04 Test: Phoenix Ember Unwrap No Claimant Signer

```typescript
// tests/vv-04-phoenix-ember-unwrap-front-run.ts
describe("VV-04: Phoenix ember unwrap front-running", () => {
  it("relayer can call ember_unwrap without claimant signature", async () => {
    // Setup:
    // 1. User deposits into Phoenix, slot PDA exists
    // 2. User queues withdrawal (phoenix_queue_withdraw)
    //
    // Front-run:
    // 3. Relayer calls phoenix_ember_unwrap (only relayer signs)
    // 4. Verify: USDC moved to vault, pending_reissue.amount updated
    // 5. Verify: claimant did NOT sign this transaction
    //
    // After fix:
    // 3. Transaction should fail because claimant is not a signer
  });
});
```

---

## VV-07 Test: Perps Slot Reissued Overdraft

```typescript
// tests/vv-07-perps-slot-reissued-overdraft.ts
describe("VV-07: Perps slot.reissued unbounded", () => {
  it("should reject reissue exceeding original deposit", async () => {
    // Setup:
    // 1. Deposit 100 USDC via jperp_open_position
    // 2. Slot PDA created with amount=100
    //
    // Attack:
    // 3. Mock Jupiter perps settlement (executor has 120 USDC)
    // 4. First reissue: 60 USDC (succeeds, slot.reissued=60)
    // 5. Second reissue: 60 USDC (should fail: 60+60 > 100)
    //
    // Current behavior:
    // 5. Second reissue SUCCEEDS (no overdraft check)
    //
    // After fix:
    // 5. Second reissue FAILS with SlotOverdraft
  });
});
```
