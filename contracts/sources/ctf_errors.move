/// Error codes for the Conditional Tokens Framework
module ctf::ctf_errors {
    /// The outcome count must be at least 2
    const E_OUTCOME_COUNT_TOO_SMALL: u64 = 0;
    /// The condition has already been resolved
    const E_ALREADY_RESOLVED: u64 = 3;
    /// Insufficient balance for the operation
    const E_INSUFFICIENT_BALANCE: u64 = 5;
    /// The denominator cannot be zero
    const E_ZERO_DENOMINATOR: u64 = 6;
    /// The payout vector is invalid (wrong length or values)
    const E_PAYOUT_VECTOR_INVALID: u64 = 7;
    /// Invalid amount (zero or negative)
    const E_INVALID_AMOUNT: u64 = 9;
    
    // Public accessor functions for error codes
    public fun outcome_count_too_small(): u64 { E_OUTCOME_COUNT_TOO_SMALL }
    public fun already_resolved(): u64 { E_ALREADY_RESOLVED }
    public fun insufficient_balance(): u64 { E_INSUFFICIENT_BALANCE }
    public fun zero_denominator(): u64 { E_ZERO_DENOMINATOR }
    public fun payout_vector_invalid(): u64 { E_PAYOUT_VECTOR_INVALID }
    public fun invalid_amount(): u64 { E_INVALID_AMOUNT }
}