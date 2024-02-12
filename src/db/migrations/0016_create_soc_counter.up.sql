-- CreateTable
CREATE TABLE if not exists "soc_counter" (
    "id" VARCHAR(42) NOT NULL,
    "id_interacted" VARCHAR(42) NOT NULL,
    "count" INTEGER,
    "created_on" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "soc_counter_pkey" PRIMARY KEY ("id", "id_interacted")
);

-- CreateIndex
CREATE INDEX  IF NOT EXISTS "id_idx" ON "soc_counter" USING HASH ("id");
CREATE INDEX  IF NOT EXISTS "id_interacted_idx" ON "soc_counter" USING HASH ("id_interacted");