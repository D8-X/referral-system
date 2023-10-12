
CREATE OR REPLACE VIEW referral_last_payment AS
SELECT 
    pool_id,
	LOWER(trader_addr) as trader_addr,
	BOOL_AND(tx_confirmed) as tx_confirmed, -- false if there are payments that haven't been confirmed yet
	MAX(block_ts) as last_payment_ts 
FROM referral_payment GROUP BY LOWER(trader_addr), pool_id;