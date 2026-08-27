DROP ROW POLICY IF EXISTS queryapi_project_isolation ON logs;

REVOKE settings_allow_custom_setting_read, settings_allow_custom_setting_write ON *.* FROM `{{QUERYAPI_USER}}`;
REVOKE SELECT ON logs FROM `{{QUERYAPI_USER}}`;

DROP TABLE IF EXISTS logs;
