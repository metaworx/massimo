-- The store that massimo v0.11.0-alpha.1 (f11ca72) wrote against the fake Cube,
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
INSERT INTO schema_version VALUES(1,1,'2026-11-09T17:25:22.636Z','massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(2,1,'2026-11-09T17:25:22.646Z','massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(3,1,'2026-11-09T17:25:22.646Z','massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(4,1,'2026-11-09T17:25:22.647Z','massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(5,1,'2026-11-09T17:25:22.649Z','massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(6,1,'2026-11-09T17:25:22.649Z','massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(7,1,'2026-11-09T17:25:22.651Z','massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)');
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
INSERT INTO roles VALUES(1,'owner','',1,0,'2026-11-09T17:25:22.635Z','2026-11-09T17:25:22.635Z',NULL,NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO roles VALUES(2,'manager','',1,0,'2026-11-09T17:25:22.635Z','2026-11-09T17:25:22.635Z',NULL,NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO roles VALUES(3,'operator','',1,0,'2026-11-09T17:25:22.635Z','2026-11-09T17:25:22.635Z',NULL,NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO roles VALUES(4,'guest','',1,0,'2026-11-09T17:25:22.635Z','2026-11-09T17:25:22.635Z',NULL,NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO roles VALUES(5,'viewer','',1,0,'2026-11-09T17:25:22.635Z','2026-11-09T17:25:22.635Z',NULL,NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO roles VALUES(6,'integration','',1,0,'2026-11-09T17:25:22.635Z','2026-11-09T17:25:22.635Z',NULL,NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
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
INSERT INTO role_capabilities VALUES(2,'restart');
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
    origin        TEXT    NOT NULL DEFAULT '', auth_version INTEGER NOT NULL DEFAULT 1, opaque_record BLOB, opaque_ksf TEXT NOT NULL DEFAULT '',
    UNIQUE (name)
);
INSERT INTO users VALUES(1,'01a1c157-da61-7968-ac9e-876ba1a9d848','owner','','',1,0,NULL,NULL,'2026-11-09T17:25:22.657Z','2026-11-09T17:25:22.657Z',NULL,NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe',1,NULL,'');
INSERT INTO users VALUES(2,'01a1c157-de1f-7839-a6cc-a031a3e00f89','anna','','',5,0,NULL,NULL,'2026-11-09T17:25:23.615Z','2026-11-09T17:25:23.615Z',NULL,NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe',2,X'58cf65c3cde748698147e98e6b0cfdfafef0ba25dc09004ab2dacec210248c35a4c5c62eb43a93c2206c76eb8959d7ba197144d5332b7eeb47c560fbaad3da0618b33414240cb34c0c0e0098fe73271b0efe35b0483e03cc6fee4bda9aad2055c0a46ddaedc7f73e3da756bef6b9c9790556af2854e3339cbcf71468452ab13baa5f1285a3ebe210c3095cda5ac8a29a2af8994b41a45d6796b08918274486e232489a4abbb69d006e575e50c43451a7f2e5d4ad88abf3bab15ac0897df03077','m=19456,t=2,p=1');
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
INSERT INTO tokens VALUES(1,'01a1c157-da61-7cda-96c5-bf1a70207b1c',1,'api','3dbd5e608845edf5f2de485da50c0a11655a49b1b5a2188168036453ab3f1b64','MASSIMO_API_TOKEN','2026-11-09T17:25:22.657Z',NULL,'2026-11-09T17:25:22.836Z','127.0.0.1',NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO tokens VALUES(2,'01a1c157-de42-75c4-baf0-a60cd3617e68',2,'api','e24e162476f1d9717f0950439c622cb69d769e1980e58abf671fbadcaa2c4dde','fixture','2026-11-09T17:25:23.650Z',NULL,'2026-11-09T17:25:23.679Z','127.0.0.1',NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
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
INSERT INTO settings VALUES('node.id','"01a1c157-da4c-7306-9b14-5e8dbe986fbe"','2026-11-09T17:25:22.636Z',NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO settings VALUES('tenant.id','"01a1c157-da4c-730f-b613-f79038ea987a"','2026-11-09T17:25:22.636Z',NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO settings VALUES('stale_after','"36h"','2026-11-09T17:25:22.870Z',NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
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
INSERT INTO cubes VALUES(1,'01a1c157-da60-7577-b953-9d57027dd108','KEQ0000100',1,'Fake Cube','127.0.0.1:59717',0,'','','[]','env','','','','',NULL,NULL,'2026-11-09T17:25:22.656Z','2026-11-09T17:25:22.656Z',NULL,1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
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
INSERT INTO events VALUES(1,'01a1c157-de51-7ca7-a6b0-ff6e80bf4b44',1,'2026-11-09T17:25:22.664Z','hello','','{"kind":"hello","time":"2026-11-09T18:25:22.664419297+01:00"}');
INSERT INTO events VALUES(2,'01a1c157-de51-7f3c-bd71-57fd9a0cfdc9',1,'2026-11-09T17:25:22.664Z','metadata','','{"kind":"metadata","time":"2026-11-09T18:25:22.664766497+01:00"}');
INSERT INTO events VALUES(3,'01a1c157-de52-7070-9179-073118e8a18c',1,'2026-11-09T17:25:22.665Z','config','0a0107','{"kind":"config","device":"0a0107","time":"2026-11-09T18:25:22.665056597+01:00"}');
INSERT INTO events VALUES(4,'01a1c157-de52-70b7-9a1f-cd5dc1bab91a',1,'2026-11-09T17:25:22.665Z','config','0a010d','{"kind":"config","device":"0a010d","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.665141697+01:00"}');
INSERT INTO events VALUES(5,'01a1c157-de52-7191-a8a3-090f47d9cc08',1,'2026-11-09T17:25:22.666Z','config','0a0108','{"kind":"config","device":"0a0108","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666305397+01:00"}');
INSERT INTO events VALUES(6,'01a1c157-de52-71d0-8e1f-bc08b6d53cb8',1,'2026-11-09T17:25:22.666Z','config','0a0109','{"kind":"config","device":"0a0109","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666356497+01:00"}');
INSERT INTO events VALUES(7,'01a1c157-de52-7203-b7f5-54ee916a0109',1,'2026-11-09T17:25:22.666Z','config','0a010b','{"kind":"config","device":"0a010b","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666372197+01:00"}');
INSERT INTO events VALUES(8,'01a1c157-de52-7237-aef2-665a51234f76',1,'2026-11-09T17:25:22.666Z','config','0a010a','{"kind":"config","device":"0a010a","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666385297+01:00"}');
INSERT INTO events VALUES(9,'01a1c157-de52-726d-a5fe-2b2cb1d3ace5',1,'2026-11-09T17:25:22.666Z','config','0a010c','{"kind":"config","device":"0a010c","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666397397+01:00"}');
INSERT INTO events VALUES(10,'01a1c157-de52-72a5-adda-8414d0b63a33',1,'2026-11-09T17:25:22.666Z','state','0a010d','{"kind":"state","device":"0a010d","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666409897+01:00"}');
INSERT INTO events VALUES(11,'01a1c157-de52-72e9-8f43-7a40a02dc1f3',1,'2026-11-09T17:25:22.666Z','state','0a0108','{"kind":"state","device":"0a0108","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666549797+01:00"}');
INSERT INTO events VALUES(12,'01a1c157-de52-7389-8487-d3f661260196',1,'2026-11-09T17:25:22.666Z','state','0a0109','{"kind":"state","device":"0a0109","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666578797+01:00"}');
INSERT INTO events VALUES(13,'01a1c157-de52-73c6-a057-898fd15e5f87',1,'2026-11-09T17:25:22.666Z','state','0a010b','{"kind":"state","device":"0a010b","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666595597+01:00"}');
INSERT INTO events VALUES(14,'01a1c157-de52-73fe-a5c2-ed4f8e6d66bc',1,'2026-11-09T17:25:22.666Z','state','0a010a','{"kind":"state","device":"0a010a","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666609497+01:00"}');
INSERT INTO events VALUES(15,'01a1c157-de52-7432-b047-0a5ee3916f12',1,'2026-11-09T17:25:22.666Z','state','0a010c','{"kind":"state","device":"0a010c","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666622697+01:00"}');
INSERT INTO events VALUES(16,'01a1c157-de52-746a-92a0-9dacc82e8932',1,'2026-11-09T17:25:22.666Z','state','0a1b2c','{"kind":"state","device":"0a1b2c","state":{"rf_address":"0a1b2c","type":"shutter_contact","name":"Fenster Kind 1","room_id":5,"serial":"MEQ0000001","reported":true,"state":{"mode":"vacation","window_open":true,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-09T18:25:22.664289097+01:00"},"time":"2026-11-09T18:25:22.666636697+01:00"}');
INSERT INTO events VALUES(17,'01a1c157-de52-74ac-ad54-d0dee70fb490',1,'2026-11-09T17:25:22.666Z','connected','','{"kind":"connected","time":"2026-11-09T18:25:22.666657997+01:00"}');
INSERT INTO events VALUES(18,'01a1c157-de52-74ed-b779-b0d23131b1d6',1,'2026-11-09T17:25:23.288Z','metadata','','{"kind":"metadata","time":"2026-11-09T18:25:23.288265092+01:00"}');
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
INSERT INTO audit VALUES(1,'01a1c157-db37-7c22-a3bb-c4eac2d12a28','2026-11-09T17:25:22.869Z',1,1,'127.0.0.1','settings.update',NULL,'','{"body":{"stale_after":"36h"},"bytes":22,"method":"PUT","ms":2,"path":"/api/v1/settings","status":200}','ok',1);
INSERT INTO audit VALUES(2,'01a1c157-db46-7c7e-916a-786cde5bbf93','2026-11-09T17:25:22.885Z',1,1,'127.0.0.1','setpoint',1,'device:0a010a','{"body":{"mode":"manual","temp":21.5},"bytes":32,"method":"PUT","ms":1,"path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","status":200}','ok',2);
INSERT INTO audit VALUES(3,'01a1c157-dc0f-7b32-923d-65733e8bf215','2026-11-09T17:25:22.900Z',1,1,'127.0.0.1','setpoint',1,'room:1','{"body":{"mode":"manual","temp":20},"bytes":30,"method":"PUT","ms":187,"path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","status":200}','ok',4);
INSERT INTO audit VALUES(4,'01a1c157-dcd8-7e75-8e8a-b1751bb6b169','2026-11-09T17:25:23.102Z',1,1,'127.0.0.1','room.rename',1,'room:1','{"body":{"name":"Fixture room"},"bytes":24,"method":"PUT","ms":186,"path":"/api/v1/cubes/KEQ0000100/rooms/1","status":200}','ok',6);
INSERT INTO audit VALUES(5,'01a1c157-dda6-7ddc-b029-9a09cee2f7e6','2026-11-09T17:25:23.303Z',1,1,'127.0.0.1','backup.take',1,'cube:KEQ0000100','{"body":{},"bytes":2,"method":"POST","ms":191,"path":"/api/v1/cubes/KEQ0000100/backups","status":201}','ok',8);
INSERT INTO audit VALUES(6,'01a1c157-de1f-7f42-a59e-abdafaae8f83','2026-11-09T17:25:23.615Z',NULL,NULL,'cli','user.add',NULL,'user:anna','{"password":true,"role":"viewer"}','ok',NULL);
INSERT INTO audit VALUES(7,'01a1c157-de49-738c-8a22-f090b3a91946','2026-11-09T17:25:23.657Z',NULL,NULL,'cli','token.add',NULL,'token:2','{"label":"fixture","user":"anna"}','ok',NULL);
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
INSERT INTO commands VALUES(1,'01a1c157-db35-7951-91a4-cdd11c4b9cb2',NULL,NULL,1,1,'','settings.update','','{"hash":"a1a56e0ab8f245f264cab562d3430faa07b670437af7d261b6497c8824be7018","method":"PUT","path":"/api/v1/settings","query":""}','acknowledged',200,replace('{\n  "auth.password_login": {\n    "value": "true",\n    "default": "true",\n    "set": false,\n    "in_effect": "true",\n    "applies": "now"\n  },\n  "backups.auto_max": {\n    "value": "120",\n    "default": "120",\n    "set": false,\n    "in_effect": "120",\n    "applies": "now"\n  },\n  "backups.auto_min": {\n    "value": "60",\n    "default": "60",\n    "set": false,\n    "in_effect": "60",\n    "applies": "now"\n  },\n  "backups.keep_days_auto": {\n    "value": "90",\n    "default": "90",\n    "set": false,\n    "in_effect": "90",\n    "applies": "now"\n  },\n  "backups.keep_days_manual": {\n    "value": "365",\n    "default": "365",\n    "set": false,\n    "in_effect": "365",\n    "applies": "now"\n  },\n  "backups.keep_newest": {\n    "value": "10",\n    "default": "10",\n    "set": false,\n    "in_effect": "10",\n    "applies": "now"\n  },\n  "commands.keep_days": {\n    "value": "0",\n    "default": "0",\n    "set": false,\n    "in_effect": "0",\n    "applies": "now"\n  },\n  "discover_interval": {\n    "value": "30m",\n    "default": "30m",\n    "set": false,\n    "in_effect": "30m",\n    "applies": "restart"\n  },\n  "log.keep_days": {\n    "value": "30",\n    "default": "30",\n    "set": false,\n    "in_effect": "30",\n    "applies": "now"\n  },\n  "node.id": {\n    "value": "01a1c157-da4c-7306-9b14-5e8dbe986fbe",\n    "set": true,\n    "read_only": true\n  },\n  "poll_interval": {\n    "value": "1m",\n    "default": "1m",\n    "set": false,\n    "in_effect": "1m",\n    "applies": "restart"\n  },\n  "refresh_interval": {\n    "value": "6h",\n    "default": "6h",\n    "set": false,\n    "in_effect": "6h",\n    "applies": "restart"\n  },\n  "stale_after": {\n    "value": "36h",\n    "default": "24h",\n    "set": true,\n    "in_effect": "36h",\n    "applies": "now"\n  },\n  "tenant.id": {\n    "value": "01a1c157-da4c-730f-b613-f79038ea987a",\n    "set": true,\n    "read_only": true\n  }\n}\n','\n',char(10)),'2026-11-09T17:25:22.869Z','2026-11-09T17:25:22.872Z','2026-11-09T17:25:22.872Z',2,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO commands VALUES(2,'01a1c157-db45-758c-a6d4-930d6cadbe0e',NULL,1,1,1,'','setpoint','device:0a010a','{"hash":"a4003acf94e9c73f51fc3c487de1a17dcdead8493e4d06e030efbfed5cf380e4","method":"PUT","path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-09T18:25:22.886212795+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:22.885Z','2026-11-09T17:25:22.886Z','2026-11-09T17:25:22.886Z',3,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO commands VALUES(3,'01a1c157-db46-729f-b3f2-067e08d5ebbd',2,1,1,1,'','step','device:0a010a','{"key":"setpoint:device:0a010a","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:22.886212795+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:22.886Z','2026-11-09T17:25:22.886Z','2026-11-09T17:25:22.886Z',3,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO commands VALUES(4,'01a1c157-db54-761f-9dce-7332d721ce8e',NULL,1,1,1,'','setpoint','room:1','{"hash":"a668a611353079d05040a7b7117ab0201078669851996d42d0d1d7f761b42b96","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-09T18:25:23.087089394+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:22.900Z','2026-11-09T17:25:23.087Z','2026-11-09T17:25:23.087Z',3,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO commands VALUES(5,'01a1c157-db54-7e62-ac74-511d6f4ea22b',4,1,1,1,'','step','','{"key":"setpoint:room:1","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:23.087089394+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:22.900Z','2026-11-09T17:25:23.087Z','2026-11-09T17:25:23.087Z',3,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO commands VALUES(6,'01a1c157-dc1e-744a-944d-d0092eae0292',NULL,1,1,1,'','room.rename','room:1','{"hash":"0d3ae207663d7e2209c687cfb34abbb665b1c5e3cdee82c501293f945e264aeb","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1","query":""}','acknowledged',200,replace('{\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:23.102Z','2026-11-09T17:25:23.289Z','2026-11-09T17:25:23.289Z',3,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO commands VALUES(7,'01a1c157-dc1f-713c-b226-d3ab905814ea',6,1,1,1,'','step','','{"key":"metadata","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:23.087089394+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:23.103Z','2026-11-09T17:25:23.288Z','2026-11-09T17:25:23.288Z',3,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO commands VALUES(8,'01a1c157-dce8-7007-a364-88ac05170587',NULL,1,1,1,'','backup.take','cube:KEQ0000100','{"hash":"100f64b5013f924491098335cbb093208b009a8b0461887abc12f9c6cdb591d1","method":"POST","path":"/api/v1/cubes/KEQ0000100/backups","query":""}','acknowledged',201,replace('{\n  "id": 1,\n  "taken_at": "2026-11-09T17:25:23.493Z",\n  "reason": "manual",\n  "format": "json",\n  "tool": "massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)",\n  "size": 25694,\n  "sha256": "11e7d8ae607dba0614100907232247a09dd7cb67fef7f756b976c1723077b5be",\n  "user": "owner"\n}\n','\n',char(10)),'2026-11-09T17:25:23.303Z','2026-11-09T17:25:23.495Z','2026-11-09T17:25:23.495Z',3,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
INSERT INTO commands VALUES(9,'01a1c157-dce8-7b01-8268-5dbabeb155b6',8,1,1,1,'','step','','{"key":"","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:23.087089394+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:23.304Z','2026-11-09T17:25:23.489Z','2026-11-09T17:25:23.489Z',3,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
CREATE TABLE IF NOT EXISTS "backups" (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    cube_id    INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    taken_at   TEXT    NOT NULL,
    reason     TEXT    NOT NULL CHECK (reason IN ('auto', 'manual', 'before_restore', 'upload')),
    format     TEXT    NOT NULL CHECK (format IN ('json', 'yaml')),
    tool       TEXT    NOT NULL DEFAULT '',      -- backup.File.Tool, the version that took it
    size       INTEGER NOT NULL,                 -- bytes before compression
    sha256     TEXT    NOT NULL,                 -- of the file before compression
    body       BLOB    NOT NULL,                 -- the backup file, gzip; empty in a tombstone
    user_id    INTEGER REFERENCES users (id),    -- who asked, for manual, before_restore and upload
    note       TEXT    NOT NULL DEFAULT '',
    version    INTEGER NOT NULL DEFAULT 1,
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT ''
);
INSERT INTO backups VALUES(1,'01a1c157-dda5-739f-8029-05659add280e',1,'2026-11-09T17:25:23.493Z','manual','json','massimo v0.11.0-alpha.1 (go1.26.8, linux/amd64)',25694,'11e7d8ae607dba0614100907232247a09dd7cb67fef7f756b976c1723077b5be',X'1f8b08000000000000ffec5c5d73a2c81abecfafa0b83aa78e63ba411db56a2e1a05c7249a01854cb2b5652120b002edf22126a7e69fedddfeb153881f28cacc183d583b789374bfcdfbbe3c4fd3f07437fcf78620c8b9e67a2676c826014b51d997a79a33927db2499014a06a1f20fc001a43586f52d52645ff07c0260064dc14632b6a66cb9e67da989883328465f041b666865c86c4bf740ccb54ad5c2f1196e9048b5bd9566b957fc7072bc158239b44940341909ee69af2d2d93dcb0300005c052108d29d8c64557535cf8bec4006107c5cdb26a66b87b21b792261199669f28620be2d0338fe8c6c12bfc5ed1c7f5636b0add9b26f2a6505db6b0769c30d41fcbec970a4606762eadb4467d8f5656ba439f2d8d254b2494c64cbd34a3bc6c05d9e89e1fbb3e6edad2d2fcada9f1fe8b2aa35ebe07679deebf681678cc681ef63671444d98203061587113bb5eacae6bf8d42d3f135779354741ab2bdc4a0d31bae9c13046963c7372262c1a62ad4b4a92abf6e43110469e020f2456f2af064e269fec8d314eca811e86069f9b64dc00b6cfb7002cce04002f48fc4a78ec7a76b6095c2865d1763dbdbf0bb49c354d71d79272bce5cf881ab11d151dbf40e742c994c9eeaae5b2ae596f9fb2f1767fa6b64f8a353feeefffe4b31b44c87f50c8795b443d351092ad3e138c361f5b04398e950c970584b391c2886254fdecc657fca72ab929b1e105f9eaa363715ed401f3888dac6b5ff3a5b067665d5947dec8e7c43736decf9b2bf6db41d8e7aebe1882653b90b2b17c470e362071b8cedd11ed389f10a6ec148a4b0bda4a21108db13ecfa235fb3a3d181daf44082203505afeb6135516fcb8b91a7f9336c3a91378a4eda4c2769ab94930746ce46f1a597bc3ca38bd674541c8ef04c73d621ab47ccb6e904fe92949da4c6187b7ec298b6cd656bae8d664a14bb9e0cae6a8a6c29a378d0203dd90fdce8ff748bd53002a93d30929e2148ba8e2dabd1266eb073da9a361dcd5cacbbb2bdc34b3cb2c519fd96a82576da6c408de27e2ced5b02c737973d0cd49a342077ccdf4a3fe49582195e3f36e9ea495eb37285f5e8e67fee5c61a309ce9f2b5549e79a28fd9e3c92f403cd2b083d57ae574068a8a94e41e9f972bd024a7d23700b46cf96eb15303a71cd82cf73e57a057c6e9ececec2e8c74ba0041a05a33fc36870bee7dc82cf5cf8bcd9affdb695b9beec6bbb92d3c6aa16cfe939816c2565565254821de5282b7e205b1b35da28d753222b5657d48eee937d5f735f47160e77a7d19656cd75b17ba0de329de968c778933cbbfd5987835332ef9e0ba032e602f6e784369300543109f08b4f025099034ee5c40107640c0d54139ee6352bd7cb0aeb5f11a413c4eaaf08d3cf0bc05f11a59f1555197141edd4c7a0ed7ded80d7c6c90f57d9c8533923fff3f2a7c0fe6cd8ffa45029907f07f237fbb53f2429e4c0c74705c551394151e5ca113941832b9013f2fbe504cc90134fd87052aba31b4d01f3d314f4ae062c54c575aa8a134787ac690c0a34e105a631f27f723ab35629a0cf510115e0e7a7ab0aec73516b05f2392e565d64f911c29351bac605a07f104637fbb5ef5540b0715401c17af9e3110554b90605347ebf02aa6428a0d41ed68dfaa9e4a77e8a1595abd03ed9a3cd65969b4f9d1bca5a6ebec8d238455f22d73cb7551684e64de8d9b7551694e64de9b9b755168ce6cde879b755167ce6cde7b995ea2550a2f247e9ac4af51f83d1cd7eedfbb7ffc146c6f6bfbdbd813beb75d7b0fd4f79bf5aad7e4fad1e7c09b05aa8d55f5cad5249a857b6edddebf45d07d9f7c44bcc0953276fbace6fadae003fd7d5ba02fe3cd7eb0af4f35bb12bb0bfd235bb93f522ac6778bdc8ab439176285e05fb7590bfd9af3d836aab67bfb4451d516d956b506deafb555b2d43b51df90acd46bbd5cea4dde00e0549f5562be7b1d772472715126ef33b764967ddc94f9fdda133c6097889919282a7eec4c800e0a2f2ad003e37e956409f976c2b90cf47b215b8e725d70ae4f3116b05eeff6fa996f5321c3c2ad32858a68fc8342af7ada0704c1d585cf38c20ca65a460c797952c89060ebd08c7698ee76b3fb6ae9601f75c5664dfc40e7958ef904dc27703ede2086e3e5e6a994ee2d3a5e4e7e6f6b3c7a5f80bc72500215d02ab5f0956940905805202a0445325288331a04b90a6e912889bad3f2adc6b463e6049d2bb1dc42f3af25703a85fefaceee7bbf99802f788c76d663a6fcf3b7dfcc263a685ba1cef3dd615bb7e8ff829c74cfdd9d8e6f51e8f9936e23b03ca0a5e5ae8b5a5b30f8cbe183e5386f7fcf452939fa0fff2b57b8f78f019457e44c8894314f6da68d16b771f459b9bca4f1c18d35d7de5bfb56cc726dbf5a4643ba963588a0de70a2d186a0b2d50c8de23deef0b121bb50f7b4304daf0ce78e95886da69bc76d955ae3acfb474b637142561d9aecd867dbe31787e1266cf5f85b962320ff25375da6db35c94437b27d7e79d1c44aa8fc71dced66ccb1f3f49af8c1ee5304be6b060933970ea5cee54d778300cc2e387ed3946b93c0ab61428b460ede6f1e9d39ab3567345fa471e334c888668d27848040c11f210c3a2edefcbedf15fb8db969f64b4a50ca07e46b587d786fffc550f5ebeb27eefb53a7d19e24aafd5f8437dbab3501bcdc79dc65ca6a510e5f963d5be84106ae9a81d97ef86511985a885107a64510a50b50474ccb479b6c3de334906a9fb57b6d1e29567f4867ab72112a7ed617dc077a52e23eadc00495d56d48501c36dfe3203beff6571878c959dd9b3b3a2cedd2170cb4e5b6b3bb76717065714270d567d09560bb1ed3db0de1e984ad0e295fb182c869538e14de81b62554c2717ff1544e9739fa5b8a175d82eea025b159fc0f0597a940edb070c3bb484f177e23c4a9c7ff93869b01a31583cdbda03eb75172c140e58c613f970d16db37a6f801629e76da6cb7716427b6d6fb3a9247a8f8cd0fa636befa5190f2b1c7bbfb51f8ad3d6451e6fed27c6f9eef9a4c11ac760852cbb0716d8ef59034eec1e653266bc2bf146863d62dc10c7ecb02709c251c6079ce009bd85c465c5e92cd8374e142f1a270d961c83a5b3cc1e588b07a6d2488235b4069def80f528755f32c1eaf517621d88f7598cd3dcf04d48f69c74cf52a42f7d2ed9732e12270d96120ff088e5f6c082a9318b357489b7b2c012071d466a1d1d6b44bde7312ce0c42f1927f11ab2435ee2eb19603d20a1cf48edb70cb0ce112701d643b31546b7c1e9a0c3f07fa2be88a2720b4d071d613a43bde7b8bcb457307a4471399c0e3a839e837a202eebd341a7a5dbe8c15b96dbd1f10360a11e428c8ec70f5159fff429d24abfdf7cfb5f000000ffff8e4f40485e640000',1,'',1,NULL,'01a1c157-da4c-7306-9b14-5e8dbe986fbe');
CREATE INDEX tokens_user ON tokens (user_id);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX audit_time ON audit (time);
CREATE INDEX audit_user ON audit (user_id, time);
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');
CREATE INDEX commands_key ON commands (user_id, idempotency_key);
CREATE INDEX events_cube ON events (cube_id);
CREATE INDEX events_cube_kind ON events (cube_id, kind);
CREATE INDEX events_cube_device ON events (cube_id, device);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);
COMMIT;
