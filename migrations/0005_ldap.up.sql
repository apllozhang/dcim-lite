-- R5c: LDAP config (single-row)

CREATE TABLE IF NOT EXISTS ldap_configs (
    id uuid PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    deleted_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    enabled boolean NOT NULL DEFAULT false,
    url varchar(500) NOT NULL DEFAULT '',
    use_tls boolean NOT NULL DEFAULT false,
    skip_tls_verify boolean NOT NULL DEFAULT false,
    bind_dn varchar(255) NOT NULL DEFAULT '',
    bind_password varchar(500) NOT NULL DEFAULT '',
    base_dn varchar(255) NOT NULL DEFAULT '',
    user_filter varchar(500) NOT NULL DEFAULT '(|(uid={{username}})(sAMAccountName={{username}}))',
    username_attribute varchar(100) NOT NULL DEFAULT 'uid',
    display_name_attribute varchar(100) NOT NULL DEFAULT 'displayName',
    email_attribute varchar(100) NOT NULL DEFAULT 'mail',
    default_role varchar(100) NOT NULL DEFAULT 'user',
    allow_local_fallback boolean NOT NULL DEFAULT true,
    group_search_base_dn varchar(255) NOT NULL DEFAULT '',
    group_filter varchar(500) NOT NULL DEFAULT '(&(objectClass=groupOfNames)(member={{userDN}}))',
    group_attribute varchar(100) NOT NULL DEFAULT 'cn',
    group_role_mappings jsonb
);
