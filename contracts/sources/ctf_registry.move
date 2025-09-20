/// Condition registry for the Conditional Tokens Framework
module ctf::ctf_registry {
    use sui::coin::{Self, TreasuryCap, Coin};
    use sui::sui;
    use sui::balance;
    use ctf::ctf_events;
    use ctf::ctf_errors;

    /// Global registry for exactly 2 conditions (positive and negative)
    /// this Global registry should be represent as a question
    /// so we don't need to track which user owns which position/condition. 
    /// We can simply verify it by how many token do they have
    /// In the mean time, adding collateral_balance is allowed, but it should not change the ratio between condition_positive and condition_negative
    public struct ConditionRegistry<phantom TreasuryCapPositive, phantom TreasuryCapNegative, phantom CoinTypeCollateral> has key {
        id: UID,
        /// Positive condition treasury capability
        condition_positive: coin::TreasuryCap<TreasuryCapPositive>,
        /// Negative condition treasury capability
        condition_negative: coin::TreasuryCap<TreasuryCapNegative>,
        /// Collateral balance backing the conditions
        collateral_balance: balance::Balance<CoinTypeCollateral>,
    }

    /// Initialize the condition registry with treasury capabilities and initial collateral
    public fun create_registry<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>(
        cap_positive: TreasuryCap<TreasuryCapPositive>,
        cap_negative: TreasuryCap<TreasuryCapNegative>,
        collateral_coin: Coin<CoinTypeCollateral>,
        ctx: &mut TxContext
    ): (
        coin::Coin<TreasuryCapPositive>,
        coin::Coin<TreasuryCapNegative>,
    ) {
        let collateral_amount = coin::value(&collateral_coin);
        
        let mut registry = ConditionRegistry<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral> {
            id: object::new(ctx),
            condition_positive: cap_positive,
            condition_negative: cap_negative,
            collateral_balance: collateral_coin.into_balance(),
        };
        
        // Mint equal amounts of positive and negative tokens based on collateral
        let positive_tokens = coin::mint(&mut registry.condition_positive, collateral_amount, ctx);
        let negative_tokens = coin::mint(&mut registry.condition_negative, collateral_amount, ctx);
        
        // Transfer tokens to the registry creator
        let sender = tx_context::sender(ctx);
        
        // Emit registry created event
        let registry_id = object::id(&registry);
        ctf_events::emit_registry_created(registry_id, sender, collateral_amount, collateral_amount);
        
        transfer::share_object(registry);

        return (positive_tokens, negative_tokens)
    }
    
    /// Add collateral to the registry and mint tokens for the sender
    public fun add_collateral<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>(
        registry: &mut ConditionRegistry<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>,
        collateral_coin: Coin<CoinTypeCollateral>,
        ctx: &mut TxContext,
    ): (coin::Coin<TreasuryCapPositive>, coin::Coin<TreasuryCapNegative>) {
        let collateral_amount = coin::value(&collateral_coin);
        assert!(collateral_amount > 0, ctf_errors::invalid_amount());
        
        // Add collateral to the balance
        balance::join(&mut registry.collateral_balance, coin::into_balance(collateral_coin));
        
        // Mint equal amounts of positive and negative tokens
        let positive_tokens = coin::mint(&mut registry.condition_positive, collateral_amount, ctx);
        let negative_tokens = coin::mint(&mut registry.condition_negative, collateral_amount, ctx);
        
        // Emit collateral added event
        let sender = tx_context::sender(ctx);
        let registry_id = object::id(registry);
        ctf_events::emit_collateral_added(sender, registry_id, collateral_amount, collateral_amount);
        
        return (positive_tokens, negative_tokens)
    }
    
