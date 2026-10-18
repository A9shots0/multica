DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM issue_external_work_receipt) THEN
        RAISE EXCEPTION 'external work receipts contain durable evidence; refusing destructive rollback';
    END IF;
END $$;

DROP TABLE IF EXISTS issue_external_work_receipt;
