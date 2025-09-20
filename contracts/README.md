# Conditional Tokens Framework (CTF) for Sui

A reusable Sui Move package that implements a Polymarket-style Conditional Tokens Framework, allowing users to create prediction markets and conditional token systems on the Sui blockchain.

## Overview

The Conditional Tokens Framework enables:

- **Condition Creation**: Create conditions with N outcomes (N ≥ 2)
- **Position Splitting**: Lock collateral and mint outcome position tokens
- **Position Merging**: Combine positions back to collateral before resolution
- **Resolution**: Finalize payouts for conditions via designated resolvers
- **Redemption**: Redeem winning positions for collateral after resolution

## Key Features

- **Generic Collateral**: Supports any coin type T (e.g., USDC, SUI)
- **Flexible Outcomes**: Binary (YES/NO) and multi-outcome conditions (3-7+ outcomes)
- **Fungible Positions**: Object-based fungible balances per (condition_id, index_set)
- **Bitmask Index Sets**: Efficient representation using bitmasks (YES=0b01, NO=0b10, etc.)
- **Precise Payouts**: Floor division with dust handling for accurate settlements
- **Safety First**: Comprehensive error handling and access control

## Architecture

### Core Modules

- `ctf_types.move` - Core data structures and type definitions
- `ctf_registry.move` - Condition creation and management
- `ctf_positions.move` - Position splitting, merging, and transfers
- `ctf_vault.move` - Collateral management and redemptions
- `ctf_resolver.move` - Resolution interface and admin functions
- `ctf_errors.move` - Error constants and accessor functions
- `ctf_events.move` - Event definitions for state changes

### Data Structures

```move
// A condition with multiple possible outcomes
struct Condition {
    id: UID,
    creator: address,
    resolver: address,
    outcome_count: u8,
    resolved: bool,
    payout_den: u64,
    payout_nums: vector<u64>,
}

// Vault holding collateral by condition
struct Vault<phantom T> {
    id: UID,
    locked_by_condition: Table<ID, u64>,
    total_balance: u64,
}

// Per-owner position balances
struct PositionBook<phantom T> {
    id: UID,
    owner: address,
    balances: Table<ConditionIndex, u64>,
}
```

## Quick Start

### 1. Build and Test

```bash
# Build the package
sui move build

# Run all tests
sui move test

# Run specific test modules
sui move test --filter ctf_basic_tests
sui move test --filter ctf_multi_outcome_tests
sui move test --filter ctf_edge_case_tests
```

### 2. Deploy to Network

```bash
# Deploy to devnet
sui client publish --gas-budget 20000000

# Note the Package ID from the output for use in transactions
```

### 3. Basic Usage with Sui CLI

#### Create Infrastructure

```bash
# Create condition registry (do this once)
sui client call --package $PACKAGE_ID --module ctf_registry --function create_registry --gas-budget 5000000

# Create vault for your collateral type (e.g., SUI)
sui client call --package $PACKAGE_ID --module ctf_vault --function create_vault --type-args 0x2::sui::SUI --gas-budget 5000000

# Create your position book
sui client call --package $PACKAGE_ID --module ctf_positions --function create_position_book --type-args 0x2::sui::SUI --gas-budget 5000000
```

#### Create a Binary Condition

```bash
# Create a YES/NO condition
sui client call --package $PACKAGE_ID --module ctf_registry --function create_condition \
    --args $REGISTRY_ID $RESOLVER_ADDRESS 2 \
    --gas-budget 5000000
```

#### Split Collateral into Positions

```bash
# Split 1000 MIST into YES/NO positions
sui client call --package $PACKAGE_ID --module ctf_positions --function split_position \
    --type-args 0x2::sui::SUI \
    --args $REGISTRY_ID $VAULT_ID $POSITION_BOOK_ID $CONDITION_ID "[1,2]" $COIN_ID \
    --gas-budget 5000000
```

#### Resolve and Redeem

```bash
# Resolve condition (only resolver can do this)
sui client call --package $PACKAGE_ID --module ctf_resolver --function report_payouts \
    --args $REGISTRY_ID $CONDITION_ID "[1000,0]" 1000 \
    --gas-budget 5000000

# Redeem winning YES position
sui client call --package $PACKAGE_ID --module ctf_vault --function redeem \
    --type-args 0x2::sui::SUI \
    --args $REGISTRY_ID $VAULT_ID $POSITION_BOOK_ID $CONDITION_ID 1 \
    --gas-budget 5000000
```

## Index Set Bitmasks

The CTF uses bitmasks to represent position index sets efficiently:

### Binary Conditions (2 outcomes)
- Outcome A (YES): `0b01` (1)
- Outcome B (NO): `0b10` (2)  
- All outcomes: `0b11` (3)

