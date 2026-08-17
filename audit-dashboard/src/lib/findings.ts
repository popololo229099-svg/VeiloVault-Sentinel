export type Severity = "HIGH" | "MEDIUM" | "LOW" | "INFO";

export interface Finding {
  id: string;
  severity: Severity;
  title: string;
  module: string;
  lines: string;
  category: string;
  description: string;
  impact: string;
  recommendation: string;
}

export const findings: Finding[] = [
  {
    id: "VV-01",
    severity: "HIGH",
    title: "Relayer fee silently skipped on native-SOL jperp_reissue_notes",
    module: "perps.rs",
    lines: "1186-1214",
    category: "Logic Error / Fee Bypass",
    description:
      "In jperp_reissue_notes, the native-SOL pool path validates ext_data.fee against the pool's fee config and ZK proof, but never actually transfers the fee to the relayer. The entire WSOL ATA balance (including the fee portion) is swept into the vault via token::close_account. Compare with the SPL path (lines 1215-1282) which correctly transfers fee from executor to relayer before moving reissue_amount to vault.",
    impact:
      "The relayer is never compensated for SOL-pool perp reissues. The vault absorbs reissue_amount + fee but TVL only increases by reissue_amount, creating untracked vault surplus. The ZK proof binds a fee that never moves, breaking the economic invariant.",
    recommendation:
      "Add fee transfer to relayer in the native-SOL jperp_reissue_notes path, mirroring the SPL path: check gross_outflow, transfer fee to relayer ATA, then close remaining balance to vault.",
  },
  {
    id: "VV-02",
    severity: "HIGH",
    title: "swap_data_hash not bound by ZK proof - relayer can substitute Jupiter routes",
    module: "swap.rs",
    lines: "46-59, 101-127, 827-828",
    category: "MEV / Relayer Trust",
    description:
      "SwapParams::hash() explicitly excludes swap_data_hash from the Poseidon hash used as a ZK proof public input (documented at swap.rs:55-57). The on-chain SHA256 check verifies SHA256(swap_data) == swap_params.swap_data_hash, but since the relayer provides both, this is trivially self-consistent. A relayer can substitute any Jupiter route with the same valid proof.",
    impact:
      "A malicious relayer can substitute the Jupiter swap route for MEV extraction, routing through pools that provide kickbacks. User is protected by min_amount_out and dest_amount (proof-bound), but receives suboptimal execution. All surplus goes to relayer.",
    recommendation:
      "Include swap_data_hash in SwapParams::hash() and re-circuit the ZK proof to include it in public inputs.",
  },
  {
    id: "VV-03",
    severity: "MEDIUM",
    title: "phoenix_queue_withdraw max_slot_amount allows arbitrary cap inflation",
    module: "phoenix.rs",
    lines: "1140-1143",
    category: "Accounting Manipulation",
    description:
      "The phoenix_queue_withdraw instruction accepts an optional max_slot_amount parameter. When provided and greater than the current slot.amount, it raises the slot cap without any on-chain proof of profitability. The slot cap is the protocol's bookkeeping guarantee preventing reissuance of more than deposited.",
    impact:
      "If Phoenix Eternal has a bug (stale margin calculation, rounding error), the inflated cap lets users mint notes for funds they never deposited. Breaks the accounting invariant.",
    recommendation:
      "Cap max_slot_amount to a bounded profit threshold, or remove the parameter entirely.",
  },
  {
    id: "VV-04",
    severity: "MEDIUM",
    title: "phoenix_ember_unwrap has no claimant signer - relayer front-running",
    module: "phoenix.rs",
    lines: "1459-1465",
    category: "Front-Running / Griefing",
    description:
      "PhoenixEmberUnwrap does not require the claimant to be a Signer. The claimant is an instruction parameter validated only against slot.claimant_pubkey (a data comparison, not a cryptographic check). Only the relayer signs the transaction. Any whitelisted relayer can call ember_unwrap for any slot.",
    impact:
      "Relayer can front-run any user's exit by calling ember_unwrap at unfavorable market conditions. Relayer controls the timing of when proceeds become available for reissue. No fund theft (USDC goes to vault), but user loses control of exit timing.",
    recommendation:
      "Add claimant: Signer<'info> to PhoenixEmberUnwrap to prevent relayer front-running.",
  },
  {
    id: "VV-05",
    severity: "MEDIUM",
    title: "Vault token accounts unconstrained for native SOL pools in TransactSwap",
    module: "lib.rs",
    lines: "918-923, 958-960",
    category: "Defense-in-Depth",
    description:
      "In the TransactSwap context, source_vault_token_account and dest_vault_token_account are UncheckedAccount with only #[account(mut)] constraints. For native SOL pools, these accounts are unused in the handler but can be any arbitrary mutable account.",
    impact:
      "Passing arbitrary accounts increases attack surface. A future code change that references these accounts for native SOL would be immediately exploitable.",
    recommendation:
      "Add conditional Anchor-level constraints or address = Pubkey::default() for native SOL contexts.",
  },
  {
    id: "VV-06",
    severity: "MEDIUM",
    title: "Jupiter Perps remaining_accounts beyond [0] not validated",
    module: "perps.rs",
    lines: "464-470, 680-685",
    category: "Account Substitution",
    description:
      "Only remaining[0] (Jupiter Perps program ID) is validated. All other 13+ accounts (perpetuals, pool, position, custody, price accounts) are passed through to the CPI without Anchor-level validation. A relayer could substitute accounts from a look-alike program.",
    impact:
      "While Jupiter's own CPI validates accounts, a malicious program accepting the same layout could behave differently. Low practical risk due to executor PDA signing, but violates defense-in-depth.",
    recommendation:
      "Validate at minimum remaining[1] against known JLP_POOL and remaining[2] against expected pool PDA.",
  },
  {
    id: "VV-07",
    severity: "MEDIUM",
    title: "Perps slot.reissued is unbounded audit counter - no overdraft guard",
    module: "perps.rs",
    lines: "1100-1104, 1475-1478",
    category: "Missing Overdraft Guard",
    description:
      "In jperp_reissue_notes and jperp_recover_native, slot.reissued is incremented as a cumulative counter but never checked against slot.amount. Compare with Phoenix's phoenix_queue_withdraw which enforces require!(new_withdrawn <= slot.amount, SlotOverdraft).",
    impact:
      "No on-chain cap on cumulative reissues per slot. A compromised claimant key could drain proceeds exceeding original deposit through repeated partial reissues.",
    recommendation:
      "Add overdraft check: require!(new_reissued <= slot.amount, SlotOverdraft) before incrementing.",
  },
  {
    id: "VV-08",
    severity: "LOW",
    title: "Executor ATA token surplus stranded on partial SPL reissues",
    module: "perps.rs",
    lines: "1271",
    category: "Fund Stranding",
    description:
      "The executor ATA is only closed when executor_ata_data.amount == gross_outflow (exact match). If a winning position's ATA holds more than gross_outflow, surplus tokens remain stranded permanently. No admin recovery path exists.",
    impact:
      "Tokens permanently locked in executor ATA if claimant key is lost after partial reissue.",
    recommendation:
      "Implement admin recovery instruction for stranded executor ATA balances after a timeout period.",
  },
  {
    id: "VV-09",
    severity: "LOW",
    title: "relayer_token_account missing mint/owner constraints in TransactSwap",
    module: "lib.rs",
    lines: "1012-1014",
    category: "Defense-in-Depth",
    description:
      "relayer_token_account is typed as Account<'info, TokenAccount> but lacks token::mint and token::authority constraints. A relayer could pass any SPL token account as their fee recipient.",
    impact:
      "Self-inflicted only (relayer passes wrong account to themselves). Defense-in-depth gap.",
    recommendation:
      "Add token::mint = dest_mint constraint.",
  },
  {
    id: "VV-10",
    severity: "LOW",
    title: "Cosigner's matching accounts marked as CPI signers in execute_jup_legs",
    module: "positions.rs",
    lines: "1866",
    category: "CPI Authorization",
    description:
      "Any account matching the cosigner's key is marked as a signer in Jupiter CPIs. If the cosigner is a malicious program, it could behave arbitrarily within Jupiter legs while being marked as a signer.",
    impact:
      "Low practical impact since all funds are swept post-swap. Potential griefing via malicious program-cosigner.",
    recommendation:
      "Restrict cosigner signer marking to wallet accounts only.",
  },
  {
    id: "VV-11",
    severity: "INFO",
    title: "reduce_to_field returns modulus instead of 0 at boundary",
    module: "swap.rs",
    lines: "63-99",
    category: "Edge Case",
    description:
      "When input bytes exactly equal the BN254 Fr modulus, the function returns the modulus itself instead of [0u8; 32]. Probability of occurrence with a Solana public key: ~2^-254 (negligible).",
    impact: "Negligible probability in practice.",
    recommendation: "Fix the boundary condition for correctness.",
  },
  {
    id: "VV-12",
    severity: "INFO",
    title: "Hardcoded byte offsets in fund_native_open_position pairing guard",
    module: "positions.rs",
    lines: "2362-2390",
    category: "Maintainability",
    description:
      "Hardcoded byte offsets (OPEN_POSITION_EXECUTOR_IDX = 15, etc.) verify the paired open_position instruction. If the OpenPosition account layout changes, these silently become wrong.",
    impact: "Fragile to future account layout changes.",
    recommendation: "Add comments documenting expected layout or use dynamic parsing.",
  },
];

export const stats = {
  total: findings.length,
  high: findings.filter((f) => f.severity === "HIGH").length,
  medium: findings.filter((f) => f.severity === "MEDIUM").length,
  low: findings.filter((f) => f.severity === "LOW").length,
  info: findings.filter((f) => f.severity === "INFO").length,
};
