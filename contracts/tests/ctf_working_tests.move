#[test_only]
module ctf::ctf_working_tests {
    use sui::coin;
    use sui::test_scenario::{Self};
    use ctf::ctf_registry;
    use ctf::ctf_errors;
    
    const ADMIN: address = @0xAD;
    const USER1: address = @0x1;
    const USER2: address = @0x2;
    const INITIAL_COLLATERAL: u64 = 1000_000_000; // 1000 TEST tokens
    const ADDITIONAL_COLLATERAL: u64 = 500_000_000; // 500 TEST tokens
    const TRADE_AMOUNT: u64 = 200_000_000; // 200 tokens

    public struct TEST_COIN has drop {}
    public struct POSITIVE_TOKEN has drop {}
    public struct NEGATIVE_TOKEN has drop {}

    #[test]
    fun test_full_token_lifecycle() {
        let mut scenario = test_scenario::begin(ADMIN);
        let scenario_ref = &mut scenario;
        
        // Step 1: Create treasury caps and initial setup
        test_scenario::next_tx(scenario_ref, ADMIN);
        let ctx = test_scenario::ctx(scenario_ref);
        let mut collateral_treasury = coin::create_treasury_cap_for_testing<TEST_COIN>(ctx);
        let positive_treasury = coin::create_treasury_cap_for_testing<POSITIVE_TOKEN>(ctx);  
        let negative_treasury = coin::create_treasury_cap_for_testing<NEGATIVE_TOKEN>(ctx);
        let initial_collateral = coin::mint(&mut collateral_treasury, INITIAL_COLLATERAL, ctx);
        
        // Step 2: Create condition registry and add collateral in same transaction
        test_scenario::next_tx(scenario_ref, USER1);
        {
            let (mut registry, pos_tokens, neg_tokens) = ctf_registry::create_registry<POSITIVE_TOKEN, NEGATIVE_TOKEN, TEST_COIN>(
                positive_treasury,
                negative_treasury, 
                initial_collateral,
                test_scenario::ctx(scenario_ref)
            );
            sui::transfer::public_transfer(pos_tokens, ADMIN);
            sui::transfer::public_transfer(neg_tokens, ADMIN);
            
            // Add additional collateral in same transaction
            let additional_coin = coin::mint(&mut collateral_treasury, ADDITIONAL_COLLATERAL, test_scenario::ctx(scenario_ref));
            let (pos_tokens, neg_tokens) = ctf_registry::add_collateral(&mut registry, additional_coin, test_scenario::ctx(scenario_ref));
            sui::transfer::public_transfer(pos_tokens, USER1);
            sui::transfer::public_transfer(neg_tokens, USER1);
            
            // Verify total supplies increased
            let expected_total = INITIAL_COLLATERAL + ADDITIONAL_COLLATERAL;
            let (pos_supply, neg_supply) = ctf_registry::get_token_supplies(&registry);
            let collateral_balance = ctf_registry::get_collateral_balance(&registry);
            
            assert!(pos_supply == expected_total, 0);
            assert!(neg_supply == expected_total, 1); 
            assert!(collateral_balance == expected_total, 2);
            
            sui::test_utils::destroy(registry);
        };
        
        // Step 5: User1 transfers some positive tokens to User2 (simulating a trade)
        test_scenario::next_tx(scenario_ref, USER1);
        {
            let mut positive_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            let negative_tokens = test_scenario::take_from_sender<coin::Coin<NEGATIVE_TOKEN>>(scenario_ref);
            
            // Split and transfer positive tokens to USER2
            let tokens_to_transfer = coin::split(&mut positive_tokens, TRADE_AMOUNT, test_scenario::ctx(scenario_ref));
            sui::transfer::public_transfer(tokens_to_transfer, USER2);
            
            test_scenario::return_to_sender(scenario_ref, positive_tokens);
            test_scenario::return_to_sender(scenario_ref, negative_tokens);
        };
        
        // Step 6: Verify User2 received the positive tokens
        test_scenario::next_tx(scenario_ref, USER2);
        {
            let received_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            
            assert!(coin::value(&received_tokens) == TRADE_AMOUNT, 3);
            
            test_scenario::return_to_sender(scenario_ref, received_tokens);
        };
        
        // Step 7: Verify tokens were successfully transferred and can be managed
        test_scenario::next_tx(scenario_ref, USER1);
        {
            let positive_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            let negative_tokens = test_scenario::take_from_sender<coin::Coin<NEGATIVE_TOKEN>>(scenario_ref);
            
            // Verify USER1 has the expected amount after the trade
            let expected_pos_amount = ADDITIONAL_COLLATERAL - TRADE_AMOUNT;
            assert!(coin::value(&positive_tokens) == expected_pos_amount, 7);
            assert!(coin::value(&negative_tokens) == ADDITIONAL_COLLATERAL, 8);
            
            test_scenario::return_to_sender(scenario_ref, positive_tokens);
            test_scenario::return_to_sender(scenario_ref, negative_tokens);
        };
        
        // Clean up
        sui::transfer::public_transfer(collateral_treasury, ADMIN);
        test_scenario::end(scenario);
    }
    
