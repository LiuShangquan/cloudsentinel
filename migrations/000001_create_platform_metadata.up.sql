CREATE TABLE platform_metadata (
    metadata_key VARCHAR(100) NOT NULL,
    metadata_value VARCHAR(255) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (metadata_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO platform_metadata (metadata_key, metadata_value)
VALUES ('schema_owner', 'golang-migrate');

