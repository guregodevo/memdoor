-- Store file binary content in SQLite for Raft replication in cluster mode
CREATE TABLE IF NOT EXISTS file_data (
    id TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    FOREIGN KEY (id) REFERENCES files(id) ON DELETE CASCADE
);