    #[test]
    fun test_multiple_users_adding_collateral() {
        let mut scenario = test_scenario::begin(ADMIN);
        let scenario_ref = &mut scenario;
        
        // Setup
        test_scenario::next_tx(scenario_ref, ADMIN);
        let ctx = test_scenario::ctx(scenario_ref);
        let mut collateral_treasury = coin::create_treasury_cap_for_testing<TEST_COIN>(ctx);
        let positive_treasury = coin::create_treasury_cap_for_testing<POSITIVE_TOKEN>(ctx);  
        let negative_treasury = coin::create_treasury_cap_for_testing<NEGATIVE_TOKEN>(ctx);
        let initial_collateral = coin::mint(&mut collateral_treasury, INITIAL_COLLATERAL, ctx);
        
        // Create registry and have both users add collateral in same transaction
        test_scenario::next_tx(scenario_ref, ADMIN);
        {
            let (mut registry, pos_tokens, neg_tokens) = ctf_registry::create_registry<POSITIVE_TOKEN, NEGATIVE_TOKEN, TEST_COIN>(
                positive_treasury,
                negative_treasury, 
                initial_collateral,
                test_scenario::ctx(scenario_ref)
            );
            sui::transfer::public_transfer(pos_tokens, ADMIN);
            sui::transfer::public_transfer(neg_tokens, ADMIN);
            
            // User1 adds collateral
            let coin1 = coin::mint(&mut collateral_treasury, ADDITIONAL_COLLATERAL, test_scenario::ctx(scenario_ref));
            let (pos_tokens1, neg_tokens1) = ctf_registry::add_collateral(&mut registry, coin1, test_scenario::ctx(scenario_ref));
            sui::transfer::public_transfer(pos_tokens1, USER1);
            sui::transfer::public_transfer(neg_tokens1, USER1);
            
            // User2 adds collateral
            let coin2 = coin::mint(&mut collateral_treasury, ADDITIONAL_COLLATERAL, test_scenario::ctx(scenario_ref));
            let (pos_tokens2, neg_tokens2) = ctf_registry::add_collateral(&mut registry, coin2, test_scenario::ctx(scenario_ref));
            sui::transfer::public_transfer(pos_tokens2, USER2);
            sui::transfer::public_transfer(neg_tokens2, USER2);
            
            // Verify final state
            let expected_total = INITIAL_COLLATERAL + (2 * ADDITIONAL_COLLATERAL);
            let (pos_supply, neg_supply) = ctf_registry::get_token_supplies(&registry);
            let collateral_balance = ctf_registry::get_collateral_balance(&registry);
            
            assert!(pos_supply == expected_total, 0);
            assert!(neg_supply == expected_total, 1);
            assert!(collateral_balance == expected_total, 2);
            
            sui::test_utils::destroy(registry);
        };
        
        // Verify both users received their tokens
        test_scenario::next_tx(scenario_ref, USER1);
        {
            let positive_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            let negative_tokens = test_scenario::take_from_sender<coin::Coin<NEGATIVE_TOKEN>>(scenario_ref);
            
            assert!(coin::value(&positive_tokens) == ADDITIONAL_COLLATERAL, 3);
            assert!(coin::value(&negative_tokens) == ADDITIONAL_COLLATERAL, 4);
            
            test_scenario::return_to_sender(scenario_ref, positive_tokens);
            test_scenario::return_to_sender(scenario_ref, negative_tokens);
        };
        
        test_scenario::next_tx(scenario_ref, USER2);
        {
            let positive_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            let negative_tokens = test_scenario::take_from_sender<coin::Coin<NEGATIVE_TOKEN>>(scenario_ref);
            
            assert!(coin::value(&positive_tokens) == ADDITIONAL_COLLATERAL, 5);
            assert!(coin::value(&negative_tokens) == ADDITIONAL_COLLATERAL, 6);
            
            test_scenario::return_to_sender(scenario_ref, positive_tokens);
            test_scenario::return_to_sender(scenario_ref, negative_tokens);
        };
        
        // Clean up
        sui::transfer::public_transfer(collateral_treasury, ADMIN);
        test_scenario::end(scenario);
    }
    
