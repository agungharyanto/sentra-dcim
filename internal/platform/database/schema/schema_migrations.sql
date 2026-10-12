-- Table managed by golang-migrate. Declared here only so sqlc can type-check queries against it.
CREATE TABLE schema_migrations (
    version bigint  NOT NULL PRIMARY KEY,
    dirty   boolean NOT NULL
);
