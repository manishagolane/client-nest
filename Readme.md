# CRM Service DB Setup Guide for rbac features

## **1. Generate ULIDs for Initial Data**
```go
func GetUlid() (string, error) {
    entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
    ms := ulid.Timestamp(time.Now())
    id, err := ulid.New(ms, entropy)
    if err != nil {
        return "", errors.New("failed to generate ULID")
    }
    return id.String(), nil
}
```

---
## **2. Insert Initial Roles & Permissions in Bulk**

### **4.1 Insert: Roles**  
Execute the following SQL query to insert roles:

```sql
INSERT INTO roles (id, name) VALUES
('01JNY6KP7QTRQBSABMN1C3GSER', 'admin'),
('01JNY6KP7QBD287JPSSATQWG5A', 'manager'),
('01JNY6KP7QWSVS4QZH7FFE1Z90', 'employee'),
('01JQDMXCYEPY2DCTH6A0TKA85T','customer')
;
```

### **4.2 Bulk Insert: Permissions**  
Execute the following SQL query to insert permissions:

```sql
INSERT INTO permissions (id, name, object, action) VALUES
('01JNY6KP705VG4XK44QVFGJ2QM', 'assign_ticket', 'ticket', 'assign'),
('01JNY6KP70FVXK04HABAAT6MBB', 'update_ticket_status', 'ticket', 'update_status'),
('01JNY6KP70ECY3HBGP66CGNEQB', 'comment_on_ticket', 'ticket', 'comment'),
('01JNY6KP70VK80E4ZY20Q3AV0M', 'delete_ticket', 'ticket', 'delete'),
('01JNY6KP708MT933GPRTZEE41W', 'close_ticket', 'ticket', 'close'),
('01JQDN0YW6Z6E4VEB1VSYS1F6B','create_reminder','ticket','reminder'),
('01JQDN2X65HB889JF8TR5N8QEY','cancel_reminder','ticket','cancel_reminder'),
('01JQDN3Z99HER30ZA91YRHHEWB','snooze_reminder','ticket','snooze_reminder')
;
```

### **4.3 Bulk Insert: Role-Permission Assignments**  
Execute the following SQL query to assign permissions to roles:

```sql
INSERT INTO role_permissions (role_id, permission_id) VALUES
-- Admin Role Permissions
('01JNY6KP7QTRQBSABMN1C3GSER', '01JNY6KP705VG4XK44QVFGJ2QM'),
('01JNY6KP7QTRQBSABMN1C3GSER', '01JNY6KP70FVXK04HABAAT6MBB'),
('01JNY6KP7QTRQBSABMN1C3GSER', '01JNY6KP70ECY3HBGP66CGNEQB'),
('01JNY6KP7QTRQBSABMN1C3GSER', '01JNY6KP70VK80E4ZY20Q3AV0M'),
('01JNY6KP7QTRQBSABMN1C3GSER', '01JNY6KP708MT933GPRTZEE41W'),
('01JNY6KP7QTRQBSABMN1C3GSER', '01JQDN0YW6Z6E4VEB1VSYS1F6B'),
('01JNY6KP7QTRQBSABMN1C3GSER', '01JQDN2X65HB889JF8TR5N8QEY'),
('01JNY6KP7QTRQBSABMN1C3GSER', '01JQDN3Z99HER30ZA91YRHHEWB'),


-- Manager Role Permissions
('01JNY6KP7QBD287JPSSATQWG5A', '01JNY6KP705VG4XK44QVFGJ2QM'),
('01JNY6KP7QBD287JPSSATQWG5A', '01JNY6KP70FVXK04HABAAT6MBB'),
('01JNY6KP7QBD287JPSSATQWG5A', '01JNY6KP70ECY3HBGP66CGNEQB'),
('01JNY6KP7QBD287JPSSATQWG5A', '01JNY6KP708MT933GPRTZEE41W'),
('01JNY6KP7QBD287JPSSATQWG5A', '01JQDN0YW6Z6E4VEB1VSYS1F6B'),
('01JNY6KP7QBD287JPSSATQWG5A', '01JQDN2X65HB889JF8TR5N8QEY'),
('01JNY6KP7QBD287JPSSATQWG5A', '01JQDN3Z99HER30ZA91YRHHEWB'),


-- Employee Role Permissions
('01JNY6KP7QWSVS4QZH7FFE1Z90', '01JNY6KP70FVXK04HABAAT6MBB'),
('01JNY6KP7QWSVS4QZH7FFE1Z90', '01JNY6KP70ECY3HBGP66CGNEQB');
('01JNY6KP7QWSVS4QZH7FFE1Z90', '01JQDN0YW6Z6E4VEB1VSYS1F6B'),
('01JNY6KP7QWSVS4QZH7FFE1Z90', '01JQDN2X65HB889JF8TR5N8QEY'),
('01JNY6KP7QWSVS4QZH7FFE1Z90', '01JQDN3Z99HER30ZA91YRHHEWB'),

-- Customer Role Permissions
('01JQDMXCYEPY2DCTH6A0TKA85T', '01JQDN0YW6Z6E4VEB1VSYS1F6B'),
('01JQDMXCYEPY2DCTH6A0TKA85T', '01JQDN2X65HB889JF8TR5N8QEY'),
('01JQDMXCYEPY2DCTH6A0TKA85T', '01JQDN3Z99HER30ZA91YRHHEWB');

```
---
# CRM Database Schema
outlines the database schema for the CRM system, detailing the tables, relationships, and the approach taken to establish foreign key constraints.

