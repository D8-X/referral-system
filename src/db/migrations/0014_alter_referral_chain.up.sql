
-- referral chain
ALTER TABLE "referral_chain"
ADD COLUMN "broker_id" VARCHAR(42) NOT NULL DEFAULT '';

ALTER TABLE "referral_chain"
DROP CONSTRAINT "child";

ALTER TABLE "referral_chain"
ADD CONSTRAINT "referral_chain_pk" PRIMARY KEY ("broker_id", "child");


-- referral code
ALTER TABLE "referral_code"
ADD COLUMN "broker_id" VARCHAR(42) NOT NULL DEFAULT '';

ALTER TABLE "referral_code"
DROP CONSTRAINT "referral_code_pkey";

ALTER TABLE "referral_code"
ADD CONSTRAINT "referral_code_pkey" PRIMARY KEY ("broker_id", "code");


-- referral code usage
ALTER TABLE "referral_code_usage"
ADD COLUMN "broker_id" VARCHAR(42) NOT NULL DEFAULT '';

ALTER TABLE "referral_code_usage"
DROP CONSTRAINT "referral_code_usage_pkey";

ALTER TABLE "referral_code_usage"
ADD CONSTRAINT "referral_code_usage_pkey" PRIMARY KEY ("broker_id","trader_addr","valid_from");

-- settings table
ALTER TABLE "referral_settings"
ADD COLUMN "broker_id" VARCHAR(42) NOT NULL DEFAULT '';

ALTER TABLE "referral_settings"
DROP CONSTRAINT "referral_settings_pkey";

ALTER TABLE "referral_settings"
ADD CONSTRAINT "referral_settings_pkey" PRIMARY KEY ("broker_id","property");

-- payment table
ALTER TABLE "referral_payment"
ADD COLUMN "broker_id" VARCHAR(42) NOT NULL DEFAULT '';

ALTER TABLE "referral_payment"
DROP CONSTRAINT "referral_payment_pkey";

ALTER TABLE "referral_payment"
ADD CONSTRAINT "referral_payment_pkey" PRIMARY KEY ("broker_id", "trader_addr", "payee_addr", "pool_id", "code", "batch_ts", "level");

-- referral_setting_cut
ALTER TABLE "referral_setting_cut"
ADD COLUMN "broker_id" VARCHAR(42) NOT NULL DEFAULT '';

ALTER TABLE "referral_setting_cut"
DROP CONSTRAINT "referral_setting_cut_pkey";

ALTER TABLE "referral_setting_cut"
ADD CONSTRAINT "referral_setting_cut_pkey" PRIMARY KEY ("broker_id", "cut_perc", "token_addr");

-- referral_failed_payment
ALTER TABLE "referral_failed_payment"
ADD COLUMN "broker_id" VARCHAR(42) NOT NULL DEFAULT '';

ALTER TABLE "referral_failed_payment"
DROP CONSTRAINT "referral_failed_payment_pkey";

ALTER TABLE "referral_failed_payment"
ADD CONSTRAINT "referral_failed_payment_pkey" PRIMARY KEY ("broker_id", "trader_addr", "payee_addr", "pool_id", "code", "batch_ts");

-- -- token holdings table: no change needed
