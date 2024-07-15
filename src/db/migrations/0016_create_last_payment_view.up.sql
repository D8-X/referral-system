-- Drop the existing view
DROP VIEW IF EXISTS referral_last_payment;
-- tx_confirmed only considered since now - paymentlookbackdays

CREATE OR REPLACE VIEW referral_last_payment AS
SELECT lower(referral_payment.broker_addr::text) AS broker_addr,
    referral_payment.pool_id,
    lower(referral_payment.trader_addr::text) AS trader_addr,
    bool_and(referral_payment.tx_confirmed) AS tx_confirmed,
    max(referral_payment.block_ts) AS last_payment_ts, 
    rs1.broker_id
FROM referral_payment
JOIN referral_settings rs1 
	ON rs1.property = 'broker_addr'
	AND lower(rs1.value) = lower(referral_payment.broker_addr)
JOIN referral_settings rs_max_lookback
	ON rs_max_lookback.property = 'payment_max_lookback_days'
	AND rs_max_lookback.broker_id = rs1.broker_id
WHERE referral_payment.block_ts > (current_date::timestamp - (rs_max_lookback.value || ' days')::interval)
GROUP BY rs_max_lookback.value, 
	rs1.broker_id, 
	(lower(referral_payment.trader_addr::text)), 
	(lower(referral_payment.broker_addr::text)), 
	referral_payment.pool_id;