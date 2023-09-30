

-- CreateTable
  -- no constraint for referral code because we could collect the data from onchain
  -- and we could encounter an unknown referral code in this case
CREATE TABLE if not exists "referral_payment" (
    "trader_addr" VARCHAR(42) NOT NULL,
    "payee_addr" VARCHAR(42) NOT NULL,
    "code" VARCHAR(200) NOT NULL,
    "pool_id" INTEGER NOT NULL,
    "batch_ts" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- payment in token's number format, single transaction
    "paid_amount_cc" DECIMAL(40,0) NOT NULL,
    "tx_hash" TEXT NOT NULL,
    "block_nr" DECIMAL(40,0),
    "tx_confirmed" BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT "referral_payment_pkey" PRIMARY KEY ("trader_addr", "payee_addr", "pool_id","batch_ts")
);

-- CreateIndex
CREATE INDEX  IF NOT EXISTS "referral_payment_batch_ts_idx" ON "referral_payment"("batch_ts");

-- CreateIndex
CREATE INDEX  IF NOT EXISTS "referral_payment_pool_id_idx" ON "referral_payment"("pool_id");

-- CreateIndex
CREATE INDEX  IF NOT EXISTS "referral_payment_trader_addr_idx" ON "referral_payment"("trader_addr");

