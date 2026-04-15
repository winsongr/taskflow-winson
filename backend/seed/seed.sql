INSERT INTO users (id, name, email, password) VALUES
  ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Test User', 'test@example.com',
   '$2a$12$hyGoNFlu1XIv.FOOWZkckOGSaRxLG2iFyuA0gt1B3XdEHGvmdQpUm')
ON CONFLICT (email) DO NOTHING;

INSERT INTO projects (id, name, description, owner_id) VALUES
  ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'Website Redesign', 'Q2 website overhaul project',
   'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')
ON CONFLICT DO NOTHING;

INSERT INTO tasks (id, title, description, status, priority, project_id, assignee_id, created_by, due_date) VALUES
  ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a31', 'Design homepage mockup', 'Create wireframes and high-fidelity mockups', 'done', 'high',
   'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', '2026-04-20'),
  ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a32', 'Implement navigation bar', 'Responsive nav with mobile hamburger menu', 'in_progress', 'medium',
   'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', '2026-04-25'),
  ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a33', 'Set up CI/CD pipeline', 'Configure GitHub Actions for automated testing', 'todo', 'low',
   'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', NULL, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', '2026-05-01')
ON CONFLICT DO NOTHING;
