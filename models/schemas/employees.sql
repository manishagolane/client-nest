CREATE TABLE employees (
  id              VARCHAR(26) PRIMARY KEY,
  name            VARCHAR(255) NOT NULL,
  email           VARCHAR(255) NOT NULL UNIQUE,
  password        VARCHAR(255) NOT NULL,
  phone           CHAR(10) NOT NULL,
  role_id         VARCHAR(26) NOT NULL REFERENCES roles(id) ON DELETE SET NULL,
  team_id         VARCHAR(26) REFERENCES teams(id) ON DELETE SET NULL,
  manager_id      VARCHAR(26) REFERENCES employees(id) ON DELETE SET NULL,  -- Employee hierarchy (Manager ID)
  created_at      TIMESTAMPTZ DEFAULT NOW(),
  updated_at      TIMESTAMPTZ DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ NULL
);