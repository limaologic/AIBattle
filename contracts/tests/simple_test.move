#[test_only]
module ctf::simple_test {
    use sui::coin;
    use sui::test_scenario::{Self};
    use ctf::ctf_registry;
    
    const ADMIN: address = @0xAD;
    const USER: address = @0x1;
    const INITIAL_COLLATERAL: u64 = 1000_000_000; // 1000 TEST tokens
    
    public struct TEST_COIN has drop {}
    public struct POSITIVE_TOKEN has drop {}
    public struct NEGATIVE_TOKEN has drop {}

    #[test]
    fun test_simple_registry_creation() {
        let mut scenario = test_scenario::begin(ADMIN);
        let scenario_ref = &mut scenario;
        
        // Step 1: Create treasury caps and collateral coin
        test_scenario::next_tx(scenario_ref, ADMIN);
        let ctx = test_scenario::ctx(scenario_ref);
        let mut collateral_treasury = coin::create_treasury_cap_for_testing<TEST_COIN>(ctx);
        let positive_treasury = coin::create_treasury_cap_for_testing<POSITIVE_TOKEN>(ctx);  
        let negative_treasury = coin::create_treasury_cap_for_testing<NEGATIVE_TOKEN>(ctx);
        let collateral_coin = collateral_treasury.mint(INITIAL_COLLATERAL, ctx);
        
        // Step 2: Create condition registry
        test_scenario::next_tx(scenario_ref, ADMIN);
        {
            let (registry, pos_tokens, neg_tokens) = ctf_registry::create_registry<POSITIVE_TOKEN, NEGATIVE_TOKEN, TEST_COIN>(
                positive_treasury,
                negative_treasury, 
                collateral_coin,
                test_scenario::ctx(scenario_ref)
            );
            // Verify token supplies and collateral balance directly here instead of in next step
            let (pos_supply, neg_supply) = ctf_registry::get_token_supplies(&registry);
            assert!(pos_supply == INITIAL_COLLATERAL, 0);
            assert!(neg_supply == INITIAL_COLLATERAL, 1);
            let collateral_balance = ctf_registry::get_collateral_balance(&registry);
            assert!(collateral_balance == INITIAL_COLLATERAL, 2);
            
            sui::test_utils::destroy(registry);
            sui::transfer::public_transfer(pos_tokens, ADMIN);
            sui::transfer::public_transfer(neg_tokens, ADMIN);
        };
        
        // Step 3: Verify admin received the initial tokens
        test_scenario::next_tx(scenario_ref, ADMIN);
        {
            let positive_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            let negative_tokens = test_scenario::take_from_sender<coin::Coin<NEGATIVE_TOKEN>>(scenario_ref);
            
            assert!(coin::value(&positive_tokens) == INITIAL_COLLATERAL, 3);
            assert!(coin::value(&negative_tokens) == INITIAL_COLLATERAL, 4);
            
            test_scenario::return_to_sender(scenario_ref, positive_tokens);
            test_scenario::return_to_sender(scenario_ref, negative_tokens);            
        };
        
        transfer::public_transfer(collateral_treasury, ADMIN);
        test_scenario::end(scenario);
    }
    
