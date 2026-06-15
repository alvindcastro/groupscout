DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'raw_projects'
          AND column_name = 'raw_data'
          AND udt_name = 'jsonb'
    ) THEN
        ALTER TABLE raw_projects
            ALTER COLUMN raw_data TYPE BYTEA
            USING CASE
                WHEN jsonb_typeof(raw_data) = 'string' THEN convert_to(raw_data #>> '{}', 'UTF8')
                ELSE convert_to(raw_data::text, 'UTF8')
            END;
    ELSIF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'raw_projects'
          AND column_name = 'raw_data'
          AND udt_name <> 'bytea'
    ) THEN
        RAISE EXCEPTION 'unsupported raw_projects.raw_data type for bytea migration';
    END IF;
END $$;
