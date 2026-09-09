-- R1/R2 core schema: auth + resource hierarchy + audit
-- Source: frozen old-system DDL (db/schema.sql), cleaned for versioned migration.

CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;

CREATE TABLE IF NOT EXISTS roles (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    code varchar(50) NOT NULL,
    name varchar(100) NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_code ON roles (code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    username varchar(100) NOT NULL,
    display_name varchar(100) NOT NULL,
    email varchar(255),
    password_hash varchar(255) NOT NULL,
    auth_source varchar(20) NOT NULL DEFAULT 'local',
    enabled boolean NOT NULL DEFAULT true,
    failed_logins bigint NOT NULL DEFAULT 0,
    last_login_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS user_roles (
    user_id uuid NOT NULL,
    role_id uuid NOT NULL,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS data_centers (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    code varchar(50) NOT NULL,
    name varchar(150) NOT NULL,
    address varchar(500),
    longitude numeric,
    latitude numeric,
    status varchar(30) NOT NULL DEFAULT 'OPERATING',
    manager varchar(100),
    contact varchar(100),
    service_provider varchar(150),
    commissioned_at timestamptz,
    remarks text,
    sort_order bigint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_data_centers_code_active ON data_centers (code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_data_centers_deleted_at ON data_centers (deleted_at);
CREATE INDEX IF NOT EXISTS idx_data_centers_status ON data_centers (status);
CREATE INDEX IF NOT EXISTS idx_data_centers_name ON data_centers (name);
CREATE INDEX IF NOT EXISTS idx_data_centers_sort_order ON data_centers (sort_order);

CREATE TABLE IF NOT EXISTS rooms (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    data_center_id uuid NOT NULL REFERENCES data_centers(id),
    code varchar(50) NOT NULL,
    name varchar(150) NOT NULL,
    building varchar(100),
    floor varchar(50),
    room_number varchar(50),
    area_square_meters numeric,
    clear_height_meters numeric,
    purpose varchar(200),
    status varchar(30) NOT NULL DEFAULT 'OPERATING',
    environment_level varchar(50),
    max_load_kg numeric,
    cooling_capacity_kw numeric,
    design_power_kw numeric,
    available_power_kw numeric,
    used_power_kw numeric,
    redundancy_policy varchar(100),
    manager varchar(100),
    contact varchar(100),
    open_hours varchar(100),
    access_notes text,
    floor_plan_enabled boolean NOT NULL DEFAULT false,
    floor_plan_format varchar(20) NOT NULL DEFAULT 'SVG',
    racks_per_row bigint NOT NULL DEFAULT 8,
    remarks text,
    sort_order bigint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rooms_data_center_code ON rooms (data_center_id, code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_rooms_deleted_at ON rooms (deleted_at);
CREATE INDEX IF NOT EXISTS idx_rooms_data_center_id ON rooms (data_center_id);
CREATE INDEX IF NOT EXISTS idx_rooms_name ON rooms (name);
CREATE INDEX IF NOT EXISTS idx_rooms_status ON rooms (status);
CREATE INDEX IF NOT EXISTS idx_rooms_sort_order ON rooms (sort_order);

CREATE TABLE IF NOT EXISTS racks (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    template_id uuid,
    template_version_id uuid,
    template_code varchar(50),
    template_name varchar(150),
    template_revision bigint NOT NULL DEFAULT 0,
    template_snapshot jsonb,
    data_center_id uuid NOT NULL REFERENCES data_centers(id),
    room_id uuid NOT NULL REFERENCES rooms(id),
    code varchar(50) NOT NULL,
    name varchar(150) NOT NULL,
    type varchar(50) NOT NULL DEFAULT 'STANDARD',
    manufacturer varchar(100),
    model_number varchar(100),
    serial_number varchar(100),
    asset_number varchar(100),
    u_height bigint NOT NULL DEFAULT 42,
    width_mm bigint NOT NULL DEFAULT 600,
    depth_mm bigint NOT NULL DEFAULT 1200,
    height_mm bigint NOT NULL DEFAULT 2000,
    load_capacity_kg numeric,
    zone varchar(100),
    rack_row varchar(50),
    rack_column varchar(50),
    aisle varchar(100),
    x_coordinate numeric,
    y_coordinate numeric,
    rotation bigint NOT NULL DEFAULT 0,
    status varchar(30) NOT NULL DEFAULT 'AVAILABLE',
    manager varchar(100),
    department varchar(100),
    purpose varchar(200),
    dual_power boolean NOT NULL DEFAULT false,
    input_circuits bigint NOT NULL DEFAULT 0,
    rated_voltage numeric,
    rated_current numeric,
    rated_power_kw numeric,
    peak_power_kw numeric,
    pdu_count bigint NOT NULL DEFAULT 0,
    remarks text,
    sort_order bigint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_racks_room_code ON racks (room_id, code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_racks_deleted_at ON racks (deleted_at);
CREATE INDEX IF NOT EXISTS idx_racks_room_id ON racks (room_id);
CREATE INDEX IF NOT EXISTS idx_racks_data_center_id ON racks (data_center_id);
CREATE INDEX IF NOT EXISTS idx_racks_name ON racks (name);
CREATE INDEX IF NOT EXISTS idx_racks_status ON racks (status);
CREATE INDEX IF NOT EXISTS idx_racks_sort_order ON racks (sort_order);

CREATE TABLE IF NOT EXISTS audit_logs (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    user_id uuid,
    action varchar(100) NOT NULL,
    resource_type varchar(100) NOT NULL,
    resource_id uuid,
    request_id varchar(100),
    before_json jsonb,
    after_json jsonb,
    result varchar(20) NOT NULL,
    error_code varchar(100),
    source varchar(50)
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs (action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs (resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id ON audit_logs (resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs (request_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_deleted_at ON audit_logs (deleted_at);
