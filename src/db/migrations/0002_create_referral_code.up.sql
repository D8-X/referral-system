
-- CreateTable
CREATE TABLE if not exists "referral_code" (
    "code" VARCHAR(200) NOT NULL,
    "referrer_addr" VARCHAR(42) NOT NULL,
    "created_on" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "expiry" TIMESTAMPTZ NOT NULL DEFAULT '2042-04-24 04:42:42 +02:00',
    "trader_rebate_perc" DECIMAL(5,2) NOT NULL DEFAULT 0,
    CONSTRAINT "referral_code_pkey" PRIMARY KEY ("code")
);

-- CreateIndex
CREATE INDEX  IF NOT EXISTS "referral_code_referrer_addr_idx" ON "referral_code" USING HASH ("referrer_addr");
