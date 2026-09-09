-- R4: rack templates + PDU domain

CREATE TABLE IF NOT EXISTS rack_templates (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    code varchar(50) NOT NULL,
    name varchar(150) NOT NULL,
    description text,
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    is_system boolean NOT NULL DEFAULT false,
    current_revision bigint NOT NULL DEFAULT 1,
    remarks text
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rack_templates_code_active ON rack_templates (code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_rack_templates_deleted_at ON rack_templates (deleted_at);

CREATE TABLE IF NOT EXISTS rack_template_versions (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    template_id uuid NOT NULL REFERENCES rack_templates(id),
    revision bigint NOT NULL,
    type varchar(50) NOT NULL DEFAULT 'STANDARD',
    manufacturer varchar(100),
    model_number varchar(100),
    u_height bigint NOT NULL DEFAULT 42,
    width_mm bigint NOT NULL DEFAULT 600,
    depth_mm bigint NOT NULL DEFAULT 1200,
    height_mm bigint NOT NULL DEFAULT 2000,
    load_capacity_kg numeric,
    dual_power boolean NOT NULL DEFAULT false,
    input_circuits bigint NOT NULL DEFAULT 0,
    rated_voltage numeric,
    rated_current numeric,
    rated_power_kw numeric,
    peak_power_kw numeric,
    pdu_count bigint NOT NULL DEFAULT 0,
    change_note varchar(500),
    created_by uuid
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rack_template_revision_active ON rack_template_versions (template_id, revision) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_rack_template_versions_deleted_at ON rack_template_versions (deleted_at);

CREATE TABLE IF NOT EXISTS pdus (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    rack_id uuid NOT NULL REFERENCES racks(id),
    code varchar(80) NOT NULL,
    name varchar(150) NOT NULL,
    manufacturer varchar(100),
    model_number varchar(100),
    serial_number varchar(120),
    input_voltage numeric,
    rated_power_w numeric,
    rated_current_a numeric,
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    remarks text
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pdus_code_active ON pdus (code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pdus_rack_id ON pdus (rack_id);
CREATE INDEX IF NOT EXISTS idx_pdus_deleted_at ON pdus (deleted_at);

CREATE TABLE IF NOT EXISTS pdu_sockets (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    pdu_id uuid NOT NULL REFERENCES pdus(id),
    socket_no bigint NOT NULL,
    standard varchar(10) NOT NULL,
    amperage_a bigint NOT NULL,
    label varchar(100),
    status varchar(20) NOT NULL DEFAULT 'AVAILABLE'
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pdu_sockets_number_active ON pdu_sockets (pdu_id, socket_no) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pdu_sockets_pdu_id ON pdu_sockets (pdu_id);
CREATE INDEX IF NOT EXISTS idx_pdu_sockets_deleted_at ON pdu_sockets (deleted_at);

CREATE TABLE IF NOT EXISTS pdu_connections (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    socket_id uuid NOT NULL REFERENCES pdu_sockets(id),
    device_id uuid NOT NULL REFERENCES devices(id),
    power_w numeric,
    circuit varchar(100),
    redundancy_role varchar(20) NOT NULL DEFAULT 'PRIMARY',
    connected_at timestamptz NOT NULL,
    connected_by uuid
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pdu_connections_socket_active ON pdu_connections (socket_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_pdu_connections_device_role_active ON pdu_connections (device_id, redundancy_role) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pdu_connections_deleted_at ON pdu_connections (deleted_at);
CREATE INDEX IF NOT EXISTS idx_pdu_connections_device_id ON pdu_connections (device_id);