### Three Outcomes
- Outcome A: `0b001` (1)
- Outcome B: `0b010` (2)
- Outcome C: `0b100` (4)
- A or B: `0b011` (3)
- All outcomes: `0b111` (7)

### Example Scenarios

**Binary Market: "Will it rain tomorrow?"**
```
YES position = 0b01 (1)
NO position = 0b10 (2)
```

**Three-way Market: "Election Winner"**  
```
Alice = 0b001 (1)
Bob = 0b010 (2)  
Carol = 0b100 (4)
Alice OR Bob = 0b011 (3)
```

## Payout Mathematics

Payouts use integer arithmetic with a common denominator:

```move
// Example: 60/40 split for binary outcome
payout_nums = [600, 400]  // Numerators
payout_den = 1000         // Common denominator

// Redemption calculation (floor division)
payout_amount = (position_balance * payout_numerator) / payout_denominator
```

**Dust Handling**: Any remainder from floor division stays in the vault and can be swept by governance after a grace period.

## Access Control

- **Condition Creation**: Anyone can create conditions
- **Resolution**: Only the designated resolver can report payouts
- **Admin Functions**: Protected by AdminCap for dust sweeping and emergency functions
- **Position Management**: Only position book owners can split/merge their positions

## Safety Features

- **Arithmetic Safety**: All operations use checked arithmetic
- **Input Validation**: Comprehensive validation of index sets and amounts
- **Replay Protection**: Conditions cannot be resolved twice
- **Balance Verification**: Insufficient balance checks prevent over-spending
- **Status Guards**: Operations only allowed in appropriate condition states

## Error Handling

The framework provides detailed error codes:

```move
E_OUTCOME_COUNT_TOO_SMALL    // Need at least 2 outcomes
E_INVALID_INDEX_SETS         // Sets overlap or don't cover all outcomes  
E_NOT_RESOLVER              // Only resolver can report payouts
E_ALREADY_RESOLVED          // Cannot resolve twice
E_NOT_RESOLVED              // Cannot redeem before resolution
E_INSUFFICIENT_BALANCE      // Not enough balance for operation
E_ZERO_DENOMINATOR         // Payout denominator cannot be zero
```

## Events

All state changes emit events:

- `ConditionCreated` - New condition created
- `PositionsSplit` - Collateral split into positions  
- `PositionsMerged` - Positions merged back to collateral
- `PayoutsReported` - Condition resolved with payouts
- `Redeemed` - Position redeemed for collateral

## Testing

The package includes comprehensive test suites:

### Basic Tests (`ctf_basic_tests.move`)
- Full binary condition lifecycle
- Split → merge → resolve → redeem flow
- Basic validation tests

### Multi-outcome Tests (`ctf_multi_outcome_tests.move`)  
- 3-outcome and 5-outcome conditions
- Combination positions (e.g., A OR B)
- Complex payout scenarios

### Edge Case Tests (`ctf_edge_case_tests.move`)
- Invalid parameters and boundary conditions
- Double resolution attempts
- Insufficient balance scenarios
- Precision and rounding edge cases

## Integration Examples

### Oracle Integration
```move
// Future extension - oracle resolver
public fun oracle_resolve(
    oracle_cap: &OracleCap,
    registry: &mut ConditionRegistry,
    condition_id: ID,
    oracle_result: vector<u64>
) {
    // Fetch data from oracle, format as payouts
    ctf_resolver::report_payouts(registry, condition_id, oracle_result, 1000, ctx);
}
```

### UMA Integration
The resolver interface is designed to be extensible for UMA (Universal Market Access) integration:

```move
// Future UMA resolver implementation
module uma_resolver {
    public fun resolve_from_uma(
        registry: &mut ConditionRegistry,
        condition_id: ID,
        uma_identifier: vector<u8>,
        timestamp: u64
    ) {
        // Fetch UMA result and resolve condition
        let uma_result = fetch_uma_data(uma_identifier, timestamp);
        let payouts = format_uma_result(uma_result);
        ctf_resolver::report_payouts(registry, condition_id, payouts.nums, payouts.den, ctx);
    }
}
```

## Gas Optimization

- **Batch Operations**: Use batch functions when available
- **Efficient Storage**: Bitmask representation minimizes storage costs
- **Lazy Cleanup**: Dust sweeping is optional and governance-controlled
- **Minimal Loops**: All loops are bounded and gas-efficient

## Security Considerations

- **No External Calls**: Framework is self-contained (except for coin operations)
- **Integer Overflow Protection**: All arithmetic uses checked operations  
- **Access Control**: Proper capability-based security
- **Input Sanitization**: All inputs validated before processing
- **State Machine Safety**: Operations only allowed in valid states

## Contributing

This is a demonstration framework. For production use, consider:

- Adding more sophisticated access controls
- Implementing fee mechanisms  
- Adding time-based expiration
- Creating governance modules
- Adding more oracle integrations

## License

MIT License - see LICENSE file for details.