    #[test]
    fun test_add_collateral_and_mint_tokens() {
        let mut scenario = test_scenario::begin(ADMIN);
        let scenario_ref = &mut scenario;
        
        let additional_collateral = 500_000_000; // 500 TEST tokens
        
        // Setup - create registry with initial collateral
        test_scenario::next_tx(scenario_ref, ADMIN);
        let ctx = test_scenario::ctx(scenario_ref);
        let mut collateral_treasury = coin::create_treasury_cap_for_testing<TEST_COIN>(ctx);
        let positive_treasury = coin::create_treasury_cap_for_testing<POSITIVE_TOKEN>(ctx);  
        let negative_treasury = coin::create_treasury_cap_for_testing<NEGATIVE_TOKEN>(ctx);
        let initial_collateral = coin::mint(&mut collateral_treasury, INITIAL_COLLATERAL, ctx);
        
        test_scenario::next_tx(scenario_ref, ADMIN);
        {
            let (mut registry, pos_tokens, neg_tokens) = ctf_registry::create_registry<POSITIVE_TOKEN, NEGATIVE_TOKEN, TEST_COIN>(
                positive_treasury,
                negative_treasury, 
                initial_collateral,
                test_scenario::ctx(scenario_ref)
            );
            transfer::public_transfer(pos_tokens, ADMIN);
            transfer::public_transfer(neg_tokens, ADMIN);
            
            // User adds more collateral in same transaction
            let additional_coin = coin::mint(&mut collateral_treasury, additional_collateral, test_scenario::ctx(scenario_ref));
            let (pos_tokens2, neg_tokens2) = ctf_registry::add_collateral(&mut registry, additional_coin, test_scenario::ctx(scenario_ref));
            sui::transfer::public_transfer(pos_tokens2, USER);
            sui::transfer::public_transfer(neg_tokens2, USER);
            
            // Verify increased supplies and collateral
            let expected_total = INITIAL_COLLATERAL + additional_collateral;
            let (pos_supply, neg_supply) = ctf_registry::get_token_supplies(&registry);
            let collateral_balance = ctf_registry::get_collateral_balance(&registry);
            assert!(pos_supply == expected_total, 0);
            assert!(neg_supply == expected_total, 1);
            assert!(collateral_balance == expected_total, 2);
            
            sui::test_utils::destroy(registry);
        };
        
        
        // Verify user received new tokens
        test_scenario::next_tx(scenario_ref, USER);
        {
            let positive_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            let negative_tokens = test_scenario::take_from_sender<coin::Coin<NEGATIVE_TOKEN>>(scenario_ref);
            
            assert!(coin::value(&positive_tokens) == additional_collateral, 3);
            assert!(coin::value(&negative_tokens) == additional_collateral, 4);
            
            test_scenario::return_to_sender(scenario_ref, positive_tokens);
            test_scenario::return_to_sender(scenario_ref, negative_tokens);
        };
        
        // Clean up treasury cap
        sui::transfer::public_transfer(collateral_treasury, ADMIN);
        test_scenario::end(scenario);
    }
    
    #[test]
    fun test_redeem_tokens_for_collateral() {
        let mut scenario = test_scenario::begin(ADMIN);
        let scenario_ref = &mut scenario;
        
        let redeem_amount = 300_000_000; // 300 TEST tokens worth
        
        // Setup - create registry with initial collateral  
        test_scenario::next_tx(scenario_ref, ADMIN);
        let ctx = test_scenario::ctx(scenario_ref);
        let mut collateral_treasury = coin::create_treasury_cap_for_testing<TEST_COIN>(ctx);
        let positive_treasury = coin::create_treasury_cap_for_testing<POSITIVE_TOKEN>(ctx);  
        let negative_treasury = coin::create_treasury_cap_for_testing<NEGATIVE_TOKEN>(ctx);
        let initial_collateral = collateral_treasury.mint(INITIAL_COLLATERAL, ctx);
        
        test_scenario::next_tx(scenario_ref, ADMIN);
        {
            let (mut registry, mut positive_tokens, mut negative_tokens) = ctf_registry::create_registry<POSITIVE_TOKEN, NEGATIVE_TOKEN, TEST_COIN>(
                positive_treasury,
                negative_treasury, 
                initial_collateral,
                test_scenario::ctx(scenario_ref)
            );
            
            // Admin redeems tokens for collateral in same transaction
            let pos_to_redeem = coin::split(&mut positive_tokens, redeem_amount, test_scenario::ctx(scenario_ref));
            let neg_to_redeem = coin::split(&mut negative_tokens, redeem_amount, test_scenario::ctx(scenario_ref));
            
            let collateral_coin = ctf_registry::redeem_tokens(&mut registry, pos_to_redeem, neg_to_redeem, test_scenario::ctx(scenario_ref));
            
            // Verify reduced supplies and collateral
            let expected_remaining = INITIAL_COLLATERAL - redeem_amount;
            let (pos_supply, neg_supply) = ctf_registry::get_token_supplies(&registry);
            let collateral_balance = ctf_registry::get_collateral_balance(&registry);
            assert!(pos_supply == expected_remaining, 0);
            assert!(neg_supply == expected_remaining, 1);
            assert!(collateral_balance == expected_remaining, 2);
            
            // Verify admin received collateral back
            assert!(coin::value(&collateral_coin) == redeem_amount, 3);
            
            sui::transfer::public_transfer(collateral_coin, ADMIN);
            sui::transfer::public_transfer(positive_tokens, ADMIN);
            sui::transfer::public_transfer(negative_tokens, ADMIN);
            sui::test_utils::destroy(registry);
        };
        
        
        transfer::public_transfer(collateral_treasury, ADMIN);
        test_scenario::end(scenario);
    }
}