    /// Redeem tokens for collateral
    public fun redeem_tokens<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>(
        registry: &mut ConditionRegistry<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>,
        positive_tokens: Coin<TreasuryCapPositive>,
        negative_tokens: Coin<TreasuryCapNegative>,
        ctx: &mut TxContext,
    ): coin::Coin<CoinTypeCollateral> {
        let positive_amount = coin::value(&positive_tokens);
        let negative_amount = coin::value(&negative_tokens);
        
        // Amounts must be equal for redemption
        assert!(positive_amount == negative_amount, ctf_errors::invalid_amount());
        assert!(positive_amount > 0, ctf_errors::invalid_amount());
        
        // Check we have enough collateral
        assert!(balance::value(&registry.collateral_balance) >= positive_amount, ctf_errors::insufficient_balance());
        
        // Burn the tokens
        coin::burn(&mut registry.condition_positive, positive_tokens);
        coin::burn(&mut registry.condition_negative, negative_tokens);
        
        // Withdraw collateral
        let collateral_balance = balance::split(&mut registry.collateral_balance, positive_amount);
        let collateral_coin = coin::from_balance(collateral_balance, ctx);
        
        // Emit tokens redeemed event
        let sender = tx_context::sender(ctx);
        let registry_id = object::id(registry);
        ctf_events::emit_tokens_redeemed(sender, registry_id, positive_amount, negative_amount, positive_amount);
        
        collateral_coin
    }
    
    /// Get token supply information
    public fun get_token_supplies<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>(
        registry: &ConditionRegistry<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>
    ): (u64, u64) {
        (
            coin::total_supply(&registry.condition_positive),
            coin::total_supply(&registry.condition_negative)
        )
    }
    
    /// Get total collateral balance in the registry
    public fun get_collateral_balance<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>(
        registry: &ConditionRegistry<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>
    ): u64 {
        balance::value(&registry.collateral_balance)
    }

    public struct ChallengeCommitment has key, store {
        id: UID,
        registry_id: ID,
        challenger_addr: address,
        solver_addr: address,
        score: u64,
        timestamp: u64,
        commitment: vector<u8>,
    }

    public fun upload_challenge_commitment<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>(
        registry: &ConditionRegistry<TreasuryCapPositive, TreasuryCapNegative, CoinTypeCollateral>,
        commitment: vector<u8>,
        challenger_addr: address,
        solver_addr: address,
        score: u64,
        timestamp: u64,
        ctx: &mut TxContext,
    ): ChallengeCommitment {
        ChallengeCommitment {
            id: object::new(ctx),
            registry_id: object::id(registry),
            challenger_addr: challenger_addr,
            solver_addr: solver_addr,
            score: score,
            timestamp: timestamp,
            commitment: commitment,
        }
    }

    public struct Vault has key, store {
        id: UID,
        bounty_balance: balance::Balance<sui::SUI>,
        admin_cap_id: ID,
    }

    public struct VaultAdminCap has key, store {
        id: UID,
    }

    public fun create_vault(
        ctx: &mut tx_context::TxContext,
    ): VaultAdminCap {
        let admin_cap = VaultAdminCap {
            id: object::new(ctx),
        };

       let vault = Vault {
            id: object::new(ctx),
            bounty_balance: balance::zero(),
            admin_cap_id: object::id(&admin_cap),
        };
        transfer::share_object(vault);

        return (admin_cap)
    }

    public fun vault_add_bounty(
        vault: &mut Vault,
        bounty: Coin<sui::SUI>,
    ) {
        vault.bounty_balance.join(bounty.into_balance());
    }

    public fun vault_transfer_bounty(
        vault: &mut Vault,
        admin_cap: &VaultAdminCap,
        ctx: &mut tx_context::TxContext,
    ): Coin<sui::SUI> {
        assert!(admin_cap.id.to_address() == vault.admin_cap_id.to_address(), 1);
        let bal = vault.bounty_balance.value();
        let coin = coin::take(&mut vault.bounty_balance, bal, ctx);
        return (coin)
    }


    public struct FakeSwapPool<phantom CoinTypePositive, phantom CoinTypeNegative> has key {
        id: UID,
        reserve: balance::Balance<sui::SUI>,
        pos_pool: balance::Balance<CoinTypePositive>,
        neg_pool: balance::Balance<CoinTypeNegative>,
    }

