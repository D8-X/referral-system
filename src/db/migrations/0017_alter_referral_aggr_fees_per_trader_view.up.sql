
-- Drop the existing view
DROP VIEW IF EXISTS referral_aggr_fees_per_trader;

CREATE OR REPLACE VIEW referral_aggr_fees_per_trader AS
SELECT th.perpetual_id / 100000 AS pool_id,
    th.trader_addr,
    th.broker_addr,
    COALESCE(codeusg.code, 'DEFAULT'::character varying) AS code,
    sum(th.fee)::numeric(40,0) AS fee_sum_cc,
    sum((th.broker_fee_tbps::numeric * abs(th.quantity_cc) - 50000::numeric) / 100000::numeric)::numeric(40,0) AS broker_fee_cc,
    min(th.trade_timestamp) AS first_trade_considered_ts,
    max(th.trade_timestamp) AS last_trade_considered_ts,
    lp.last_payment_ts,
    COALESCE(lp.last_payment_ts, (CURRENT_DATE::timestamp without time zone - ((rs.value::text || ' days'::text)::interval))::timestamp with time zone) AS pay_period_start_ts
 FROM trades_history th
     JOIN referral_settings rs2 ON rs2.property::text = 'broker_addr'::text 
        AND lower(rs2.value)=lower(th.broker_addr)
     JOIN referral_settings rs ON rs.property::text = 'payment_max_lookback_days'::text 
        AND rs.broker_id = rs2.broker_id
     LEFT JOIN referral_last_payment lp ON lower(lp.trader_addr) = lower(th.trader_addr::text) AND lp.pool_id = (th.perpetual_id / 100000) 
        AND lower(lp.trader_addr) = lower(th.trader_addr::text) 
        AND lower(lp.broker_addr) = lower(th.broker_addr::text)
     LEFT JOIN referral_code_usage codeusg ON lower(th.trader_addr::text) = lower(codeusg.trader_addr::text) AND lower(th.broker_addr::text) = lower(rs2.value::text) AND codeusg.valid_to > now()
  WHERE (lp.last_payment_ts IS NULL AND (CURRENT_DATE::timestamp without time zone - ((rs.value::text || ' days'::text)::interval)) < th.trade_timestamp 
  	OR lp.last_payment_ts < th.trade_timestamp) 
  	AND (lp.pool_id IS NULL OR lp.pool_id = (th.perpetual_id / 100000)) 
  	AND (lp.tx_confirmed IS NULL OR lp.tx_confirmed = true)
  GROUP BY lp.pool_id, rs2.value, th.trader_addr, th.broker_addr, lp.last_payment_ts, codeusg.code, (th.perpetual_id / 100000), rs.value
  ORDER BY th.trader_addr;

