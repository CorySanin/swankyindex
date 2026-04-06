CREATE TABLE downloads (
    Path VARCHAR(256),
    Filename VARCHAR(128),
    AccessDomain VARCHAR(64),
    UserAgent VARCHAR(64),
    Timestamp DATETIME
);

CREATE INDEX IF NOT EXISTS download_index ON downloads (Path, Filename)
