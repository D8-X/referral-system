CREATE OR REPLACE VIEW soc_glbl_rank AS
SELECT 
    c.id,
    coalesce ((SELECT sum(c2.count) FROM soc_counter c2 WHERE c2.id_interacted = c.id), 0) AS interaction_count
FROM 
    soc_counter c
GROUP BY 
    c.id
order by interaction_count desc;