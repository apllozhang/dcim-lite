-- R5b: operation policy + approval records

CREATE TABLE IF NOT EXISTS operation_policies (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    assign_approval_enabled boolean NOT NULL DEFAULT false,
    move_approval_enabled boolean NOT NULL DEFAULT false,
    approval_mode varchar(20) NOT NULL DEFAULT 'SINGLE',
    default_approver_role varchar(100) NOT NULL DEFAULT 'system_admin'
);

CREATE TABLE IF NOT EXISTS approval_records (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    device_id uuid NOT NULL REFERENCES devices(id),
    operation varchar(30) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'PENDING',
    requested_by uuid,
    requested_at timestamptz NOT NULL,
    requested_device_version bigint NOT NULL,
    source_position jsonb,
    target_rack_id uuid NOT NULL REFERENCES racks(id),
    target_start_u bigint NOT NULL,
    target_height_u bigint NOT NULL,
    target_orientation varchar(20) NOT NULL DEFAULT 'NORMAL',
    reason varchar(500),
    decided_by uuid,
    decided_at timestamptz,
    decision_comment varchar(500)
);
CREATE INDEX IF NOT EXISTS idx_approval_records_device_id ON approval_records (device_id);
CREATE INDEX IF NOT EXISTS idx_approval_records_status ON approval_records (status);
CREATE INDEX IF NOT EXISTS idx_approval_records_deleted_at ON approval_records (deleted_at);
