

ALTER VIEW referral_aggr_fees_per_trader
    DROP COLUMN broker_fee_cc;

ALTER VIEW referral_aggr_fees_per_trader
    ADD COLUMN broker_fee_cc NUMERIC(40);

-- round down the broker fee
UPDATE referral_aggr_fees_per_trader
SET broker_fee_cc = SUM((th.broker_fee_tbps * ABS(th.quantity_cc) - 50000) / 100000);

