-- Which AI provider and model scored each lead, so scores from different
-- providers can be told apart. Empty for leads the rule pre-scorer skipped
-- and for leads created before the provider chain.
ALTER TABLE leads ADD COLUMN IF NOT EXISTS ai_provider TEXT NOT NULL DEFAULT '';
ALTER TABLE leads ADD COLUMN IF NOT EXISTS ai_model TEXT NOT NULL DEFAULT '';
