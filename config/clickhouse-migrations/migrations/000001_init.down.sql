DROP ROW POLICY IF EXISTS queryapi_project_isolation ON logs;

REVOKE SELECT ON logs FROM queryapi;

DROP TABLE IF EXISTS logs;
