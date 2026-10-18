-- Match the bounded immutable Chat correction contract. Existing directives
-- remain intact; the previous 4,000-character check rejected detailed reviews.
ALTER TABLE issue_owner_directive
    DROP CONSTRAINT issue_owner_directive_content_check,
    ADD CONSTRAINT issue_owner_directive_content_check
        CHECK (octet_length(btrim(content)) BETWEEN 1 AND 65536);
