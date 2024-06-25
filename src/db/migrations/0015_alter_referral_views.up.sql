

-- === VIEWS === --
-- Drop the existing views
DROP VIEW IF EXISTS referral_last_payment, referral_aggr_fees_per_trader;

-- last payment view referral_last_payment
CREATE OR REPLACE VIEW referral_last_payment AS
SELECT 
    LOWER(broker_addr) as broker_addr,
    pool_id,
	LOWER(trader_addr) as trader_addr,
	BOOL_AND(tx_confirmed) as tx_confirmed, -- false if there are payments that haven't been confirmed yet
	MAX(block_ts) as last_payment_ts
FROM referral_payment GROUP BY LOWER(trader_addr), LOWER(broker_addr), pool_id;

-- referral_aggr_fees_per_trader
CREATE OR REPLACE VIEW referral_aggr_fees_per_trader AS
SELECT 
    rs_broker_id.broker_id,
    th.perpetual_id/100000 as pool_id,
    th.trader_addr,
    th.broker_addr,
    COALESCE(codeusg.code,'DEFAULT') as code,
    CAST(sum(th.fee) as DECIMAL(40,0)) as fee_sum_cc,
    CAST(SUM((th.broker_fee_tbps * ABS(th.quantity_cc) - 50000) / 100000) as DECIMAL(40,0)) as broker_fee_cc, -- ABDK 64x64 format; rounding down
    min(th.trade_timestamp) as first_trade_considered_ts,
    max(th.trade_timestamp) as last_trade_considered_ts,
    lp.last_payment_ts,
    coalesce(lp.last_payment_ts, current_date::timestamp - (rs.value || ' days')::interval) as pay_period_start_ts
FROM trades_history th
JOIN referral_settings rs_broker_id -- join settings table to get broker-id for given broker address
	 ON rs_broker_id.property = 'broker_addr'
	 AND rs_broker_id.value = th.broker_addr
JOIN referral_settings rs 
	ON rs.property = 'payment_max_lookback_days'
	and rs.broker_id = rs_broker_id.broker_id
LEFT JOIN referral_code_usage codeusg
    ON LOWER(th.trader_addr) = LOWER(codeusg.trader_addr)
    AND rs_broker_id.broker_id = LOWER(codeusg.broker_id)
    AND codeusg.valid_to > NOW()
LEFT JOIN referral_last_payment lp
    ON LOWER(lp.trader_addr)=LOWER(th.trader_addr)
	AND lp.pool_id=th.perpetual_id/100000
    AND LOWER(th.broker_addr)=LOWER(lp.broker_addr)
WHERE (
		(lp.last_payment_ts IS null and current_date::timestamp - (rs.value || ' days')::interval < th.trade_timestamp) 
		OR GREATEST(current_date::timestamp - (rs.value || ' days')::interval, lp.last_payment_ts) < th.trade_timestamp
	)
GROUP BY pool_id, rs_broker_id.broker_id, th.trader_addr, th.broker_addr, th.perpetual_id/100000, rs.value,codeusg.code,lp.last_payment_ts;
