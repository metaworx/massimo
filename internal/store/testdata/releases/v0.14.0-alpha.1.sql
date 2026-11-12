-- The store that massimo v0.14.0-alpha.1 (ab5dfdf) wrote against the fake Cube,
-- made by tools/store-fixture/make.sh on 2026-11-11: a text dump
-- (sqlite3 .dump) that TestMigratesReleasedStores loads and migrates.
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE schema_version (
    version    INTEGER NOT NULL,   -- the migration applied
    min_reader INTEGER NOT NULL,   -- the oldest schema version a binary may be built for and still open this file
    applied_at TEXT    NOT NULL,
    applied_by TEXT    NOT NULL    -- the massimo version string
);
INSERT INTO schema_version VALUES(1,1,'2026-11-11T16:31:00.084Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(2,1,'2026-11-11T16:31:00.092Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(3,1,'2026-11-11T16:31:00.093Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(4,1,'2026-11-11T16:31:00.094Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(5,1,'2026-11-11T16:31:00.096Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(6,1,'2026-11-11T16:31:00.097Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(7,1,'2026-11-11T16:31:00.098Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(8,1,'2026-11-11T16:31:00.098Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(9,1,'2026-11-11T16:31:00.100Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(10,1,'2026-11-11T16:31:00.101Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(11,1,'2026-11-11T16:31:00.102Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(12,1,'2026-11-11T16:31:00.102Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(13,1,'2026-11-11T16:31:00.106Z','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)');
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
INSERT INTO roles VALUES(1,'owner','',1,0,'2026-11-11T16:31:00.084Z','2026-11-11T16:31:00.084Z',NULL,NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO roles VALUES(2,'manager','',1,0,'2026-11-11T16:31:00.084Z','2026-11-11T16:31:00.084Z',NULL,NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO roles VALUES(3,'operator','',1,0,'2026-11-11T16:31:00.084Z','2026-11-11T16:31:00.084Z',NULL,NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO roles VALUES(4,'guest','',1,0,'2026-11-11T16:31:00.084Z','2026-11-11T16:31:00.084Z',NULL,NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO roles VALUES(5,'viewer','',1,0,'2026-11-11T16:31:00.084Z','2026-11-11T16:31:00.084Z',NULL,NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO roles VALUES(6,'integration','',1,0,'2026-11-11T16:31:00.084Z','2026-11-11T16:31:00.084Z',NULL,NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
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
INSERT INTO role_capabilities VALUES(2,'passwords');
INSERT INTO role_capabilities VALUES(6,'configure');
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
INSERT INTO users VALUES(1,'01a1cb72-ca0f-768c-a40d-1335ff6aa101','owner','','',1,0,NULL,NULL,'2026-11-11T16:31:00.111Z','2026-11-11T16:31:00.111Z',NULL,NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b',1,NULL,'');
INSERT INTO users VALUES(2,'01a1cb72-cdb4-7be3-9fbf-35922304c8fe','anna','','',5,0,NULL,NULL,'2026-11-11T16:31:01.044Z','2026-11-11T16:31:01.044Z',NULL,NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b',2,X'241070cc1475f2de336363d0be290365f66c3635aa4cc961738b1197d07fa76f957d602de95f5bb037fbc31bf6a24c7343a05734ee63f0ce151306e4203af0f4b82060b43d296474557a0d63c14eabc4a02ba16fd0e7104684880d700bd73199b47dbfb5fc0ddda0a7a6fdace4b317cdb5868120e62aa612a531f62f9778c3965b7d12af1e5b6f2ac1668c73fa2c9eef4a737f7547ab0ce6df72a478726816195c24bf0b235ecd1e50c15b4a887812be26b82111b4f5d2f9e5b6b4c02bb2806a','m=19456,t=2,p=1');
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
INSERT INTO tokens VALUES(1,'01a1cb72-ca0f-7d78-93c3-4a9dc208e861',1,'api','3dbd5e608845edf5f2de485da50c0a11655a49b1b5a2188168036453ab3f1b64','MASSIMO_API_TOKEN','2026-11-11T16:31:00.111Z',NULL,'2026-11-11T16:31:00.281Z','127.0.0.1',NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO tokens VALUES(2,'01a1cb72-cdcf-7d83-8df3-f291efb9c45f',2,'api','c6655dda1937c0eaec2998d156be6ec736d7729dcdaf2e93490d419752a2ec44','fixture','2026-11-11T16:31:01.071Z',NULL,'2026-11-11T16:31:01.102Z','127.0.0.1',NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
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
INSERT INTO settings VALUES('node.id','"01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b"','2026-11-11T16:31:00.084Z',NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO settings VALUES('tenant.id','"01a1cb72-c9f4-7832-9890-c86010d22b97"','2026-11-11T16:31:00.084Z',NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO settings VALUES('log.server_version','"v0.14.0-alpha.1"','2026-11-11T16:31:00.117Z',NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO settings VALUES('stale_after','"36h"','2026-11-11T16:31:00.321Z',NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
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
INSERT INTO cubes VALUES(1,'01a1cb72-ca0e-78a6-8b16-a6d7a6aca086','KEQ0000100',1,'Fake Cube','127.0.0.1:47141',0,'','','[]','env','','','','',NULL,NULL,'2026-11-11T16:31:00.110Z','2026-11-11T16:31:00.110Z',NULL,1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
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
INSERT INTO audit VALUES(1,'01a1cb72-cae2-74f7-9c3f-5779a0da09de','2026-11-11T16:31:00.320Z',1,1,'127.0.0.1','settings.update',NULL,'','{"body":{"stale_after":"36h"},"bytes":22,"method":"PUT","ms":1,"path":"/api/v1/settings","status":200}','ok',1);
INSERT INTO audit VALUES(2,'01a1cb72-caf8-73ab-a668-8cdec14bb11d','2026-11-11T16:31:00.341Z',1,1,'127.0.0.1','setpoint',1,'device:MEQ0000101','{"body":{"mode":"manual","temp":21.5},"bytes":32,"method":"PUT","ms":1,"path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","status":200}','ok',2);
INSERT INTO audit VALUES(3,'01a1cb72-cbc0-75b0-a848-e62c968ad9f1','2026-11-11T16:31:00.363Z',1,1,'127.0.0.1','setpoint',1,'room:1','{"body":{"mode":"manual","temp":20},"bytes":30,"method":"PUT","ms":180,"path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","status":200}','ok',4);
INSERT INTO audit VALUES(4,'01a1cb72-cc89-7661-9a50-15452b0b6609','2026-11-11T16:31:00.557Z',1,1,'127.0.0.1','room.rename',1,'room:1','{"body":{"name":"Fixture room"},"bytes":24,"method":"PUT","ms":187,"path":"/api/v1/cubes/KEQ0000100/rooms/1","status":200}','ok',6);
INSERT INTO audit VALUES(5,'01a1cb72-cd59-793a-89a4-d8d76bc6a735','2026-11-11T16:31:00.757Z',1,1,'127.0.0.1','backup.take',1,'cube:KEQ0000100','{"body":{},"bytes":2,"method":"POST","ms":196,"path":"/api/v1/cubes/KEQ0000100/backups","status":201}','ok',8);
INSERT INTO audit VALUES(6,'01a1cb72-cdb5-7842-8bed-646a0bec6293','2026-11-11T16:31:01.045Z',NULL,NULL,'cli','user.add',NULL,'user:anna','{"password":true,"role":"viewer"}','ok',NULL);
INSERT INTO audit VALUES(7,'01a1cb72-cdd4-7730-b31a-2c2afa8f3a6f','2026-11-11T16:31:01.076Z',NULL,NULL,'cli','token.add',NULL,'token:2','{"label":"fixture","user":"anna"}','ok',NULL);
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
INSERT INTO commands VALUES(1,'01a1cb72-cae0-7a6a-9ce3-8ddddde2c52a',NULL,NULL,1,1,'','settings.update','','{"hash":"a1a56e0ab8f245f264cab562d3430faa07b670437af7d261b6497c8824be7018","method":"PUT","path":"/api/v1/settings","query":""}','acknowledged',200,replace('{\n  "auth.password_login": {\n    "value": "true",\n    "default": "true",\n    "set": false,\n    "in_effect": "true",\n    "applies": "now"\n  },\n  "backups.auto_max": {\n    "value": "120",\n    "default": "120",\n    "set": false,\n    "in_effect": "120",\n    "applies": "now"\n  },\n  "backups.auto_min": {\n    "value": "60",\n    "default": "60",\n    "set": false,\n    "in_effect": "60",\n    "applies": "now"\n  },\n  "backups.keep_days_auto": {\n    "value": "90",\n    "default": "90",\n    "set": false,\n    "in_effect": "90",\n    "applies": "now"\n  },\n  "backups.keep_days_manual": {\n    "value": "365",\n    "default": "365",\n    "set": false,\n    "in_effect": "365",\n    "applies": "now"\n  },\n  "backups.keep_newest": {\n    "value": "10",\n    "default": "10",\n    "set": false,\n    "in_effect": "10",\n    "applies": "now"\n  },\n  "commands.keep_days": {\n    "value": "0",\n    "default": "0",\n    "set": false,\n    "in_effect": "0",\n    "applies": "now"\n  },\n  "discover_interval": {\n    "value": "30m",\n    "default": "30m",\n    "set": false,\n    "in_effect": "30m",\n    "applies": "restart"\n  },\n  "log.keep_days": {\n    "value": "30",\n    "default": "30",\n    "set": false,\n    "in_effect": "30",\n    "applies": "now"\n  },\n  "node.id": {\n    "value": "01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b",\n    "set": true,\n    "read_only": true\n  },\n  "poll_interval": {\n    "value": "1m",\n    "default": "1m",\n    "set": false,\n    "in_effect": "1m",\n    "applies": "restart"\n  },\n  "refresh_interval": {\n    "value": "6h",\n    "default": "6h",\n    "set": false,\n    "in_effect": "6h",\n    "applies": "restart"\n  },\n  "stale_after": {\n    "value": "36h",\n    "default": "24h",\n    "set": true,\n    "in_effect": "36h",\n    "applies": "now"\n  },\n  "tenant.id": {\n    "value": "01a1cb72-c9f4-7832-9890-c86010d22b97",\n    "set": true,\n    "read_only": true\n  },\n  "update.channel": {\n    "value": "alpha",\n    "default": "alpha",\n    "set": false,\n    "in_effect": "alpha",\n    "applies": "now"\n  }\n}\n','\n',char(10)),'2026-11-11T16:31:00.320Z','2026-11-11T16:31:00.322Z','2026-11-11T16:31:00.322Z',2,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO commands VALUES(2,'01a1cb72-caf5-758d-9a6e-df6fb1d6c98d',NULL,1,1,1,'','setpoint','device:MEQ0000101','{"hash":"a4003acf94e9c73f51fc3c487de1a17dcdead8493e4d06e030efbfed5cf380e4","method":"PUT","path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-11T17:31:00.342670607+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T16:31:00.341Z','2026-11-11T16:31:00.344Z','2026-11-11T16:31:00.344Z',3,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO commands VALUES(3,'01a1cb72-caf6-7ae0-82be-6c0473e26a68',2,1,1,1,'','step','device:0a010a','{"key":"setpoint:device:0a010a","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T17:31:00.342670607+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T16:31:00.342Z','2026-11-11T16:31:00.343Z','2026-11-11T16:31:00.343Z',3,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO commands VALUES(4,'01a1cb72-cb0b-73b5-a7f4-154fd4311f4a',NULL,1,1,1,'','setpoint','room:1','{"hash":"a668a611353079d05040a7b7117ab0201078669851996d42d0d1d7f761b42b96","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-11T17:31:00.543563593+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T16:31:00.363Z','2026-11-11T16:31:00.544Z','2026-11-11T16:31:00.544Z',3,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO commands VALUES(5,'01a1cb72-cb0c-738a-b429-ac69caedd4cf',4,1,1,1,'','step','','{"key":"setpoint:room:1","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T17:31:00.543563593+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T16:31:00.364Z','2026-11-11T16:31:00.544Z','2026-11-11T16:31:00.544Z',3,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO commands VALUES(6,'01a1cb72-cbce-7154-8b07-0391f25353b8',NULL,1,1,1,'','room.rename','room:1','{"hash":"0d3ae207663d7e2209c687cfb34abbb665b1c5e3cdee82c501293f945e264aeb","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1","query":""}','acknowledged',200,replace('{\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T16:31:00.558Z','2026-11-11T16:31:00.745Z','2026-11-11T16:31:00.745Z',3,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO commands VALUES(7,'01a1cb72-cbcf-70b4-a291-8a026539f4e2',6,1,1,1,'','step','','{"key":"metadata","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T17:31:00.543563593+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T16:31:00.559Z','2026-11-11T16:31:00.745Z','2026-11-11T16:31:00.745Z',3,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO commands VALUES(8,'01a1cb72-cc95-77c4-8ce9-ef6044a74156',NULL,1,1,1,'','backup.take','cube:KEQ0000100','{"hash":"100f64b5013f924491098335cbb093208b009a8b0461887abc12f9c6cdb591d1","method":"POST","path":"/api/v1/cubes/KEQ0000100/backups","query":""}','acknowledged',201,replace('{\n  "id": 1,\n  "taken_at": "2026-11-11T16:31:00.951Z",\n  "reason": "manual",\n  "format": "json",\n  "tool": "massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)",\n  "size": 25694,\n  "sha256": "ef50a177395fb0e2bc619c06d24bb2beac45d70c34423a71392151a94bb181c6",\n  "user": "owner"\n}\n','\n',char(10)),'2026-11-11T16:31:00.757Z','2026-11-11T16:31:00.953Z','2026-11-11T16:31:00.953Z',3,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
INSERT INTO commands VALUES(9,'01a1cb72-cc96-7523-83e5-8d25d7fbb70b',8,1,1,1,'','step','','{"key":"","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T17:31:00.543563593+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T16:31:00.758Z','2026-11-11T16:31:00.947Z','2026-11-11T16:31:00.947Z',3,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
CREATE TABLE IF NOT EXISTS "events" (
    id        INTEGER PRIMARY KEY,
    uuid      TEXT    NOT NULL UNIQUE,
    cube_id   INTEGER REFERENCES cubes (id) ON DELETE CASCADE, -- NULL: the server's
    time      TEXT    NOT NULL,
    kind      TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    source    TEXT    NOT NULL DEFAULT '',       -- device, cube, system, config; '' a row of a binary before
    level     INTEGER NOT NULL DEFAULT 0,        -- slog's: -4 debug, 0 info, 4 warn, 8 error
    device    TEXT    NOT NULL DEFAULT '',       -- radio address, '' when none
    serial    TEXT    NOT NULL DEFAULT '',       -- the device's serial number, or the Cube's for its own lines
    user_id   INTEGER REFERENCES users (id),     -- who started it; NULL: the Cube, the server's own jobs, a server without users
    data      TEXT    NOT NULL                   -- JSON: the event object as GET …/log returns it
);
INSERT INTO events VALUES(1,'01a1cb72-cdb6-7250-9744-4a3c838a6717',NULL,'2026-11-11T16:31:01.045Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"user.add","set":{"password":true,"role":"viewer"},"subject":{"display_name":"","id":2,"name":"anna","role":"viewer","uuid":"01a1cb72-cdb4-7be3-9fbf-35922304c8fe"},"target":"user:anna","via":"cli"},"time":"2026-11-11T17:31:01.045925758+01:00"}');
INSERT INTO events VALUES(2,'01a1cb72-cdd5-7aaa-88f2-82b4470ddef4',NULL,'2026-11-11T16:31:01.077Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"token.add","set":{"label":"fixture","user":"anna"},"target":"token:2","via":"cli"},"time":"2026-11-11T17:31:01.077124956+01:00"}');
INSERT INTO events VALUES(3,'01a1cb72-ce03-7d26-903b-384fc9233a46',NULL,'2026-11-11T16:31:00.117Z','server_started','system',0,'','',NULL,'{"kind":"server_started","source":"system","level":"info","summary":{"version":"v0.14.0-alpha.1"},"time":"2026-11-11T17:31:00.117952322+01:00"}');
INSERT INTO events VALUES(4,'01a1cb72-ce04-7a84-b418-a6708a0f893c',1,'2026-11-11T16:31:00.120Z','hello','cube',-4,'','',NULL,'{"kind":"hello","source":"cube","level":"debug","summary":{"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50},"time":"2026-11-11T17:31:00.120086122+01:00"}');
INSERT INTO events VALUES(5,'01a1cb72-ce04-7dac-9d33-3f3f15826225',1,'2026-11-11T16:31:00.120Z','metadata','cube',-4,'','',NULL,'{"kind":"metadata","source":"cube","level":"debug","summary":{"devices":7,"rooms":6},"time":"2026-11-11T17:31:00.120179322+01:00"}');
INSERT INTO events VALUES(6,'01a1cb72-ce04-7de1-ac54-b98506489332',1,'2026-11-11T16:31:00.120Z','config','cube',-4,'0a0107','KEQ0000100',NULL,'{"kind":"config","device":"0a0107","serial":"KEQ0000100","source":"cube","level":"debug","summary":{"portal_enabled":false,"tz_summer":"BST","tz_winter":"GMT"},"time":"2026-11-11T17:31:00.120748322+01:00"}');
INSERT INTO events VALUES(7,'01a1cb72-ce04-7e0c-8433-63a525ecd168',1,'2026-11-11T16:31:00.120Z','config','device',-4,'0a010d','MEQ0000106',NULL,'{"kind":"config","device":"0a010d","serial":"MEQ0000106","source":"device","level":"debug","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"summary":{"comfort_temp":21.5,"eco_temp":16.5,"temp_offset":0},"time":"2026-11-11T17:31:00.120790622+01:00"}');
INSERT INTO events VALUES(8,'01a1cb72-ce04-7e7d-abd6-0630d5a8de5d',1,'2026-11-11T16:31:00.121Z','config','device',-4,'0a0108','MEQ0000103',NULL,'{"kind":"config","device":"0a0108","serial":"MEQ0000103","source":"device","level":"debug","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T17:31:00.121221922+01:00"}');
INSERT INTO events VALUES(9,'01a1cb72-ce04-7e9f-8539-d649cf861c33',1,'2026-11-11T16:31:00.122Z','config','device',-4,'0a0109','MEQ0000102',NULL,'{"kind":"config","device":"0a0109","serial":"MEQ0000102","source":"device","level":"debug","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T17:31:00.122274722+01:00"}');
INSERT INTO events VALUES(10,'01a1cb72-ce05-700b-8c87-3a712e26aa8a',1,'2026-11-11T16:31:00.122Z','config','device',-4,'0a010b','MEQ0000104',NULL,'{"kind":"config","device":"0a010b","serial":"MEQ0000104","source":"device","level":"debug","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T17:31:00.122311822+01:00"}');
INSERT INTO events VALUES(11,'01a1cb72-ce05-7096-ab49-5f7cab7e9ff1',1,'2026-11-11T16:31:00.122Z','config','device',-4,'0a010a','MEQ0000101',NULL,'{"kind":"config","device":"0a010a","serial":"MEQ0000101","source":"device","level":"debug","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T17:31:00.122338722+01:00"}');
INSERT INTO events VALUES(12,'01a1cb72-ce05-711c-b6a9-54f8e44c9651',1,'2026-11-11T16:31:00.122Z','config','device',-4,'0a010c','MEQ0000105',NULL,'{"kind":"config","device":"0a010c","serial":"MEQ0000105","source":"device","level":"debug","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T17:31:00.122360222+01:00"}');
INSERT INTO events VALUES(13,'01a1cb72-ce05-7140-97fa-322c0b5cc4dd',1,'2026-11-11T16:31:00.122Z','state','device',0,'0a010d','MEQ0000106',NULL,'{"kind":"state","device":"0a010d","serial":"MEQ0000106","source":"device","level":"info","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"time":"2026-11-11T17:31:00.122377422+01:00"}');
INSERT INTO events VALUES(14,'01a1cb72-ce05-7167-a861-dcb899fe7a85',1,'2026-11-11T16:31:00.122Z','state','device',0,'0a0108','MEQ0000103',NULL,'{"kind":"state","device":"0a0108","serial":"MEQ0000103","source":"device","level":"info","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"time":"2026-11-11T17:31:00.122455222+01:00"}');
INSERT INTO events VALUES(15,'01a1cb72-ce05-71cb-91c4-6b9b0e856543',1,'2026-11-11T16:31:00.122Z','state','device',0,'0a0109','MEQ0000102',NULL,'{"kind":"state","device":"0a0109","serial":"MEQ0000102","source":"device","level":"info","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"time":"2026-11-11T17:31:00.122523122+01:00"}');
INSERT INTO events VALUES(16,'01a1cb72-ce05-7261-82b7-753f425c6651',1,'2026-11-11T16:31:00.122Z','state','device',0,'0a010b','MEQ0000104',NULL,'{"kind":"state","device":"0a010b","serial":"MEQ0000104","source":"device","level":"info","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"time":"2026-11-11T17:31:00.122581322+01:00"}');
INSERT INTO events VALUES(17,'01a1cb72-ce05-728b-9a56-3204007e77ec',1,'2026-11-11T16:31:00.122Z','state','device',0,'0a010a','MEQ0000101',NULL,'{"kind":"state","device":"0a010a","serial":"MEQ0000101","source":"device","level":"info","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"time":"2026-11-11T17:31:00.122620822+01:00"}');
INSERT INTO events VALUES(18,'01a1cb72-ce05-72bc-9bcb-2abb8db5f373',1,'2026-11-11T16:31:00.122Z','state','device',0,'0a010c','MEQ0000105',NULL,'{"kind":"state","device":"0a010c","serial":"MEQ0000105","source":"device","level":"info","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"time":"2026-11-11T17:31:00.122646322+01:00"}');
INSERT INTO events VALUES(19,'01a1cb72-ce05-7c54-8b91-7f4382c76fe0',1,'2026-11-11T16:31:00.122Z','state','device',0,'0a1b2c','MEQ0000001',NULL,'{"kind":"state","device":"0a1b2c","serial":"MEQ0000001","source":"device","level":"info","state":{"rf_address":"0a1b2c","type":"shutter_contact","name":"Fenster Kind 1","room_id":5,"serial":"MEQ0000001","reported":true,"state":{"mode":"vacation","window_open":true,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T17:31:00.119668222+01:00"},"time":"2026-11-11T17:31:00.122726722+01:00"}');
INSERT INTO events VALUES(20,'01a1cb72-ce05-7ca4-bb25-5bbd65cbe585',1,'2026-11-11T16:31:00.122Z','connected','cube',0,'','',NULL,'{"kind":"connected","source":"cube","level":"info","summary":{"cause":"start","devices":7,"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50,"rooms":6},"time":"2026-11-11T17:31:00.122778822+01:00"}');
INSERT INTO events VALUES(21,'01a1cb72-ce05-7cd0-a531-60e4c7e40d6c',NULL,'2026-11-11T16:31:00.320Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"settings.update","changes":{"stale_after":{"from":null,"to":"36h"}},"set":{"stale_after":"36h"}},"time":"2026-11-11T17:31:00.320444708+01:00"}');
INSERT INTO events VALUES(22,'01a1cb72-ce05-7d20-af96-5ae6cd9fd2b6',1,'2026-11-11T16:31:00.341Z','change','config',0,'','MEQ0000101',1,'{"kind":"change","serial":"MEQ0000101","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":21.5},"target":"device:MEQ0000101"},"time":"2026-11-11T17:31:00.341048707+01:00"}');
INSERT INTO events VALUES(23,'01a1cb72-ce05-7d56-a0fa-a5629b65da5e',1,'2026-11-11T16:31:00.363Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":20},"target":"room:1"},"time":"2026-11-11T17:31:00.363054605+01:00"}');
INSERT INTO events VALUES(24,'01a1cb72-ce05-7dd4-8580-3bdb7a6ee9e1',1,'2026-11-11T16:31:00.745Z','metadata','cube',0,'','',NULL,'{"kind":"metadata","source":"cube","level":"info","list_changes":[{"what":"room_renamed","room":1,"name":"Fixture room","from":"Wohnzimmer"}],"time":"2026-11-11T17:31:00.745053079+01:00"}');
INSERT INTO events VALUES(25,'01a1cb72-ce05-7df4-bb65-d1ee90f4f810',1,'2026-11-11T16:31:00.557Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"room.rename","set":{"name":"Fixture room"},"target":"room:1"},"time":"2026-11-11T17:31:00.557899192+01:00"}');
INSERT INTO events VALUES(26,'01a1cb72-ce05-7e12-80bd-4f4a713cfd55',1,'2026-11-11T16:31:00.952Z','backup','system',0,'','',1,'{"kind":"backup","source":"system","level":"info","summary":{"bytes":25694,"id":1,"reason":"manual"},"time":"2026-11-11T17:31:00.952798464+01:00"}');
INSERT INTO events VALUES(27,'01a1cb72-d5cb-7703-82db-ca1d19a76f94',NULL,'2026-11-11T16:31:03.114Z','server_stopping','system',0,'','',NULL,'{"kind":"server_stopping","source":"system","level":"info","summary":{"why":"stop"},"time":"2026-11-11T17:31:03.114664815+01:00"}');
CREATE TABLE push_subscriptions (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_id   INTEGER NOT NULL REFERENCES tokens (id) ON DELETE CASCADE,
    endpoint   TEXT    NOT NULL UNIQUE,
    p256dh     TEXT    NOT NULL,             -- the browser's P-256 key, base64url
    auth       TEXT    NOT NULL,             -- the browser's auth secret, base64url
    label      TEXT    NOT NULL DEFAULT '',  -- 'Firefox on the phone'
    created_at TEXT    NOT NULL,
    last_ok_at TEXT,                         -- the last message the push service took
    failures   INTEGER NOT NULL DEFAULT 0    -- failed deliveries since then
);
CREATE TABLE IF NOT EXISTS "backups" (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,  -- never given out twice
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
INSERT INTO backups VALUES(1,'01a1cb72-cd58-7000-aee4-3cb3b94af194',1,'2026-11-11T16:31:00.951Z','manual','json','massimo v0.14.0-alpha.1 (go1.26.8, linux/amd64)',25694,'ef50a177395fb0e2bc619c06d24bb2beac45d70c34423a71392151a94bb181c6',X'1f8b08000000000000ffec5c5d73a2c81abecfafa0b83aa78e63bac118b56a2e1a05c7249a01854cb2b5652120b002edf22126a7e69fedddfeb153881f28cacc183d581bbc984cf7dbbcefcbf3340d4f77c37faf08829c69ae6762876c10b014957d79a23943d9271b044901aafa09c24f100ee06d83860d00fe03a27fc9b829c656d4cc963dcfb4313103655829834fb23535e43224fea56358a6aae55a89b04c27985fcbb65aadfc3b3e5809461ad920a21c0882f434d79417ceee591e0000e032084190ee7828abaaab795e64073280e076651b9bae1dca6ee489846558a6c92b82f8be08e0f853b241fc16b773fc69d9c0b666cbbea994156caf1ca40d5704f1fb3ac3a1829db1a96f129d62d797ada1e6c8234b53c90631962d4f2b6d190377712686ef4f1bd7d7b63c2f6b7f7ea2cbaad6a881ebc579afda079e311c05be8f9d6110650bf618541c46ec546f9636ff6d189a8eafb9eba4a2d390ed0506edee60e99c20481b3bbe11110bd655a1a64d54f975138a20480307912f7a5d81c7634ff3879ea660478d40070bcbf74d025e60dbfb1360fa7b12a07f263e75383e5d05cb14d6ecba18dbde9adf751aa6baeac85b5971e6dc0f5c8d888edaa4b7a763c964f254b7dd5229b7ccdf7fb938d35f3dc31f9df277fff75f8aa1653aac6538aca41d9a8e4a50990e47190e6ff63b84990e950c87d594c3be6258f2f8cd5cf4a72cb72ab9ee01f1e5a96a3353d1f6f481bda8ad5dfbafd3456057564dd9c7eed03734d7c69e2ffb9b469be1a8bb1a8e683295bbb074410cd62eb6b0c1d81eee309d18afe0068c440a9b4b2a1a81b03dc6ae3ff4353b1a1da8750f24085253f0aa1ede24ea6d793ef4347f8a4d27f246d1499be9246d9572f2c0c8d930bef492976774d19a8e8ac3219e6ace2ae4cd01b36d3a81bf20652ba911c69e9f30a66d33d99a69c3a912c5ae2583ab9a225bca301e34484ff60337fa7fbac5721881d40e1849cf10245dc796e5681337d83a6d4d9b0ca72ed65dd9dee2251ed9e28c7e4bd4125b6dd6a046716f4bbb96c0f1cd450f03d5060dc82df3f7d24f79a56086d7db067d7394d7ac5c612dbaf99f3a57586f80d3e74a55d2b9264abf278f24fd40f30a424f95eb05101a6aaa53507aba5c2f8052df08dc82d193e57a018c8e5db3e0f354b95e009feba7b393307a7b0e9440bd60f457180d4ef79c5bf0990b9f57bbb5df3732d7977d6d5b72da58d5e2393d2790ada4cc4a8a4ab0a51c65c50f646bad46ebe55a4a64c5ea8adad27db2ef6beeebd0c2e1f634dac2aab92e76f7d45ba633196e19af9267b73bebb0774ae6dd730154c65cc0ee9cd07a12802a26013ef824009539e0548e1c7040c6d04035e0715eb3723dafb0fe88201d21563f224cbf2e003f224abf2aaa32e282eab18f419bfbda1eaff5a31faeb291a77246fed7e54f81fdc9b0ff45a15220ff0ee4af766b7f4a52c8818f0f0a8a837282a2ca9503728206172027e4f7cb099821279eb0e1a45647d79a02e6a729e86d0d58a88acb5415478e0e59d3181468c0334c63e4ffe47462ad52409fa3022ac0cf4f5715d8e7a2d60ae4735cac3acbf2238447a374890b40ff208cae766bdfab8060fda00282b5f2ed010554b90405347abf02aa6428a0d41ed6b5faa9e4a77e8a15958bd03ed9a3cd79969b8f9d1bca5a6e3ecbd238459f23d73cb7551684e64de8c9b7551694e64de9a9b755168ce6cde869b755167ce6cde7a995ea3950a2f247e9a44af51f83d1d56eedfbb7ffc17ac6f6bf9dbd815beb7597b0fd4f79bf5abdf9915addfb12e04da1563fb85aa592502f6d9bbbd7f1bb0eb2ef89e79813a68ede749ddf5a5d017eaeab7505fc79aed715e8e7b76257607fa16b7647eb4558cbf07a96578722ed50bc0af67190bfdaad3d816aab65bfb4451d506d954b506deafb555b3543b51df80acd5abb554fa4dde0160549f5562de7b1d7724b2715126efd3b744967ddc98f9fdda133c609788e919282c7eec4c800e0acf2ad003e37e956409f976c2b90cf47b215b8e725d70ae4f3116b05eeff6fa996f5321c3c28d32858a60fc8342af7ada07044ed595cf38c20ca65a860c797952c8906f6bd08c7698ee76b3fb7ae9601f74c5664dfc40eb95fef900dc27703edec08ae3f5e6a994ee2d3a5e497c6e6b3c7a5f80bc72500215d02cb5f0956943105805202a0445325288311a04b90a6e912889bad3e2adc6d443e6049d23b6dc4cfdbf23703a8dfeeacce97bbd98802f788c72d66326bcdda3dfcc263a6893a1cef3dd614bb768ff809c74cfce9c8e6f52e8f9916e2db7dca0a5e9ae8b5a9b30f8c3e1f3c5386f7fcf452959fa0fff2ad738f78f005457e44c8890314765b68de6d751e459b9bc84f1c18d11d7de9bfb968c726db75a5643ba96d588a0d670a2d186a13cd51c8de23deef09121bb50fbb03045af0ce78695b86daaebf76d865ae3acf3475b63b102561d1aec5863dbede7f7e12a6cfdf849962320ff2d3cda4d362b92887d656aecf5b3988540f8fda9cadd9963f7a925e193dca619acc61ce2673e0d499dcbe59e1c130088f1e36e718e5f228d852a0d082b59dc7e7cf2bce9a8d25e9b73c6698100dd0b8fe90081822e42186459bdfd7ebc3bf70bb2d3fce684b1940fd82aa0faf75fff99b1ebc7c63fdeeebcde465802bdd66fd0ff5e9ce422d341bb5eb3399964294e78f557b1242a8a9a3565cbe1b446514a22642e891452940d512d031d3e2d9367bcf2419a4ee5fd97a93579ed11bea5e87489cb406b53edf913a8ca8737d2475585117fa0cb7fecbf4f9ded7f91d32967666c7ce8a3a7787c0353b69aeecdc8e5de85f509c3458b505584dc4b676c07a7b602a419357ee63b01856e28437a1678837623ab9f8af204a5f7a2cc50dacfd765117d81bf1090c9ea54769bfbdcfb0034b18fd20cea3c4f9e78f9306ab1e83c5b3cd1db05eb7c142619f653c910fe79d16ab77fb689e72de623a7c7b2eb456f6169b4aa2fbc808cd3f36f66e9af1b0c2b1f71bfbbe382d5de4f1c67e649c1f9e4f1aac510c56c8b23b6081dd9ed5e7c4ce412663c63b126f64d823c60d71c40eba92201c64bccf099ed09d4b5c569cf69c7de344f1ac71d260c931583acbec80357f602af5245803abdffe01588f52e72513ac6e6f2ed680789fc538cd0dde8464cf49f72c45fadae3923de72c71d26029f1008f586e072c981ab3584397782b0b2cb1df66a4e6c1b146d4bb1ec3024efc9a7112af213be025be9601d603127a8cd47acb00eb147112603d349a61741b9cf4db0cff27ea89282a37d1a4df162653d47d8ecb0b7b05a3471497c349bfddef3aa80be2b23ee9b79bba8d1ebc45b9151ddf0716ea22c4e878f41095f5cf9f23adf4fbd5f7ff050000ffff2956d5535e640000',1,'',1,NULL,'01a1cb72-c9f4-7828-8aff-a1e0e47b2c2b');
DELETE FROM sqlite_sequence;
INSERT INTO sqlite_sequence VALUES('backups',1);
CREATE INDEX tokens_user ON tokens (user_id);
CREATE INDEX audit_time ON audit (time);
CREATE INDEX audit_user ON audit (user_id, time);
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');
CREATE INDEX commands_key ON commands (user_id, idempotency_key);
CREATE INDEX events_cube ON events (cube_id);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX events_cube_kind ON events (cube_id, kind);
CREATE INDEX events_cube_source ON events (cube_id, source);
CREATE INDEX events_cube_serial ON events (cube_id, serial);
CREATE INDEX events_user ON events (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX push_subscriptions_user ON push_subscriptions (user_id);
CREATE INDEX push_subscriptions_token ON push_subscriptions (token_id);
CREATE INDEX events_cube_warn ON events (cube_id) WHERE level >= 4;
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);
COMMIT;
