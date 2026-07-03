ALTER TABLE campaign_documents
    ADD COLUMN IF NOT EXISTS tsv tsvector
        GENERATED ALWAYS AS (to_tsvector('portuguese', content)) STORED;

CREATE INDEX IF NOT EXISTS campaign_documents_tsv_idx
    ON campaign_documents USING GIN(tsv);
