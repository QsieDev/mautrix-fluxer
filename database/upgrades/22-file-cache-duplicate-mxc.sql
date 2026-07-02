-- v22 (compatible with v19+): Allow non-unique mxc URIs in file cache
CREATE TABLE new_fluxer_file (
    url       TEXT,
    encrypted BOOLEAN,
    mxc       TEXT NOT NULL,

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

INSERT INTO new_fluxer_file (url, encrypted, mxc, id, emoji_name, size, width, height, mime_type, decryption_info, timestamp)
SELECT url, encrypted, mxc, id, emoji_name, size, width, height, mime_type, decryption_info, timestamp FROM fluxer_file;

DROP TABLE fluxer_file;
ALTER TABLE new_fluxer_file RENAME TO fluxer_file;

CREATE INDEX fluxer_file_mxc_idx ON fluxer_file (mxc);
