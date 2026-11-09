-- The store that massimo v0.10.0-alpha.1 (6f69515) wrote against the fake Cube,
-- made by tools/store-fixture/make.sh on 2026-11-09: a text dump
-- (sqlite3 .dump) that TestMigratesReleasedStores loads and migrates.
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE schema_version (
    version    INTEGER NOT NULL,   -- the migration applied
    min_reader INTEGER NOT NULL,   -- the oldest schema version a binary may be built for and still open this file
    applied_at TEXT    NOT NULL,
    applied_by TEXT    NOT NULL    -- the massimo version string
);
INSERT INTO schema_version VALUES(1,1,'2026-11-09T17:25:15.613Z','massimo v0.10.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(2,1,'2026-11-09T17:25:15.626Z','massimo v0.10.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(3,1,'2026-11-09T17:25:15.626Z','massimo v0.10.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(4,1,'2026-11-09T17:25:15.627Z','massimo v0.10.0-alpha.1 (go1.26.8, linux/amd64)');
CREATE TABLE roles (
    id         INTEGER PRIMARY KEY,
    key        TEXT    NOT NULL,              -- 'owner', 'manager', 'operator', 'guest', 'viewer', 'integration', or the key of a created role
    name       TEXT    NOT NULL DEFAULT '',   -- display name; '' shows the key's translated default
    builtin    INTEGER NOT NULL DEFAULT 0,    -- 1: seeded; owner is builtin and immutable
    disabled   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,
    updated_by INTEGER REFERENCES users (id),
    created_by INTEGER REFERENCES users (id),
    version    INTEGER NOT NULL DEFAULT 1,    -- a role replicates with its set; a change of the set bumps it
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT '',   -- node.id of the node that made the last change
    UNIQUE (key)
);
INSERT INTO roles VALUES(1,'owner','',1,0,'2026-11-09T17:25:15.612Z','2026-11-09T17:25:15.612Z',NULL,NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO roles VALUES(2,'manager','',1,0,'2026-11-09T17:25:15.612Z','2026-11-09T17:25:15.612Z',NULL,NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO roles VALUES(3,'operator','',1,0,'2026-11-09T17:25:15.612Z','2026-11-09T17:25:15.612Z',NULL,NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO roles VALUES(4,'guest','',1,0,'2026-11-09T17:25:15.612Z','2026-11-09T17:25:15.612Z',NULL,NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO roles VALUES(5,'viewer','',1,0,'2026-11-09T17:25:15.612Z','2026-11-09T17:25:15.612Z',NULL,NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO roles VALUES(6,'integration','',1,0,'2026-11-09T17:25:15.612Z','2026-11-09T17:25:15.612Z',NULL,NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
CREATE TABLE role_capabilities (
    role_id    INTEGER NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    capability TEXT    NOT NULL,              -- a capability key, docs/store.md section 6
    PRIMARY KEY (role_id, capability)
);
INSERT INTO role_capabilities VALUES(2,'read');
INSERT INTO role_capabilities VALUES(2,'setpoint');
INSERT INTO role_capabilities VALUES(2,'lock');
INSERT INTO role_capabilities VALUES(2,'configure');
INSERT INTO role_capabilities VALUES(2,'rename');
INSERT INTO role_capabilities VALUES(2,'structure');
INSERT INTO role_capabilities VALUES(2,'pair');
INSERT INTO role_capabilities VALUES(2,'discover');
INSERT INTO role_capabilities VALUES(2,'backup');
INSERT INTO role_capabilities VALUES(2,'restore');
INSERT INTO role_capabilities VALUES(2,'guests');
INSERT INTO role_capabilities VALUES(3,'read');
INSERT INTO role_capabilities VALUES(3,'setpoint');
INSERT INTO role_capabilities VALUES(3,'lock');
INSERT INTO role_capabilities VALUES(3,'configure');
INSERT INTO role_capabilities VALUES(3,'discover');
INSERT INTO role_capabilities VALUES(3,'backup');
INSERT INTO role_capabilities VALUES(4,'read');
INSERT INTO role_capabilities VALUES(4,'setpoint');
INSERT INTO role_capabilities VALUES(4,'configure');
INSERT INTO role_capabilities VALUES(5,'read');
INSERT INTO role_capabilities VALUES(6,'read');
INSERT INTO role_capabilities VALUES(6,'setpoint');
INSERT INTO role_capabilities VALUES(6,'lock');
INSERT INTO role_capabilities VALUES(2,'preferences');
INSERT INTO role_capabilities VALUES(3,'preferences');
INSERT INTO role_capabilities VALUES(5,'preferences');
CREATE TABLE capabilities_disabled (
    capability  TEXT    NOT NULL,
    disabled_at TEXT    NOT NULL,
    disabled_by INTEGER REFERENCES users (id),
    PRIMARY KEY (capability)
);
CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    uuid          TEXT    NOT NULL UNIQUE,
    name          TEXT    NOT NULL,              -- the login name, lower case
    display_name  TEXT    NOT NULL DEFAULT '',
    password_hash TEXT    NOT NULL DEFAULT '',   -- argon2id in PHC string form; '' = no password login (keys only)
    role_id       INTEGER NOT NULL REFERENCES roles (id),
    disabled      INTEGER NOT NULL DEFAULT 0,
    expires_at    TEXT,                          -- guests: disabled from this time on
    invited_by    INTEGER REFERENCES users (id),
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id),
    last_login_at TEXT,
    version       INTEGER NOT NULL DEFAULT 1,
    deleted_at    TEXT,
    origin        TEXT    NOT NULL DEFAULT '', auth_version INTEGER NOT NULL DEFAULT 1,
    UNIQUE (name)
);
INSERT INTO users VALUES(1,'01a1c157-bef0-7c7e-83e5-58f32aa68af7','owner','','',1,0,NULL,NULL,'2026-11-09T17:25:15.632Z','2026-11-09T17:25:15.632Z',NULL,NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f',1);
INSERT INTO users VALUES(2,'01a1c157-c1ea-7732-beaa-4377d6937478','anna','','$argon2id$v=19$m=65536,t=1,p=4$ZcG6N8IWT43BTT1BBrCmdA$AhB2JDPbYqDr9diAxyD15axWDRl7TAl8q734PfMys6g',5,0,NULL,NULL,'2026-11-09T17:25:16.394Z','2026-11-09T17:25:16.394Z',NULL,NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f',1);
CREATE TABLE tokens (
    id             INTEGER PRIMARY KEY,
    uuid           TEXT    NOT NULL UNIQUE,
    user_id        INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind           TEXT    NOT NULL CHECK (kind IN ('session', 'api')),
    hash           TEXT    NOT NULL UNIQUE,      -- SHA-256 of the token, hex; the token itself is never stored
    label          TEXT    NOT NULL DEFAULT '',  -- 'MASSIMO_API_TOKEN', 'Loxone', 'Firefox on the laptop'
    created_at     TEXT    NOT NULL,
    expires_at     TEXT,                         -- NULL: never (API keys); sessions: moved on every use
    last_used_at   TEXT,
    last_used_from TEXT    NOT NULL DEFAULT '',  -- the client's address
    revoked_at     TEXT,
    version        INTEGER NOT NULL DEFAULT 1,
    deleted_at     TEXT,
    origin         TEXT    NOT NULL DEFAULT ''
);
INSERT INTO tokens VALUES(1,'01a1c157-bef1-720d-8d1f-d7205ce73d71',1,'api','3dbd5e608845edf5f2de485da50c0a11655a49b1b5a2188168036453ab3f1b64','MASSIMO_API_TOKEN','2026-11-09T17:25:15.633Z',NULL,'2026-11-09T17:25:15.815Z','127.0.0.1',NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO tokens VALUES(2,'01a1c157-c1fe-7750-9bdf-248f7803baa4',2,'api','fe5ffecb9386a115781be21b0b0c59662503867ea24533995b5e47c3e767b41c','fixture','2026-11-09T17:25:16.414Z',NULL,'2026-11-09T17:25:16.427Z','127.0.0.1',NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
CREATE TABLE settings (
    key        TEXT    NOT NULL,   -- 'stale_after', 'poll_interval', 'node.id', 'tenant.id', …
    value      TEXT    NOT NULL,   -- JSON: a string, a number or a document
    updated_at TEXT    NOT NULL,
    updated_by INTEGER REFERENCES users (id),
    version    INTEGER NOT NULL DEFAULT 1,
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (key)
);
INSERT INTO settings VALUES('node.id','"01a1c157-bedc-7e51-b390-0f36fcac1f2f"','2026-11-09T17:25:15.612Z',NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO settings VALUES('tenant.id','"01a1c157-bedc-7e5c-9a0d-dc50954b3ba0"','2026-11-09T17:25:15.612Z',NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO settings VALUES('stale_after','"36h"','2026-11-09T17:25:15.855Z',NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
CREATE TABLE cubes (
    id                 INTEGER PRIMARY KEY,
    uuid               TEXT    NOT NULL UNIQUE,
    serial             TEXT    NOT NULL,              -- upper case, as printed on the Cube
    configured         INTEGER NOT NULL DEFAULT 1,    -- 0: found by discovery, or removed; not served
    name               TEXT    NOT NULL DEFAULT '',
    address            TEXT    NOT NULL DEFAULT '',   -- host[:port]; '' with follow_discovery: from discovery
    follow_discovery   INTEGER NOT NULL DEFAULT 0,    -- the address follows the Cube's discovery answers
    proxy_address      TEXT    NOT NULL DEFAULT '',   -- the server's listener for this Cube, host[:port]; '' = :62910 with a single Cube, none with several
    proxy_serial       TEXT    NOT NULL DEFAULT '',   -- the serial used in discovery replies; '' = the Cube's
    hidden             TEXT    NOT NULL DEFAULT '[]', -- JSON list of radio addresses, as _IGNORE
    source             TEXT    NOT NULL DEFAULT 'env' CHECK (source IN ('env', 'ui', 'discovery', 'flag')),
    -- what discovery reported last, for GET /api/v1/cubes
    mac                TEXT    NOT NULL DEFAULT '',
    rf_address         TEXT    NOT NULL DEFAULT '',
    firmware           TEXT    NOT NULL DEFAULT '',
    discovered_address TEXT    NOT NULL DEFAULT '',
    first_seen         TEXT,
    last_seen          TEXT,
    created_at         TEXT    NOT NULL,
    updated_at         TEXT    NOT NULL,
    updated_by         INTEGER REFERENCES users (id),
    version            INTEGER NOT NULL DEFAULT 1,
    deleted_at         TEXT,
    origin             TEXT    NOT NULL DEFAULT '',
    UNIQUE (serial)
);
INSERT INTO cubes VALUES(1,'01a1c157-beef-797f-8b8c-a31cea578865','KEQ0000100',1,'Fake Cube','127.0.0.1:42109',0,'','','[]','env','','','','',NULL,NULL,'2026-11-09T17:25:15.631Z','2026-11-09T17:25:15.631Z',NULL,1,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
CREATE TABLE preferences (
    uuid       TEXT    NOT NULL UNIQUE,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key        TEXT    NOT NULL,   -- 'scheme' (the massimo-scheme/1 document), 'ui' (poll, expert, clock, language)
    value      TEXT    NOT NULL,   -- JSON
    updated_at TEXT    NOT NULL,
    version    INTEGER NOT NULL DEFAULT 1,
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, key)
);
CREATE TABLE events (
    id        INTEGER PRIMARY KEY,
    uuid      TEXT    NOT NULL UNIQUE,
    cube_id   INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    time      TEXT    NOT NULL,
    kind      TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    device    TEXT    NOT NULL DEFAULT '',       -- radio address, '' for Cube-level events
    data      TEXT    NOT NULL                   -- JSON: the event object as GET …/log returns it
);
INSERT INTO events VALUES(1,'01a1c157-c2e2-71be-9d35-186cdfa3d2b8',1,'2026-11-09T17:25:15.640Z','hello','','{"kind":"hello","time":"2026-11-09T18:25:15.640107764+01:00"}');
INSERT INTO events VALUES(2,'01a1c157-c2e2-7429-b6f5-629bceb2d7a0',1,'2026-11-09T17:25:15.640Z','metadata','','{"kind":"metadata","time":"2026-11-09T18:25:15.640509464+01:00"}');
INSERT INTO events VALUES(3,'01a1c157-c2e2-7467-b084-a60f352e016a',1,'2026-11-09T17:25:15.640Z','config','0a0107','{"kind":"config","device":"0a0107","time":"2026-11-09T18:25:15.640528564+01:00"}');
INSERT INTO events VALUES(4,'01a1c157-c2e2-74a3-8e8b-2577f8553150',1,'2026-11-09T17:25:15.640Z','config','0a010d','{"kind":"config","device":"0a010d","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.640540864+01:00"}');
INSERT INTO events VALUES(5,'01a1c157-c2e2-7521-b26a-1fc88167c35c',1,'2026-11-09T17:25:15.640Z','config','0a0108','{"kind":"config","device":"0a0108","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.640823764+01:00"}');
INSERT INTO events VALUES(6,'01a1c157-c2e2-7542-96b2-d57db18f4b7c',1,'2026-11-09T17:25:15.640Z','config','0a0109','{"kind":"config","device":"0a0109","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.640850464+01:00"}');
INSERT INTO events VALUES(7,'01a1c157-c2e2-7561-b6d7-e60e5c4d2177',1,'2026-11-09T17:25:15.640Z','config','0a010b','{"kind":"config","device":"0a010b","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.640865164+01:00"}');
INSERT INTO events VALUES(8,'01a1c157-c2e2-7581-a3d6-0f8f36cd2103',1,'2026-11-09T17:25:15.640Z','config','0a010a','{"kind":"config","device":"0a010a","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.640875364+01:00"}');
INSERT INTO events VALUES(9,'01a1c157-c2e2-75a0-9995-0d2e30ecbab5',1,'2026-11-09T17:25:15.640Z','config','0a010c','{"kind":"config","device":"0a010c","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.640885664+01:00"}');
INSERT INTO events VALUES(10,'01a1c157-c2e2-7614-84da-d4abbd402499',1,'2026-11-09T17:25:15.640Z','state','0a010d','{"kind":"state","device":"0a010d","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.640897064+01:00"}');
INSERT INTO events VALUES(11,'01a1c157-c2e2-7636-9a57-6e251d97c636',1,'2026-11-09T17:25:15.640Z','state','0a0108','{"kind":"state","device":"0a0108","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.640993364+01:00"}');
INSERT INTO events VALUES(12,'01a1c157-c2e2-7685-8df1-f4997089ce0a',1,'2026-11-09T17:25:15.641Z','state','0a0109','{"kind":"state","device":"0a0109","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.641030163+01:00"}');
INSERT INTO events VALUES(13,'01a1c157-c2e2-76ca-848d-70321d286fa7',1,'2026-11-09T17:25:15.641Z','state','0a010b','{"kind":"state","device":"0a010b","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.641050763+01:00"}');
INSERT INTO events VALUES(14,'01a1c157-c2e2-76ff-959c-e9cab6bb4e37',1,'2026-11-09T17:25:15.641Z','state','0a010a','{"kind":"state","device":"0a010a","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.641070863+01:00"}');
INSERT INTO events VALUES(15,'01a1c157-c2e2-7722-b50e-3db9a10516e8',1,'2026-11-09T17:25:15.641Z','state','0a010c','{"kind":"state","device":"0a010c","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.641088863+01:00"}');
INSERT INTO events VALUES(16,'01a1c157-c2e2-7742-987a-a4644715f9c2',1,'2026-11-09T17:25:15.641Z','state','0a1b2c','{"kind":"state","device":"0a1b2c","state":{"rf_address":"0a1b2c","type":"shutter_contact","name":"Fenster Kind 1","room_id":5,"serial":"MEQ0000001","reported":true,"state":{"mode":"vacation","window_open":true,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:15.639907464+01:00"},"time":"2026-11-09T18:25:15.641111363+01:00"}');
INSERT INTO events VALUES(17,'01a1c157-c2e2-77aa-995e-2258aaf5a2e9',1,'2026-11-09T17:25:15.641Z','connected','','{"kind":"connected","time":"2026-11-09T18:25:15.641140663+01:00"}');
INSERT INTO events VALUES(18,'01a1c157-c2e2-77cd-b87e-5e56727be171',1,'2026-11-09T17:25:16.272Z','metadata','','{"kind":"metadata","time":"2026-11-09T18:25:16.272587657+01:00"}');
CREATE TABLE audit (
    id        INTEGER PRIMARY KEY,
    uuid      TEXT    NOT NULL UNIQUE,
    time      TEXT    NOT NULL,
    user_id   INTEGER REFERENCES users (id),     -- NULL: no authentication configured, or the server itself
    token_id  INTEGER REFERENCES tokens (id),
    remote    TEXT    NOT NULL DEFAULT '',       -- the client's address
    action    TEXT    NOT NULL,                  -- 'setpoint', 'room.config', 'cube.update', 'user.add', 'login', …
    cube_id   INTEGER REFERENCES cubes (id),
    target    TEXT    NOT NULL DEFAULT '',       -- 'device:0abc12', 'room:3', 'user:anna'
    detail    TEXT    NOT NULL DEFAULT '{}',     -- JSON: the request as received, secrets removed, and the result
    result    TEXT    NOT NULL DEFAULT 'ok' CHECK (result IN ('ok', 'refused', 'failed'))
, command_id INTEGER REFERENCES commands (id) ON DELETE SET NULL);
INSERT INTO audit VALUES(1,'01a1c157-bfcf-7c91-8978-3b07281b9a0b','2026-11-09T17:25:15.854Z',1,1,'127.0.0.1','settings.update',NULL,'','{"body":{"stale_after":"36h"},"bytes":22,"method":"PUT","ms":1,"path":"/api/v1/settings","status":200}','ok',1);
INSERT INTO audit VALUES(2,'01a1c157-bfdf-7352-8e10-7eb6e6af8128','2026-11-09T17:25:15.869Z',1,1,'127.0.0.1','setpoint',1,'device:0a010a','{"body":{"mode":"manual","temp":21.5},"bytes":32,"method":"PUT","ms":1,"path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","status":200}','ok',2);
INSERT INTO audit VALUES(3,'01a1c157-c0a8-7077-875a-eea70335cdc1','2026-11-09T17:25:15.884Z',1,1,'127.0.0.1','setpoint',1,'room:1','{"body":{"mode":"manual","temp":20},"bytes":30,"method":"PUT","ms":187,"path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","status":200}','ok',4);
INSERT INTO audit VALUES(4,'01a1c157-c171-71ec-b9c4-f3012bc4f367','2026-11-09T17:25:16.085Z',1,1,'127.0.0.1','room.rename',1,'room:1','{"body":{"name":"Fixture room"},"bytes":24,"method":"PUT","ms":186,"path":"/api/v1/cubes/KEQ0000100/rooms/1","status":200}','ok',6);
INSERT INTO audit VALUES(5,'01a1c157-c1ea-7b7e-8be8-ed361101eacd','2026-11-09T17:25:16.394Z',NULL,NULL,'cli','user.add',NULL,'user:anna','{"password":true,"role":"viewer"}','ok',NULL);
INSERT INTO audit VALUES(6,'01a1c157-c1fe-7b91-a275-db576a8a528e','2026-11-09T17:25:16.414Z',NULL,NULL,'cli','token.add',NULL,'token:2','{"label":"fixture","user":"anna"}','ok',NULL);
CREATE TABLE backups (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    cube_id    INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    taken_at   TEXT    NOT NULL,
    reason     TEXT    NOT NULL CHECK (reason IN ('connect', 'daily', 'before_restore', 'manual', 'upload')),
    format     TEXT    NOT NULL CHECK (format IN ('json', 'yaml')),
    tool       TEXT    NOT NULL DEFAULT '',      -- backup.File.Tool, the version that took it
    size       INTEGER NOT NULL,                 -- bytes before compression
    sha256     TEXT    NOT NULL,
    body       BLOB    NOT NULL,                 -- the backup file, gzip
    user_id    INTEGER REFERENCES users (id),    -- who asked, for manual and before_restore
    note       TEXT    NOT NULL DEFAULT '',
    version    INTEGER NOT NULL DEFAULT 1,
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT ''
);
CREATE TABLE auth_failures (
    key          TEXT    PRIMARY KEY,            -- 'addr:<ip>' or 'user:<name>'
    window_start TEXT    NOT NULL,
    count        INTEGER NOT NULL,
    locked_until TEXT
);
CREATE TABLE commands (
    id              INTEGER PRIMARY KEY,
    uuid            TEXT    NOT NULL UNIQUE,
    parent_id       INTEGER REFERENCES commands (id) ON DELETE CASCADE,  -- a step's request
    cube_id         INTEGER REFERENCES cubes (id),
    user_id         INTEGER REFERENCES users (id),                       -- NULL: no authentication, or the server itself
    token_id        INTEGER REFERENCES tokens (id),
    idempotency_key TEXT    NOT NULL DEFAULT '',   -- the client's Idempotency-Key; '' after 24 hours
    action          TEXT    NOT NULL,              -- as audit.action; 'step' for a step
    target          TEXT    NOT NULL DEFAULT '',   -- 'device:0abc12', 'room:3', 'user:anna'
    request         TEXT    NOT NULL DEFAULT '{}', -- JSON: the method, the path, the body (secrets removed) and its hash; a step: its key and lines
    state           TEXT    NOT NULL CHECK (state IN ('queued', 'sent', 'acknowledged', 'failed', 'dropped')),
    status          INTEGER NOT NULL DEFAULT 0,    -- the HTTP status answered; 0 while running, and for a step
    result          TEXT    NOT NULL DEFAULT '{}', -- JSON: the response body; a step: discarded, the budget, the error
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL,
    finished_at     TEXT,
    version         INTEGER NOT NULL DEFAULT 1,
    deleted_at      TEXT,
    origin          TEXT    NOT NULL DEFAULT ''
);
INSERT INTO commands VALUES(1,'01a1c157-bfce-7917-af3d-92ae1dc9eb31',NULL,NULL,1,1,'','settings.update','','{"hash":"a1a56e0ab8f245f264cab562d3430faa07b670437af7d261b6497c8824be7018","method":"PUT","path":"/api/v1/settings","query":""}','acknowledged',200,replace('{\n  "commands.keep_days": {\n    "value": "0",\n    "default": "0",\n    "set": false,\n    "in_effect": "0",\n    "applies": "now"\n  },\n  "discover_interval": {\n    "value": "30m",\n    "default": "30m",\n    "set": false,\n    "in_effect": "30m",\n    "applies": "restart"\n  },\n  "log.keep_days": {\n    "value": "30",\n    "default": "30",\n    "set": false,\n    "in_effect": "30",\n    "applies": "now"\n  },\n  "node.id": {\n    "value": "01a1c157-bedc-7e51-b390-0f36fcac1f2f",\n    "set": true,\n    "read_only": true\n  },\n  "poll_interval": {\n    "value": "1m",\n    "default": "1m",\n    "set": false,\n    "in_effect": "1m",\n    "applies": "restart"\n  },\n  "refresh_interval": {\n    "value": "6h",\n    "default": "6h",\n    "set": false,\n    "in_effect": "6h",\n    "applies": "restart"\n  },\n  "stale_after": {\n    "value": "36h",\n    "default": "24h",\n    "set": true,\n    "in_effect": "36h",\n    "applies": "now"\n  },\n  "tenant.id": {\n    "value": "01a1c157-bedc-7e5c-9a0d-dc50954b3ba0",\n    "set": true,\n    "read_only": true\n  }\n}\n','\n',char(10)),'2026-11-09T17:25:15.854Z','2026-11-09T17:25:15.856Z','2026-11-09T17:25:15.856Z',2,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO commands VALUES(2,'01a1c157-bfdd-7783-94e3-ebee45654bca',NULL,1,1,1,'','setpoint','device:0a010a','{"hash":"a4003acf94e9c73f51fc3c487de1a17dcdead8493e4d06e030efbfed5cf380e4","method":"PUT","path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-09T18:25:15.870349161+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:15.869Z','2026-11-09T17:25:15.871Z','2026-11-09T17:25:15.871Z',3,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO commands VALUES(3,'01a1c157-bfde-7715-bafa-01907a4b2cbe',2,1,1,1,'','step','device:0a010a','{"key":"setpoint:device:0a010a","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:15.870349161+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:15.870Z','2026-11-09T17:25:15.871Z','2026-11-09T17:25:15.871Z',3,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO commands VALUES(4,'01a1c157-bfec-773b-af05-5ca4827ef888',NULL,1,1,1,'','setpoint','room:1','{"hash":"a668a611353079d05040a7b7117ab0201078669851996d42d0d1d7f761b42b96","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-09T18:25:16.071413959+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:15.884Z','2026-11-09T17:25:16.072Z','2026-11-09T17:25:16.072Z',3,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO commands VALUES(5,'01a1c157-bfed-74f7-99fc-d10fec212060',4,1,1,1,'','step','','{"key":"setpoint:room:1","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:16.071413959+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:15.885Z','2026-11-09T17:25:16.071Z','2026-11-09T17:25:16.071Z',3,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO commands VALUES(6,'01a1c157-c0b5-7cbb-8aa2-0f2cd5129ebc',NULL,1,1,1,'','room.rename','room:1','{"hash":"0d3ae207663d7e2209c687cfb34abbb665b1c5e3cdee82c501293f945e264aeb","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1","query":""}','acknowledged',200,replace('{\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:16.085Z','2026-11-09T17:25:16.273Z','2026-11-09T17:25:16.273Z',3,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
INSERT INTO commands VALUES(7,'01a1c157-c0b6-77d4-bd4f-914296e03a54',6,1,1,1,'','step','','{"key":"metadata","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:16.071413959+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:16.086Z','2026-11-09T17:25:16.272Z','2026-11-09T17:25:16.272Z',3,NULL,'01a1c157-bedc-7e51-b390-0f36fcac1f2f');
CREATE INDEX tokens_user ON tokens (user_id);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX audit_time ON audit (time);
CREATE INDEX audit_user ON audit (user_id, time);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');
CREATE INDEX commands_key ON commands (user_id, idempotency_key);
CREATE INDEX events_cube ON events (cube_id);
CREATE INDEX events_cube_kind ON events (cube_id, kind);
CREATE INDEX events_cube_device ON events (cube_id, device);
COMMIT;
