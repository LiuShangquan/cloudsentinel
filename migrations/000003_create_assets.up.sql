CREATE TABLE hosts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,
    address VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_hosts_name (name),
    KEY idx_hosts_status_id (status, id),
    CONSTRAINT fk_hosts_created_by FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT fk_hosts_updated_by FOREIGN KEY (updated_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE services (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    host_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(100) NOT NULL,
    type ENUM('http', 'tcp') NOT NULL,
    target VARCHAR(2048) NOT NULL,
    description TEXT NOT NULL,
    status ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
    created_by BIGINT UNSIGNED NOT NULL,
    updated_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_services_host_name (host_id, name),
    KEY idx_services_status_id (status, id),
    CONSTRAINT fk_services_host FOREIGN KEY (host_id) REFERENCES hosts(id),
    CONSTRAINT fk_services_created_by FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT fk_services_updated_by FOREIGN KEY (updated_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