    #[test]
    #[expected_failure(
        abort_code = ctf_errors::E_INVALID_AMOUNT,
        location   = ctf::ctf_registry
    )]
    fun test_unequal_token_redemption_fails() {
        let mut scenario = test_scenario::begin(ADMIN);
        let scenario_ref = &mut scenario;
        
        // Setup
        test_scenario::next_tx(scenario_ref, ADMIN);
        let ctx = test_scenario::ctx(scenario_ref);
        let mut collateral_treasury = coin::create_treasury_cap_for_testing<TEST_COIN>(ctx);
        let positive_treasury = coin::create_treasury_cap_for_testing<POSITIVE_TOKEN>(ctx);  
        let negative_treasury = coin::create_treasury_cap_for_testing<NEGATIVE_TOKEN>(ctx);
        let initial_collateral = collateral_treasury.mint( INITIAL_COLLATERAL, ctx);
        
        // Create registry and try to redeem unequal amounts in same transaction (should fail)
        test_scenario::next_tx(scenario_ref, ADMIN);
        {
            let (mut registry, mut positive_tokens, mut negative_tokens) = ctf_registry::create_registry<POSITIVE_TOKEN, NEGATIVE_TOKEN, TEST_COIN>(
                positive_treasury,
                negative_treasury, 
                initial_collateral,
                test_scenario::ctx(scenario_ref)
            );
            
            // Try to redeem different amounts (this should fail)
            let pos_to_redeem = coin::split(&mut positive_tokens, 100_000_000, test_scenario::ctx(scenario_ref));
            let neg_to_redeem = coin::split(&mut negative_tokens, 200_000_000, test_scenario::ctx(scenario_ref)); // Different amount!
            
            let collateral_coin = ctf_registry::redeem_tokens(&mut registry, pos_to_redeem, neg_to_redeem, test_scenario::ctx(scenario_ref));
            sui::transfer::public_transfer(collateral_coin, ADMIN);
            
            // Clean up remaining tokens (though this should not be reached due to expected failure)
            sui::test_utils::destroy(positive_tokens);
            sui::test_utils::destroy(negative_tokens);
            sui::test_utils::destroy(registry);
        };
        
        sui::transfer::public_transfer(collateral_treasury, ADMIN);
        test_scenario::end(scenario);
    }
    
    #[test]
    #[expected_failure(
    abort_code = ctf_errors::E_INVALID_AMOUNT,
    location   = ctf::ctf_registry
)]
    fun test_zero_collateral_fails() {
        let mut scenario = test_scenario::begin(ADMIN);
        let scenario_ref = &mut scenario;
        
        // Setup
        test_scenario::next_tx(scenario_ref, ADMIN);
        let ctx = test_scenario::ctx(scenario_ref);
        let mut collateral_treasury = coin::create_treasury_cap_for_testing<TEST_COIN>(ctx);
        let positive_treasury = coin::create_treasury_cap_for_testing<POSITIVE_TOKEN>(ctx);  
        let negative_treasury = coin::create_treasury_cap_for_testing<NEGATIVE_TOKEN>(ctx);
        let initial_collateral = collateral_treasury.mint(INITIAL_COLLATERAL, ctx);
        
        // Create registry and try to add zero collateral in same transaction (should fail)
        test_scenario::next_tx(scenario_ref, USER1);
        {
            let (mut registry, positive_tokens, negative_tokens) = ctf_registry::create_registry<POSITIVE_TOKEN, NEGATIVE_TOKEN, TEST_COIN>(
                positive_treasury,
                negative_treasury, 
                initial_collateral,
                test_scenario::ctx(scenario_ref)
            );
            sui::transfer::public_transfer(positive_tokens, ADMIN);
            sui::transfer::public_transfer(negative_tokens, ADMIN);
            
            // Try to add zero collateral (should fail)
            let zero_coin = coin::mint(&mut collateral_treasury, 0, test_scenario::ctx(scenario_ref)); // Zero amount!
            
            let (positive_tokens, negative_tokens) = ctf_registry::add_collateral(&mut registry, zero_coin, test_scenario::ctx(scenario_ref));
            sui::transfer::public_transfer(positive_tokens, USER1);
            sui::transfer::public_transfer(negative_tokens, USER1);
            
            // Clean up (though this should not be reached due to expected failure)
            sui::test_utils::destroy(registry);
        };
        
        // Clean up
        sui::transfer::public_transfer(collateral_treasury, ADMIN);
        test_scenario::end(scenario);
    }
    
    #[test]
    fun test_token_transfers_between_users() {
        let mut scenario = test_scenario::begin(ADMIN);
        let scenario_ref = &mut scenario;
        
        // Setup registry
        test_scenario::next_tx(scenario_ref, ADMIN);
        let ctx = test_scenario::ctx(scenario_ref);
        let mut collateral_treasury = coin::create_treasury_cap_for_testing<TEST_COIN>(ctx);
        let positive_treasury = coin::create_treasury_cap_for_testing<POSITIVE_TOKEN>(ctx);  
        let negative_treasury = coin::create_treasury_cap_for_testing<NEGATIVE_TOKEN>(ctx);
        let initial_collateral = collateral_treasury.mint(INITIAL_COLLATERAL, ctx);
        
        // Create registry and transfer tokens to USER1 in same transaction
        let transfer_amount = INITIAL_COLLATERAL / 2;
        test_scenario::next_tx(scenario_ref, ADMIN);
        {
            let (registry, mut positive_tokens, mut negative_tokens) = ctf_registry::create_registry<POSITIVE_TOKEN, NEGATIVE_TOKEN, TEST_COIN>(
                positive_treasury,
                negative_treasury, 
                initial_collateral,
                test_scenario::ctx(scenario_ref)
            );
            
            // Admin transfers half of each token type to USER1
            let pos_to_transfer = coin::split(&mut positive_tokens, transfer_amount, test_scenario::ctx(scenario_ref));
            let neg_to_transfer = coin::split(&mut negative_tokens, transfer_amount, test_scenario::ctx(scenario_ref));
            
            sui::transfer::public_transfer(pos_to_transfer, USER1);
            sui::transfer::public_transfer(neg_to_transfer, USER1);
            sui::transfer::public_transfer(positive_tokens, ADMIN);
            sui::transfer::public_transfer(negative_tokens, ADMIN);
            
            sui::test_utils::destroy(registry);
        };
        
        // Verify USER1 received the tokens
        test_scenario::next_tx(scenario_ref, USER1);
        {
            let positive_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            let negative_tokens = test_scenario::take_from_sender<coin::Coin<NEGATIVE_TOKEN>>(scenario_ref);
            
            assert!(coin::value(&positive_tokens) == transfer_amount, 0);
            assert!(coin::value(&negative_tokens) == transfer_amount, 1);
            
            test_scenario::return_to_sender(scenario_ref, positive_tokens);
            test_scenario::return_to_sender(scenario_ref, negative_tokens);
        };
        
        // Verify ADMIN still has remaining tokens
        test_scenario::next_tx(scenario_ref, ADMIN);
        {
            let positive_tokens = test_scenario::take_from_sender<coin::Coin<POSITIVE_TOKEN>>(scenario_ref);
            let negative_tokens = test_scenario::take_from_sender<coin::Coin<NEGATIVE_TOKEN>>(scenario_ref);
            
            assert!(coin::value(&positive_tokens) == transfer_amount, 2);
            assert!(coin::value(&negative_tokens) == transfer_amount, 3);
            
            test_scenario::return_to_sender(scenario_ref, positive_tokens);
            test_scenario::return_to_sender(scenario_ref, negative_tokens);
        };
        
        sui::transfer::public_transfer(collateral_treasury, ADMIN);
        test_scenario::end(scenario);
    }
}