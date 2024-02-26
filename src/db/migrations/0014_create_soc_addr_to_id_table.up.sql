-- CreateTable
CREATE TABLE if not exists "soc_addr_to_id" (
    "addr" VARCHAR(42) NOT NULL,
    "id" VARCHAR(42) NOT NULL,
    "created_on" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "soc_addr_to_id_pk" PRIMARY KEY ("addr")
);

-- CreateIndex
CREATE INDEX  IF NOT EXISTS "addr_idx" ON "soc_addr_to_id" USING HASH ("addr");