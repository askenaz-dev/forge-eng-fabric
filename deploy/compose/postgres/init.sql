-- Bootstrap auxiliary databases / users for stack components.
CREATE USER keycloak WITH PASSWORD 'keycloak';
CREATE DATABASE keycloak OWNER keycloak;

CREATE USER openfga WITH PASSWORD 'openfga';
CREATE DATABASE openfga OWNER openfga;

-- Forge application schemas (owned by `forge`)
CREATE DATABASE forge_control_plane OWNER forge;
CREATE DATABASE forge_registry      OWNER forge;
CREATE DATABASE forge_audit         OWNER forge;
CREATE DATABASE forge_litellm       OWNER forge;