## Tables and Relationships

## Create Roles table as it's referenced by role_permissions
```sql 
CREATE TABLE roles (
  id VARCHAR(26) PRIMARY KEY,
  name TEXT UNIQUE NOT NULL
);
```

## Create Permissions table as it's referenced by role_permissions
```sql 
CREATE TABLE permissions (
  id VARCHAR(26) PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  object TEXT NOT NULL,
  action TEXT NOT NULL
);
```

## Create Role-Permissions table 
```sql 
CREATE TABLE role_permissions (
  role_id VARCHAR(26) REFERENCES roles(id) ON DELETE CASCADE,
  permission_id VARCHAR(26) REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);
```
##  **Teams Table**

The `teams` table stores information about different teams in the organization.

```sql
CREATE TABLE teams (
  id          VARCHAR(26) PRIMARY KEY,
  name        VARCHAR(255) NOT NULL UNIQUE,
  created_at  TIMESTAMPTZ DEFAULT NOW(),
  updated_at  TIMESTAMPTZ DEFAULT NOW(),
  deleted_at  TIMESTAMPTZ NULL
);
```

###  **Employees Table (Initial Creation)**

The `employees` table is initially created **without** the `manager_id` and `team_id` columns to prevent circular dependencies.

```sql
CREATE TABLE employees (
  id           VARCHAR(26) PRIMARY KEY,
  name         VARCHAR(255) NOT NULL,
  email        VARCHAR(255) NOT NULL UNIQUE,
  password     VARCHAR(255) NOT NULL,
  phone        CHAR(10) NOT NULL,
  role_id      VARCHAR(26) NOT NULL REFERENCES roles(id) ON DELETE SET NULL,
  created_at   TIMESTAMPTZ DEFAULT NOW(),
  updated_at   TIMESTAMPTZ DEFAULT NOW(),
  deleted_at   TIMESTAMPTZ NULL
);
```

### 3. **Altering Tables to Add Foreign Keys**

After both tables exist, the following `ALTER TABLE` statements are executed to add foreign key references:

```sql
ALTER TABLE teams ADD COLUMN manager_id VARCHAR(26) REFERENCES employees(id) ON DELETE SET NULL;
ALTER TABLE employees ADD COLUMN team_id VARCHAR(26) REFERENCES teams(id) ON DELETE SET NULL;
ALTER TABLE employees ADD COLUMN manager_id VARCHAR(26) REFERENCES employees(id) ON DELETE SET NULL;
```
## Create Tickets table 
```sql 
CREATE TABLE tickets (
  id VARCHAR(26) PRIMARY KEY,
  created_by VARCHAR(26) NOT NULL,
  created_by_type TEXT NULL, 
  assigned_to VARCHAR(26) REFERENCES employees(id) NULL,
  team_id VARCHAR(26) REFERENCES teams(id) NULL,
  category TEXT[] DEFAULT '{}',  
  priority TEXT NOT NULL DEFAULT 'low', 
  status TEXT NOT NULL DEFAULT 'open',  
  tags TEXT[] DEFAULT '{}',
  response_time INTERVAL,
  watchers JSONB DEFAULT '[]',
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL
);
```

## Create Ticket Attachments 
```sql 
CREATE TABLE ticket_attachments (
  id         VARCHAR(26) PRIMARY KEY,
  ticket_id  VARCHAR(26) REFERENCES tickets(id) ON DELETE CASCADE, -- Links attachment to ticket
  file_url   TEXT NULL, -- Stores presigned S3 URL or file path
  uploaded_by VARCHAR(26) NOT NULL, -- Tracks who uploaded the file
  uploader_type TEXT NULL
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL
);
```