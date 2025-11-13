-- name: AssignPermissionToRole :exec
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id 
FROM roles r, permissions p
WHERE r.name = $1 AND p.name = ANY($2::text[]);

-- name: GetRolePermissions :many
SELECT p.name FROM permissions p
JOIN role_permissions rp ON p.id = rp.permission_id
JOIN roles r ON rp.role_id = r.id
WHERE r.name = $1;

-- (Full Access)
-- INSERT INTO role_permissions (role_id, permission_id)
-- SELECT r.id, p.id FROM roles r, permissions p
-- WHERE r.name = 'admin';

-- for Manager
-- INSERT INTO role_permissions (role_id, permission_id)
-- SELECT r.id, p.id 
-- FROM roles r, permissions p
-- WHERE r.name = 'manager' 
-- AND p.name IN ('assign_ticket', 'update_ticket_status', 'comment_on_ticket', 'close_ticket');


-- INSERT INTO role_permissions (role_id, permission_id)
-- VALUES
-- -- Admin (Full Access)
-- ((SELECT id FROM roles WHERE name = 'admin'), (SELECT id FROM permissions WHERE name = 'assign_ticket')),
-- ((SELECT id FROM roles WHERE name = 'admin'), (SELECT id FROM permissions WHERE name = 'update_ticket_status')),
-- ((SELECT id FROM roles WHERE name = 'admin'), (SELECT id FROM permissions WHERE name = 'comment_on_ticket')),
-- ((SELECT id FROM roles WHERE name = 'admin'), (SELECT id FROM permissions WHERE name = 'delete_ticket')),
-- ((SELECT id FROM roles WHERE name = 'admin'), (SELECT id FROM permissions WHERE name = 'close_ticket')),

-- -- Manager
-- ((SELECT id FROM roles WHERE name = 'manager'), (SELECT id FROM permissions WHERE name = 'assign_ticket')),
-- ((SELECT id FROM roles WHERE name = 'manager'), (SELECT id FROM permissions WHERE name = 'update_ticket_status')),
-- ((SELECT id FROM roles WHERE name = 'manager'), (SELECT id FROM permissions WHERE name = 'comment_on_ticket')),
-- ((SELECT id FROM roles WHERE name = 'manager'), (SELECT id FROM permissions WHERE name = 'close_ticket')),

-- -- Employee
-- ((SELECT id FROM roles WHERE name = 'employee'), (SELECT id FROM permissions WHERE name = 'comment_on_ticket')),
-- ((SELECT id FROM roles WHERE name = 'employee'), (SELECT id FROM permissions WHERE name = 'update_ticket_status'));