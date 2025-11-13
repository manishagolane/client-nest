CREATE TABLE role_permissions (
  role_id VARCHAR(26) REFERENCES roles(id) ON DELETE CASCADE,
  permission_id VARCHAR(26) REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

