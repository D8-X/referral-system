-- AddLevelToReferralPayment

ALTER TABLE "referral_payment" DROP CONSTRAINT referral_payment_pkey;

ALTER TABLE "referral_payment"
ADD COLUMN "level" INTEGER NOT NULL DEFAULT 0,
ADD CONSTRAINT "referral_payment_pkey" PRIMARY KEY ("trader_addr", "payee_addr", "pool_id", "code", "batch_ts", "level");

-- AddLevelToFailedReferralPayment

ALTER TABLE "referral_failed_payment" DROP CONSTRAINT referral_failed_payment_pkey;

ALTER TABLE "referral_failed_payment"
ADD COLUMN "level" INTEGER NOT NULL DEFAULT 0,
ADD CONSTRAINT "referral_failed_payment_pkey" PRIMARY KEY ("trader_addr", "payee_addr", "pool_id", "code", "batch_ts", "level");
