REVOKE ALL PRIVILEGES ON authorization_codes, hub_sessions, applications FROM identity_app;
DROP TABLE authorization_codes;
DROP TABLE hub_sessions;
DROP TABLE applications;
