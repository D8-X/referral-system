
-- Drop the existing view
DROP VIEW IF EXISTS referral_aggr_fees_per_trader;

CREATE OR REPLACE VIEW referral_aggr_fees_per_trader AS
SELECT 
    th.perpetual_id/100000 as pool_id,
    th.trader_addr,
    th.broker_addr,
    COALESCE(codeusg.code,'DEFAULT') as code,
    sum(th.fee) as fee_sum_cc,
    SUM((th.broker_fee_tbps * ABS(th.quantity_cc) - 50000) / 100000) as broker_fee_cc, -- ABDK 64x64 format; rounding down
    min(th.trade_timestamp) as first_trade_considered_ts,
    max(th.trade_timestamp) as last_trade_considered_ts,
    lp.last_payment_ts,
    coalesce(lp.last_payment_ts, current_date::timestamp - (rs.value || ' days')::interval) as pay_period_start_ts
FROM trades_history th
join referral_settings rs on rs.property = 'payment_max_lookback_days'
join referral_settings rs2 on rs2.property = 'broker_addr'
LEFT JOIN referral_last_payment lp
    ON LOWER(lp.trader_addr)=LOWER(th.trader_addr) and lp.pool_id=th.perpetual_id/100000
LEFT JOIN referral_code_usage codeusg
    ON LOWER(th.trader_addr) = LOWER(codeusg.trader_addr)
    AND LOWER(th.broker_addr) = LOWER(rs2.value)
    AND codeusg.valid_to > NOW()
WHERE ((lp.last_payment_ts IS null and current_date::timestamp - (rs.value || ' days')::interval < th.trade_timestamp) OR lp.last_payment_ts<th.trade_timestamp)
    AND (lp.pool_id IS NULL OR lp.pool_id = th.perpetual_id/100000)
    AND (lp.tx_confirmed IS NULL OR lp.tx_confirmed=true)
GROUP BY pool_id, rs2.value, th.trader_addr, th.broker_addr, lp.last_payment_ts, codeusg.code, th.perpetual_id/100000, rs.value
ORDER BY th.trader_addr;


