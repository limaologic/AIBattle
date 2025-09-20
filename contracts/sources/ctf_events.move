/// Events for the Conditional Tokens Framework
module ctf::ctf_events {
    use sui::event;
    
    /// Event emitted when a new condition registry is created
    public struct RegistryCreated has copy, drop {
        registry_id: ID,
        creator: address,
        initial_collateral: u64,
        initial_tokens_minted: u64,
    }
    
    /// Event emitted when collateral is added and tokens are minted
    public struct CollateralAdded has copy, drop {
        user: address,
        registry_id: ID,
        collateral_amount: u64,
        tokens_minted: u64,
    }
    
    /// Event emitted when tokens are redeemed for collateral
    public struct TokensRedeemed has copy, drop {
        user: address,
        registry_id: ID,
        positive_tokens_burned: u64,
        negative_tokens_burned: u64,
        collateral_withdrawn: u64,
    }
    
    
    
    /// Emit a registry created event
    public(package) fun emit_registry_created(
        registry_id: ID,
        creator: address,
        initial_collateral: u64,
        initial_tokens_minted: u64,
    ) {
        event::emit(RegistryCreated {
            registry_id,
            creator,
            initial_collateral,
            initial_tokens_minted,
        });
    }
    
    /// Emit a collateral added event
    public(package) fun emit_collateral_added(
        user: address,
        registry_id: ID,
        collateral_amount: u64,
        tokens_minted: u64,
    ) {
        event::emit(CollateralAdded {
            user,
            registry_id,
            collateral_amount,
            tokens_minted,
        });
    }
    
    /// Emit a tokens redeemed event
    public(package) fun emit_tokens_redeemed(
        user: address,
        registry_id: ID,
        positive_tokens_burned: u64,
        negative_tokens_burned: u64,
        collateral_withdrawn: u64,
    ) {
        event::emit(TokensRedeemed {
            user,
            registry_id,
            positive_tokens_burned,
            negative_tokens_burned,
            collateral_withdrawn,
        });
    }
    
}