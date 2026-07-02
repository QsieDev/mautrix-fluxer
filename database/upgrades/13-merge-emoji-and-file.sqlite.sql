-- v13: Merge tables used for cached custom emojis and attachments
CREATE TABLE new_fluxer_file (
    url       TEXT,
    encrypted BOOLEAN,
    mxc       TEXT NOT NULL UNIQUE,

    id         TEXT,
    emoji_name TEXT,

    size            BIGINT NOT NULL,
    width           INTEGER,
    height          INTEGER,
    mime_type       TEXT NOT NULL,
    decryption_info jsonb,
    timestamp       BIGINT NOT NULL,

    PRIMARY KEY (url, encrypted)
);

INSERT INTO new_fluxer_file (url, encrypted, id, mxc, size, width, height, mime_type, decryption_info, timestamp)
SELECT url, encrypted, id, mxc, size, width, height, mime_type, decryption_info, timestamp FROM fluxer_file;

DROP TABLE fluxer_file;
ALTER TABLE new_fluxer_file RENAME TO fluxer_file;