    public fun create_fake_swap_pool<CoinTypePositive, CoinTypeNegative>(
        initial_reserve: Coin<sui::SUI>,
        initial_pos: Coin<CoinTypePositive>,
        initial_neg: Coin<CoinTypeNegative>,
        ctx: &mut tx_context::TxContext,
    ) {
        let fake_swap_pool = FakeSwapPool {
            id: object::new(ctx),
            reserve: initial_reserve.into_balance(),
            pos_pool: initial_pos.into_balance(),
            neg_pool: initial_neg.into_balance(),
        };
        transfer::share_object(fake_swap_pool);
    }

    public fun fake_swap_pool_add_coins<CoinTypePositive, CoinTypeNegative>(
        fake_swap_pool: &mut FakeSwapPool<CoinTypePositive, CoinTypeNegative>,
        additional_reserve: Option<Coin<sui::SUI>>,
        additional_pos: Option<Coin<CoinTypePositive>>,
        additional_neg: Option<Coin<CoinTypeNegative>>,
    ) {
        if (option::is_some(&additional_reserve)) {
            let bal = option::destroy_some(additional_reserve).into_balance();
            fake_swap_pool.reserve.join(bal);
        } else { option::destroy_none(additional_reserve) };
        if (option::is_some(&additional_pos)) {
            let bal = option::destroy_some(additional_pos).into_balance();
            fake_swap_pool.pos_pool.join(bal);
        } else { option::destroy_none(additional_pos) };
        if (option::is_some(&additional_neg)) {
            let bal = option::destroy_some(additional_neg).into_balance();
            fake_swap_pool.neg_pool.join(bal);
        } else { option::destroy_none(additional_neg) };
    }

    public fun fake_swap_buy<CoinTypePositive, CoinTypeNegative>(
        fake_swap_pool: &mut FakeSwapPool<CoinTypePositive, CoinTypeNegative>,
        input_reserve: Option<Coin<sui::SUI>>,
        output_reserve_amount: u64, 
        input_pos: Option<Coin<CoinTypePositive>>,
        output_pos_amount: u64, 
        input_neg: Option<Coin<CoinTypeNegative>>,
        output_neg_amount: u64, 
        ctx: &mut tx_context::TxContext,
    ): (
        Option<Coin<sui::SUI>>,
        Option<Coin<CoinTypePositive>>,
        Option<Coin<CoinTypeNegative>>,
    ) {
        if (option::is_some(&input_reserve)) {
            let bal = option::destroy_some(input_reserve).into_balance();
            fake_swap_pool.reserve.join(bal);
        } else { option::destroy_none(input_reserve) };
        if (option::is_some(&input_pos)) {
            let bal = option::destroy_some(input_pos).into_balance();
            fake_swap_pool.pos_pool.join(bal);
        } else { option::destroy_none(input_pos) };
        if (option::is_some(&input_neg)) {
            let bal = option::destroy_some(input_neg).into_balance();
            fake_swap_pool.neg_pool.join(bal);
        } else { option::destroy_none(input_neg) };

        let output_reserve = if (output_reserve_amount > 0) {
            let bal = balance::split(&mut fake_swap_pool.reserve, output_reserve_amount);
            option::some(coin::from_balance(bal, ctx))
        } else { option::none() };
        let output_pos = if (output_pos_amount > 0) {
            let bal = balance::split(&mut fake_swap_pool.pos_pool, output_pos_amount);
            option::some(coin::from_balance(bal, ctx))
        } else { option::none() };
        let output_neg = if (output_neg_amount > 0) {
            let bal = balance::split(&mut fake_swap_pool.neg_pool, output_neg_amount);
            option::some(coin::from_balance(bal, ctx))
        } else { option::none() };
        (output_reserve, output_pos, output_neg)
    }
}