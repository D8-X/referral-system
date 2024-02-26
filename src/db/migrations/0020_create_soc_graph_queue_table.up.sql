-- CreateTable
CREATE TABLE if not exists "soc_graph_queue" (
    "id" VARCHAR(42) NOT NULL,
    "created_on" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "soc_graph_queue_pkey" PRIMARY KEY ("id")
);