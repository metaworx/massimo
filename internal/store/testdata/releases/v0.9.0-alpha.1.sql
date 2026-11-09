-- The store that massimo v0.9.0-alpha.1 (f8f4d34) wrote against the fake Cube,
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
INSERT INTO schema_version VALUES(1,1,'2026-11-09T17:25:08.139Z','massimo v0.9.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(2,1,'2026-11-09T17:25:08.149Z','massimo v0.9.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(3,1,'2026-11-09T17:25:08.149Z','massimo v0.9.0-alpha.1 (go1.26.8, linux/amd64)');
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
INSERT INTO roles VALUES(1,'owner','',1,0,'2026-11-09T17:25:08.138Z','2026-11-09T17:25:08.138Z',NULL,NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO roles VALUES(2,'manager','',1,0,'2026-11-09T17:25:08.138Z','2026-11-09T17:25:08.138Z',NULL,NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO roles VALUES(3,'operator','',1,0,'2026-11-09T17:25:08.138Z','2026-11-09T17:25:08.138Z',NULL,NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO roles VALUES(4,'guest','',1,0,'2026-11-09T17:25:08.138Z','2026-11-09T17:25:08.138Z',NULL,NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO roles VALUES(5,'viewer','',1,0,'2026-11-09T17:25:08.138Z','2026-11-09T17:25:08.138Z',NULL,NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO roles VALUES(6,'integration','',1,0,'2026-11-09T17:25:08.138Z','2026-11-09T17:25:08.138Z',NULL,NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
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
INSERT INTO users VALUES(1,'01a1c157-a1b8-7bf9-954d-57012d5f81bf','owner','','',1,0,NULL,NULL,'2026-11-09T17:25:08.152Z','2026-11-09T17:25:08.152Z',NULL,NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75',1);
INSERT INTO users VALUES(2,'01a1c157-a557-7ccb-a577-cbda10d3f364','anna','','$argon2id$v=19$m=65536,t=1,p=4$7xu5Bt6R/MiiqZf2JqezLQ$tdJ5TWIPdLsUj3rFvZb/8z5bAEX1WkV/C2MLFmCc8Kk',5,0,NULL,NULL,'2026-11-09T17:25:09.079Z','2026-11-09T17:25:09.079Z',NULL,NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75',1);
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
INSERT INTO tokens VALUES(1,'01a1c157-a1b9-70e9-b3f2-64e24cc74990',1,'api','3dbd5e608845edf5f2de485da50c0a11655a49b1b5a2188168036453ab3f1b64','MASSIMO_API_TOKEN','2026-11-09T17:25:08.153Z',NULL,'2026-11-09T17:25:08.339Z','127.0.0.1',NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO tokens VALUES(2,'01a1c157-a58a-7d3d-9c2b-14705991b68b',2,'api','ee0cd7e44e229113de868ec125ad5d1b11ba9aaf73753f480fb95af7066a4fc7','fixture','2026-11-09T17:25:09.130Z',NULL,'2026-11-09T17:25:09.150Z','127.0.0.1',NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
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
INSERT INTO settings VALUES('node.id','"01a1c157-a1ab-73bf-befb-f85e40080e75"','2026-11-09T17:25:08.139Z',NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO settings VALUES('tenant.id','"01a1c157-a1ab-73cc-a0f5-b3ac942b58e2"','2026-11-09T17:25:08.139Z',NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO settings VALUES('stale_after','"36h"','2026-11-09T17:25:08.377Z',NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
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
INSERT INTO cubes VALUES(1,'01a1c157-a1b7-7c7c-b096-02e34b9d9703','KEQ0000100',1,'Fake Cube','127.0.0.1:35841',0,'','','[]','env','','','','',NULL,NULL,'2026-11-09T17:25:08.151Z','2026-11-09T17:25:08.151Z',NULL,1,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
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
INSERT INTO audit VALUES(1,'01a1c157-a299-7d4f-991d-1145fcee0d5a','2026-11-09T17:25:08.375Z',1,1,'127.0.0.1','settings.update',NULL,'','{"body":{"stale_after":"36h"},"bytes":22,"method":"PUT","ms":2,"path":"/api/v1/settings","status":200}','ok',1);
INSERT INTO audit VALUES(2,'01a1c157-a2aa-7973-8a53-cb293623e52d','2026-11-09T17:25:08.390Z',1,1,'127.0.0.1','setpoint',1,'device:0a010a','{"body":{"mode":"manual","temp":21.5},"bytes":32,"method":"PUT","ms":4,"path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","status":200}','ok',2);
INSERT INTO audit VALUES(3,'01a1c157-a374-7c1b-8e92-6de2cd88c3b8','2026-11-09T17:25:08.408Z',1,1,'127.0.0.1','setpoint',1,'room:1','{"body":{"mode":"manual","temp":20},"bytes":30,"method":"PUT","ms":188,"path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","status":200}','ok',4);
INSERT INTO audit VALUES(4,'01a1c157-a43e-7bb2-8fb2-16e743a9f441','2026-11-09T17:25:08.611Z',1,1,'127.0.0.1','room.rename',1,'room:1','{"body":{"name":"Fixture room"},"bytes":24,"method":"PUT","ms":186,"path":"/api/v1/cubes/KEQ0000100/rooms/1","status":200}','ok',6);
INSERT INTO audit VALUES(5,'01a1c157-a558-73e6-a54d-7fc3404425be','2026-11-09T17:25:09.080Z',NULL,NULL,'cli','user.add',NULL,'user:anna','{"password":true,"role":"viewer"}','ok',NULL);
INSERT INTO audit VALUES(6,'01a1c157-a58b-71df-af25-db71480155c5','2026-11-09T17:25:09.131Z',NULL,NULL,'cli','token.add',NULL,'token:2','{"label":"fixture","user":"anna"}','ok',NULL);
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
INSERT INTO commands VALUES(1,'01a1c157-a298-7e71-9086-f02ea7d5b5fd',NULL,NULL,1,1,'','settings.update','','{"hash":"a1a56e0ab8f245f264cab562d3430faa07b670437af7d261b6497c8824be7018","method":"PUT","path":"/api/v1/settings","query":""}','acknowledged',200,replace('{\n  "commands.keep_days": {\n    "value": "0",\n    "default": "0",\n    "set": false,\n    "in_effect": "0",\n    "applies": "now"\n  },\n  "discover_interval": {\n    "value": "30m",\n    "default": "30m",\n    "set": false,\n    "in_effect": "30m",\n    "applies": "restart"\n  },\n  "node.id": {\n    "value": "01a1c157-a1ab-73bf-befb-f85e40080e75",\n    "set": true,\n    "read_only": true\n  },\n  "poll_interval": {\n    "value": "1m",\n    "default": "1m",\n    "set": false,\n    "in_effect": "1m",\n    "applies": "restart"\n  },\n  "refresh_interval": {\n    "value": "6h",\n    "default": "6h",\n    "set": false,\n    "in_effect": "6h",\n    "applies": "restart"\n  },\n  "stale_after": {\n    "value": "36h",\n    "default": "24h",\n    "set": true,\n    "in_effect": "36h",\n    "applies": "now"\n  },\n  "tenant.id": {\n    "value": "01a1c157-a1ab-73cc-a0f5-b3ac942b58e2",\n    "set": true,\n    "read_only": true\n  }\n}\n','\n',char(10)),'2026-11-09T17:25:08.376Z','2026-11-09T17:25:08.378Z','2026-11-09T17:25:08.378Z',2,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO commands VALUES(2,'01a1c157-a2a7-7955-a6ae-7cb078f7ce47',NULL,1,1,1,'','setpoint','device:0a010a','{"hash":"a4003acf94e9c73f51fc3c487de1a17dcdead8493e4d06e030efbfed5cf380e4","method":"PUT","path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-09T18:25:08.393809045+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:08.391Z','2026-11-09T17:25:08.394Z','2026-11-09T17:25:08.394Z',3,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO commands VALUES(3,'01a1c157-a2a8-77f0-8ed2-b8ef96f79d14',2,1,1,1,'','step','device:0a010a','{"key":"setpoint:device:0a010a","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:08.393809045+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:08.392Z','2026-11-09T17:25:08.394Z','2026-11-09T17:25:08.394Z',3,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO commands VALUES(4,'01a1c157-a2b9-7bab-a457-c3ad8dcef6a1',NULL,1,1,1,'','setpoint','room:1','{"hash":"a668a611353079d05040a7b7117ab0201078669851996d42d0d1d7f761b42b96","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-09T18:25:08.596387642+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:08.409Z','2026-11-09T17:25:08.596Z','2026-11-09T17:25:08.596Z',3,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO commands VALUES(5,'01a1c157-a2ba-780c-9454-209acdf655e6',4,1,1,1,'','step','','{"key":"setpoint:room:1","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:08.596387642+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:08.410Z','2026-11-09T17:25:08.596Z','2026-11-09T17:25:08.596Z',3,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO commands VALUES(6,'01a1c157-a387-767a-b86d-223dafd6052d',NULL,1,1,1,'','room.rename','room:1','{"hash":"0d3ae207663d7e2209c687cfb34abbb665b1c5e3cdee82c501293f945e264aeb","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1","query":""}','acknowledged',200,replace('{\n  "ok": true\n}\n','\n',char(10)),'2026-11-09T17:25:08.615Z','2026-11-09T17:25:08.798Z','2026-11-09T17:25:08.798Z',3,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
INSERT INTO commands VALUES(7,'01a1c157-a387-7e52-aa87-b24fb71576c6',6,1,1,1,'','step','','{"key":"metadata","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-09T18:25:08.596387642+01:00","max_memory_slots":50},"discarded":false}','2026-11-09T17:25:08.615Z','2026-11-09T17:25:08.798Z','2026-11-09T17:25:08.798Z',3,NULL,'01a1c157-a1ab-73bf-befb-f85e40080e75');
CREATE INDEX tokens_user ON tokens (user_id);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX events_device_time ON events (cube_id, device, time);
CREATE INDEX audit_time ON audit (time);
CREATE INDEX audit_user ON audit (user_id, time);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');
CREATE INDEX commands_key ON commands (user_id, idempotency_key);
COMMIT;
