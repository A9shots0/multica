DELETE FROM inbox_item
WHERE details->>'recovery_owner_wait' = 'true';
