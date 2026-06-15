DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'raw_projects'
          AND column_name = 'raw_data'
          AND data_type = 'bytea'
    ) THEN
        RAISE EXCEPTION 'raw_projects.raw_data bytea migration is not safely reversible';
    END IF;
END $$;
