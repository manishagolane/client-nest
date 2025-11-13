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