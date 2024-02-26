CREATE OR REPLACE VIEW soc_top3_interactions AS
SELECT ati.id, 
    ati.addr, 
    top_interactions.id_interacted, 
    top_interactions.addr_interacted, 
    COALESCE(top_interactions.count,0) AS count,
    COALESCE(top_interactions.rnk, 0) AS rnk
FROM soc_addr_to_id ati 
LEFT JOIN (
    SELECT id, id_interacted, addr_interacted, count, row_num as rnk
    FROM (
        SELECT 
            c0.id,
            c0.id_interacted,
            ati0.addr as addr_interacted,
            count,
            ROW_NUMBER() OVER (PARTITION BY c0.id ORDER BY count DESC) AS row_num
        FROM soc_counter as c0
        JOIN soc_addr_to_id as ati0 ON c0.id_interacted = ati0.id
    ) AS ranked
    WHERE row_num <= 3
) AS top_interactions
ON ati.id = top_interactions.id;