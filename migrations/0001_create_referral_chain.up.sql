-- CreateTable
CREATE TABLE if not exists "referral_chain" (
    "parent" VARCHAR(42) NOT NULL,
    "child" VARCHAR(42) NOT NULL,
    "pass_on" DECIMAL(5,2) NOT NULL DEFAULT 0,
    "created_on" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "child" PRIMARY KEY ("child")
);

-- CreateIndex
CREATE INDEX  IF NOT EXISTS "referral_chain_parent_idx" ON "referral_chain" USING HASH ("parent");