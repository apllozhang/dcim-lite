-- R3: device types, devices, U-slot positions, occupancies, histories

CREATE TABLE IF NOT EXISTS device_types (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    code varchar(50) NOT NULL,
    name varchar(150) NOT NULL,
    category varchar(40) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    default_height_u bigint NOT NULL DEFAULT 1,
    default_weight_kg numeric,
    default_rated_power_w numeric,
    default_peak_power_w numeric,
    default_dual_power boolean NOT NULL DEFAULT false,
    description text,
    sort_order bigint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_device_types_code_active ON device_types (code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_device_types_deleted_at ON device_types (deleted_at);

CREATE TABLE IF NOT EXISTS devices (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    type_id uuid NOT NULL REFERENCES device_types(id),
    code varchar(80) NOT NULL,
    name varchar(150) NOT NULL,
    asset_number varchar(100),
    serial_number varchar(120),
    manufacturer varchar(100),
    model_number varchar(100),
    specification varchar(500),
    firmware_version varchar(100),
    purchase_batch varchar(100),
    warranty_expires_at timestamptz,
    organization varchar(150),
    manager varchar(100),
    contact varchar(100),
    business_system varchar(150),
    application_name varchar(150),
    lifecycle_status varchar(30) NOT NULL DEFAULT 'WAITING_RACK',
    height_u bigint NOT NULL DEFAULT 1,
    width_mm bigint,
    depth_mm bigint,
    height_mm bigint,
    weight_kg numeric,
    rated_power_w numeric,
    peak_power_w numeric,
    input_voltage numeric,
    dual_power_required boolean NOT NULL DEFAULT false,
    management_ip varchar(64),
    business_ip varchar(64),
    mac_address varchar(64),
    management_protocol varchar(100),
    monitoring_status varchar(50),
    external_qr_code varchar(255),
    external_qr_code_url varchar(500),
    tags varchar(500),
    remarks text
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_code_active ON devices (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_asset_active ON devices (asset_number) WHERE deleted_at IS NULL AND asset_number <> '';
CREATE INDEX IF NOT EXISTS idx_devices_deleted_at ON devices (deleted_at);
CREATE INDEX IF NOT EXISTS idx_devices_type_id ON devices (type_id);
CREATE INDEX IF NOT EXISTS idx_devices_name ON devices (name);
CREATE INDEX IF NOT EXISTS idx_devices_lifecycle_status ON devices (lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_devices_serial_number ON devices (serial_number);
CREATE INDEX IF NOT EXISTS idx_devices_management_ip ON devices (management_ip);
CREATE INDEX IF NOT EXISTS idx_devices_business_ip ON devices (business_ip);
CREATE INDEX IF NOT EXISTS idx_devices_mac_address ON devices (mac_address);

CREATE TABLE IF NOT EXISTS rack_device_positions (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    device_id uuid NOT NULL REFERENCES devices(id),
    rack_id uuid NOT NULL REFERENCES racks(id),
    room_id uuid NOT NULL REFERENCES rooms(id),
    data_center_id uuid NOT NULL REFERENCES data_centers(id),
    start_u bigint NOT NULL,
    height_u bigint NOT NULL,
    end_u bigint NOT NULL,
    orientation varchar(20) NOT NULL DEFAULT 'NORMAL',
    installed_at timestamptz NOT NULL,
    installed_by uuid,
    reason varchar(500)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rack_device_positions_device_active ON rack_device_positions (device_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_rack_device_positions_deleted_at ON rack_device_positions (deleted_at);
CREATE INDEX IF NOT EXISTS idx_rack_device_positions_rack_id ON rack_device_positions (rack_id);
CREATE INDEX IF NOT EXISTS idx_rack_device_positions_room_id ON rack_device_positions (room_id);
CREATE INDEX IF NOT EXISTS idx_rack_device_positions_data_center_id ON rack_device_positions (data_center_id);

CREATE TABLE IF NOT EXISTS rack_u_occupancies (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    position_id uuid NOT NULL REFERENCES rack_device_positions(id),
    device_id uuid NOT NULL REFERENCES devices(id),
    rack_id uuid NOT NULL REFERENCES racks(id),
    start_u bigint NOT NULL,
    end_u bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rack_u_occupancies_deleted_at ON rack_u_occupancies (deleted_at);
CREATE INDEX IF NOT EXISTS idx_rack_u_occupancies_device_id ON rack_u_occupancies (device_id);
CREATE INDEX IF NOT EXISTS idx_rack_u_occupancies_position_id ON rack_u_occupancies (position_id);
CREATE INDEX IF NOT EXISTS idx_rack_u_occupancies_rack_id ON rack_u_occupancies (rack_id);
-- U-slot exclusivity: same rack, inclusive range overlap forbidden
CREATE EXTENSION IF NOT EXISTS btree_gist;
ALTER TABLE rack_u_occupancies
    DROP CONSTRAINT IF EXISTS rack_u_occupancies_no_overlap;
ALTER TABLE rack_u_occupancies
    ADD CONSTRAINT rack_u_occupancies_no_overlap
    EXCLUDE USING gist (rack_id WITH =, int8range(start_u, end_u, '[]') WITH &&)
    WHERE (deleted_at IS NULL);

CREATE TABLE IF NOT EXISTS device_position_histories (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    device_id uuid NOT NULL REFERENCES devices(id),
    operation varchar(30) NOT NULL,
    from_position jsonb,
    to_position jsonb,
    reason varchar(500),
    actor_id uuid,
    request_id varchar(100)
);
CREATE INDEX IF NOT EXISTS idx_device_position_histories_device_id ON device_position_histories (device_id);
CREATE INDEX IF NOT EXISTS idx_device_position_histories_deleted_at ON device_position_histories (deleted_at);
