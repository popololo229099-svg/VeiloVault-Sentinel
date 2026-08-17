# Veilo SDK

Go SDK for interacting with the Veilo Privacy Pool program on Solana.

## Features

- **Type-safe** Go client for all Veilo instructions
- **PDA derivation** for all program accounts
- **Transaction building** with proper instruction serialization
- **Zero-knowledge proof** integration helpers
- **Real-time monitoring** via WebSocket subscriptions

## Installation

```bash
go get github.com/popolo229099-svg/veilo-sdk
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/gagliardetto/solana-go"
    "github.com/popolo229099-svg/veilo-sdk/pkg/client"
    "github.com/popolo229099-svg/veilo-sdk/pkg/types"
)

func main() {
    // Create client
    config := client.DefaultConfig()
    config.RPCEndpoint = "https://api.mainnet-beta.solana.com"
    c := client.NewClient(config)
    defer c.Close()

    // Get pool status
    ctx := context.Background()
    mint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
    
    status, err := c.GetPoolStatus(ctx, mint)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Pool Balance: %d lamports\n", status.Balance)
    fmt.Printf("Pool Healthy: %v\n", status.IsHealthy)
}
```

## API Reference

### Client

```go
// Create a new client
client := client.NewClient(client.Config{
    RPCEndpoint: "https://api.mainnet-beta.solana.com",
    WSEndpoint:  "wss://api.mainnet-beta.solana.com",
    ProgramID:   types.ProgramID,
    Commitment:  rpc.CommitmentFinalized,
})

// Get pool configuration
config, err := client.GetPoolConfig(ctx, mint)

// Get vault balance
balance, err := client.GetVaultBalance(ctx, mint)

// Check if nullifier is spent
spent, err := client.IsNullifierSpent(ctx, nullifier)

// Get recent slot
slot, err := client.GetRecentSlot(ctx)

// Get health status
health, err := client.Health(ctx)
```

### Types

```go
// Pool configuration
type PoolConfig struct {
    Authority          solana.PublicKey
    Mint               solana.PublicKey
    Vault              solana.PublicKey
    FeeBPS             uint16
    MinWithdrawalFee   uint64
    IsNativeSOL        bool
}

// Swap parameters
type SwapParams struct {
    AmountIn     uint64
    MinAmountOut uint64
    Fee          uint64
    Recipient    solana.PublicKey
    Relayer      solana.PublicKey
    SwapDataHash [32]byte
}

// Transaction request
type TransactionRequest struct {
    Type        string
    Pool        solana.PublicKey
    Proof       []byte
    PublicInputs [][]byte
    ExtData     *ExtData
    SwapParams  *SwapParams
}
```

### PDA Helpers

```go
// Find pool config PDA
pda, bump, err := types.FindPoolConfigPDA(mint)

// Find vault PDA
vault, bump, err := types.FindVaultPDA(mint)

// Find nullifier PDA
nullifierPDA, bump, err := types.FindNullifierPDA(nullifier)

// Find position pool PDA
positionPDA, bump, err := types.FindPositionPoolPDA(mintA, mintB)

// Find Phoenix slot PDA
slotPDA, bump, err := types.FindPhoenixSlotPDA(pool, user)
```

## Program ID

```
GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU
```

## License

MIT
