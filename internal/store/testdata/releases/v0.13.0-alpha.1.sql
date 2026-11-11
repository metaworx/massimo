-- The store that massimo v0.13.0-alpha.1 (b385064) wrote against the fake Cube,
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
INSERT INTO schema_version VALUES(1,1,'2026-11-11T08:22:25.540Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(2,1,'2026-11-11T08:22:25.548Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(3,1,'2026-11-11T08:22:25.549Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(4,1,'2026-11-11T08:22:25.549Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(5,1,'2026-11-11T08:22:25.554Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(6,1,'2026-11-11T08:22:25.554Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(7,1,'2026-11-11T08:22:25.557Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(8,1,'2026-11-11T08:22:25.558Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(9,1,'2026-11-11T08:22:25.563Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(10,1,'2026-11-11T08:22:25.564Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(11,1,'2026-11-11T08:22:25.564Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(12,1,'2026-11-11T08:22:25.565Z','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)');
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
INSERT INTO roles VALUES(1,'owner','',1,0,'2026-11-11T08:22:25.536Z','2026-11-11T08:22:25.536Z',NULL,NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO roles VALUES(2,'manager','',1,0,'2026-11-11T08:22:25.536Z','2026-11-11T08:22:25.536Z',NULL,NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO roles VALUES(3,'operator','',1,0,'2026-11-11T08:22:25.536Z','2026-11-11T08:22:25.536Z',NULL,NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO roles VALUES(4,'guest','',1,0,'2026-11-11T08:22:25.536Z','2026-11-11T08:22:25.536Z',NULL,NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO roles VALUES(5,'viewer','',1,0,'2026-11-11T08:22:25.536Z','2026-11-11T08:22:25.536Z',NULL,NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO roles VALUES(6,'integration','',1,0,'2026-11-11T08:22:25.536Z','2026-11-11T08:22:25.536Z',NULL,NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
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
INSERT INTO users VALUES(1,'01a1c9b3-7c21-7f18-9af2-6456cd3bb183','owner','','',1,0,NULL,NULL,'2026-11-11T08:22:25.569Z','2026-11-11T08:22:25.569Z',NULL,NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790',1,NULL,'');
INSERT INTO users VALUES(2,'01a1c9b3-8022-7565-bac6-ae27eefe3878','anna','','',5,0,NULL,NULL,'2026-11-11T08:22:26.594Z','2026-11-11T08:22:26.594Z',NULL,NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790',2,X'92676da6754caf45ed72ee6b3e31bc74f6a97f28f5b0ae9ee97e539ee3c8b004e9846e6b757c224490125bdfa7d951f321b2fcc2b111ae4c42163354a9087007c7bf9aaada1e252aebda7e0776743d3a935fb4fc2b5315f3e34857b01eebe3971994e09f119744c3b8feba7f9cb4197d3dca370846249216f9a0ae07bda5ee32285c335334800d36e6d5225fa8f0dfd1bbc4af6fc46932a7df4450dfc1521119ceb3e722911f5396827f79aef02d81d60a215b142e1788f38eca5684716a61c9','m=19456,t=2,p=1');
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
INSERT INTO tokens VALUES(1,'01a1c9b3-7c22-768b-861e-2057cc9195a9',1,'api','3dbd5e608845edf5f2de485da50c0a11655a49b1b5a2188168036453ab3f1b64','MASSIMO_API_TOKEN','2026-11-11T08:22:25.570Z',NULL,'2026-11-11T08:22:25.748Z','127.0.0.1',NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO tokens VALUES(2,'01a1c9b3-804a-78d2-ae24-c9fc48a16d44',2,'api','c75d7206362713de6dbf2a43ede32aaceacf7417dfc3804ed9a9b3dc5a32f0f2','fixture','2026-11-11T08:22:26.634Z',NULL,'2026-11-11T08:22:26.670Z','127.0.0.1',NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
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
INSERT INTO settings VALUES('node.id','"01a1c9b3-7c03-7976-a6b5-b3ad514b0790"','2026-11-11T08:22:25.539Z',NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO settings VALUES('tenant.id','"01a1c9b3-7c03-7996-bfb6-80d5475c047a"','2026-11-11T08:22:25.539Z',NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO settings VALUES('log.server_version','"v0.13.0-alpha.1"','2026-11-11T08:22:25.579Z',NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO settings VALUES('stale_after','"36h"','2026-11-11T08:22:25.810Z',NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
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
INSERT INTO cubes VALUES(1,'01a1c9b3-7c20-7ce8-8084-3791b0b8a62a','KEQ0000100',1,'Fake Cube','127.0.0.1:52937',0,'','','[]','env','','','','',NULL,NULL,'2026-11-11T08:22:25.568Z','2026-11-11T08:22:25.568Z',NULL,1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
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
INSERT INTO audit VALUES(1,'01a1c9b3-7d13-78f4-80d3-8a963e20e2a3','2026-11-11T08:22:25.808Z',1,1,'127.0.0.1','settings.update',NULL,'','{"body":{"stale_after":"36h"},"bytes":22,"method":"PUT","ms":3,"path":"/api/v1/settings","status":200}','ok',1);
INSERT INTO audit VALUES(2,'01a1c9b3-7d2a-7be7-a4c8-6ffe758cde9b','2026-11-11T08:22:25.831Z',1,1,'127.0.0.1','setpoint',1,'device:MEQ0000101','{"body":{"mode":"manual","temp":21.5},"bytes":32,"method":"PUT","ms":2,"path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","status":200}','ok',2);
INSERT INTO audit VALUES(3,'01a1c9b3-7df3-7a85-bf32-37cb022cc41a','2026-11-11T08:22:25.858Z',1,1,'127.0.0.1','setpoint',1,'room:1','{"body":{"mode":"manual","temp":20},"bytes":30,"method":"PUT","ms":176,"path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","status":200}','ok',4);
INSERT INTO audit VALUES(4,'01a1c9b3-7ebf-778c-ae5a-80a222ad60d3','2026-11-11T08:22:26.062Z',1,1,'127.0.0.1','room.rename',1,'room:1','{"body":{"name":"Fixture room"},"bytes":24,"method":"PUT","ms":174,"path":"/api/v1/cubes/KEQ0000100/rooms/1","status":200}','ok',6);
INSERT INTO audit VALUES(5,'01a1c9b3-7f8c-7f41-8552-ca5bd80424de','2026-11-11T08:22:26.256Z',1,1,'127.0.0.1','backup.take',1,'cube:KEQ0000100','{"body":{},"bytes":2,"method":"POST","ms":188,"path":"/api/v1/cubes/KEQ0000100/backups","status":201}','ok',8);
INSERT INTO audit VALUES(6,'01a1c9b3-8023-7ba5-9255-200ac03be402','2026-11-11T08:22:26.595Z',NULL,NULL,'cli','user.add',NULL,'user:anna','{"password":true,"role":"viewer"}','ok',NULL);
INSERT INTO audit VALUES(7,'01a1c9b3-804f-7b7c-a7e9-0c58739aade1','2026-11-11T08:22:26.639Z',NULL,NULL,'cli','token.add',NULL,'token:2','{"label":"fixture","user":"anna"}','ok',NULL);
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
INSERT INTO commands VALUES(1,'01a1c9b3-7d10-7b50-9647-873a70c35378',NULL,NULL,1,1,'','settings.update','','{"hash":"a1a56e0ab8f245f264cab562d3430faa07b670437af7d261b6497c8824be7018","method":"PUT","path":"/api/v1/settings","query":""}','acknowledged',200,replace('{\n  "auth.password_login": {\n    "value": "true",\n    "default": "true",\n    "set": false,\n    "in_effect": "true",\n    "applies": "now"\n  },\n  "backups.auto_max": {\n    "value": "120",\n    "default": "120",\n    "set": false,\n    "in_effect": "120",\n    "applies": "now"\n  },\n  "backups.auto_min": {\n    "value": "60",\n    "default": "60",\n    "set": false,\n    "in_effect": "60",\n    "applies": "now"\n  },\n  "backups.keep_days_auto": {\n    "value": "90",\n    "default": "90",\n    "set": false,\n    "in_effect": "90",\n    "applies": "now"\n  },\n  "backups.keep_days_manual": {\n    "value": "365",\n    "default": "365",\n    "set": false,\n    "in_effect": "365",\n    "applies": "now"\n  },\n  "backups.keep_newest": {\n    "value": "10",\n    "default": "10",\n    "set": false,\n    "in_effect": "10",\n    "applies": "now"\n  },\n  "commands.keep_days": {\n    "value": "0",\n    "default": "0",\n    "set": false,\n    "in_effect": "0",\n    "applies": "now"\n  },\n  "discover_interval": {\n    "value": "30m",\n    "default": "30m",\n    "set": false,\n    "in_effect": "30m",\n    "applies": "restart"\n  },\n  "log.keep_days": {\n    "value": "30",\n    "default": "30",\n    "set": false,\n    "in_effect": "30",\n    "applies": "now"\n  },\n  "node.id": {\n    "value": "01a1c9b3-7c03-7976-a6b5-b3ad514b0790",\n    "set": true,\n    "read_only": true\n  },\n  "poll_interval": {\n    "value": "1m",\n    "default": "1m",\n    "set": false,\n    "in_effect": "1m",\n    "applies": "restart"\n  },\n  "refresh_interval": {\n    "value": "6h",\n    "default": "6h",\n    "set": false,\n    "in_effect": "6h",\n    "applies": "restart"\n  },\n  "stale_after": {\n    "value": "36h",\n    "default": "24h",\n    "set": true,\n    "in_effect": "36h",\n    "applies": "now"\n  },\n  "tenant.id": {\n    "value": "01a1c9b3-7c03-7996-bfb6-80d5475c047a",\n    "set": true,\n    "read_only": true\n  },\n  "update.channel": {\n    "value": "alpha",\n    "default": "alpha",\n    "set": false,\n    "in_effect": "alpha",\n    "applies": "now"\n  }\n}\n','\n',char(10)),'2026-11-11T08:22:25.808Z','2026-11-11T08:22:25.812Z','2026-11-11T08:22:25.812Z',2,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO commands VALUES(2,'01a1c9b3-7d28-7603-94e1-0b99fda9a3c1',NULL,1,1,1,'','setpoint','device:MEQ0000101','{"hash":"a4003acf94e9c73f51fc3c487de1a17dcdead8493e4d06e030efbfed5cf380e4","method":"PUT","path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-11T09:22:25.833956786+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T08:22:25.832Z','2026-11-11T08:22:25.835Z','2026-11-11T08:22:25.835Z',3,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO commands VALUES(3,'01a1c9b3-7d29-7a22-8917-e68e0e07521e',2,1,1,1,'','step','device:0a010a','{"key":"setpoint:device:0a010a","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T09:22:25.833956786+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T08:22:25.833Z','2026-11-11T08:22:25.834Z','2026-11-11T08:22:25.834Z',3,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO commands VALUES(4,'01a1c9b3-7d42-7d27-b6d0-b787bb2b6e4f',NULL,1,1,1,'','setpoint','room:1','{"hash":"a668a611353079d05040a7b7117ab0201078669851996d42d0d1d7f761b42b96","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-11T09:22:26.034828485+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T08:22:25.858Z','2026-11-11T08:22:26.035Z','2026-11-11T08:22:26.035Z',3,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO commands VALUES(5,'01a1c9b3-7d43-7c13-b974-fdb42638b57a',4,1,1,1,'','step','','{"key":"setpoint:room:1","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T09:22:26.034828485+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T08:22:25.859Z','2026-11-11T08:22:26.035Z','2026-11-11T08:22:26.035Z',3,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO commands VALUES(6,'01a1c9b3-7e0f-70ca-a3d9-af2b1424c31a',NULL,1,1,1,'','room.rename','room:1','{"hash":"0d3ae207663d7e2209c687cfb34abbb665b1c5e3cdee82c501293f945e264aeb","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1","query":""}','acknowledged',200,replace('{\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T08:22:26.063Z','2026-11-11T08:22:26.239Z','2026-11-11T08:22:26.239Z',3,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO commands VALUES(7,'01a1c9b3-7e0f-7f08-89a3-8ee6ca4ace58',6,1,1,1,'','step','','{"key":"metadata","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T09:22:26.034828485+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T08:22:26.063Z','2026-11-11T08:22:26.239Z','2026-11-11T08:22:26.239Z',3,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO commands VALUES(8,'01a1c9b3-7ed1-72cb-a2a3-8c838df0cdae',NULL,1,1,1,'','backup.take','cube:KEQ0000100','{"hash":"100f64b5013f924491098335cbb093208b009a8b0461887abc12f9c6cdb591d1","method":"POST","path":"/api/v1/cubes/KEQ0000100/backups","query":""}','acknowledged',201,replace('{\n  "id": 1,\n  "taken_at": "2026-11-11T08:22:26.443Z",\n  "reason": "manual",\n  "format": "json",\n  "tool": "massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)",\n  "size": 25694,\n  "sha256": "93e8a2ceac7f59d2a3139f9fdcd53604bc2493abb2dfee0f047b0ec48e033ad8",\n  "user": "owner"\n}\n','\n',char(10)),'2026-11-11T08:22:26.257Z','2026-11-11T08:22:26.445Z','2026-11-11T08:22:26.445Z',3,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
INSERT INTO commands VALUES(9,'01a1c9b3-7ed2-7267-b045-5fc2159961dc',8,1,1,1,'','step','','{"key":"","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T09:22:26.034828485+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T08:22:26.258Z','2026-11-11T08:22:26.438Z','2026-11-11T08:22:26.438Z',3,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
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
INSERT INTO backups VALUES(1,'01a1c9b3-7f8b-73e7-bcd4-6a46b510eb1f',1,'2026-11-11T08:22:26.443Z','manual','json','massimo v0.13.0-alpha.1 (go1.26.8, linux/amd64)',25694,'93e8a2ceac7f59d2a3139f9fdcd53604bc2493abb2dfee0f047b0ec48e033ad8',X'1f8b08000000000000ffec5c5d73a2c81abecfafa0b83aa78e63ba411db56a2e1a05c7249a01854cb2b5652120b002edf22126a7e69fedddfeb153881f28cacc183d583b789374bfcdfbbe3c4fd3f07437fcf78620c8b9e67a2676c826014b51d997a79a33927db2499014a06a1f20fc00e110349a14d5a46aff01b009001937c5d88a9ad9b2e7993626e6a00ce932f8205b33432e43e25f3a8665aa56ae9708cb7482c5ad6cabb5cabfe3839560ac914d22ca8120484f734d79e9ec9ee5010000ae821004e94e46b2aaba9ae745762003083eae6d13d3b543d98d3c91b00ccb34794310df96011c7f463689dfe2768e3f2b1bd8d66cd93795b282edb583b4e186207edf643852b03331f56da233ecfab235d21c796c692ad92426b2e569a51d63e02ecfc4f0fd59f3f6d6961765edcf0f7459d59a7570bb3cef75fbc03346e3c0f7b1330aa26cc101838ac3889d5a7565f3df46a1e9f89abb492a3a0dd95e62d0e90d57ce0982b4b1e31b11b16053156ada54955fb7a10882347010f9a237157832f1347fe4690a76d40874b0b47cdb26e005b67d380166702001fa47e253c7e3d335b04a61c3ae8bb1ed6df8dda461aaeb8ebc9315672efcc0d588e8a86d7a073a964c264f75d72d9572cbfcfd978b33fd3532fcd1297ff77fffa5185aa6c37a86c34adaa1e9a80495e9709ce1b07ad821cc74a86438aca51c0e14c392276fe6b23f65b955c94d0f882f4f559b9b8a76a00f1c446de3da7f9d2d03bbb26aca3e7647bea1b936f67cd9df36da0e47bdf5704493a9dc85950b62b871b1830dc6f6688fe9c47805b7602452d85e52d10884ed0976fd91afd9d1e8406d7a2041909a82d7f5b09aa8b7e5c5c8d3fc19369dc81b45276da693b455cac9032367a3f8d24b5e9ed1456b3a2a0e4778a639eb90d52366db74027f49ca4e52638c3d3f614cdbe6b235d74633258a5d4f06573545b694513c68909eec076ef47fbac56a1881d41e1849cf10245dc796d5681337d8396d4d9b8e662ed65dd9dee1251ed9e28c7e4bd4123b6d36a046713f96f62d81e39bcb1e066a4d1a903be66fa51ff24ac10caf1f9b74f524af59b9c27a74f33f77aeb0d104e7cf95aaa4734d947e4f1e49fa81e615849e2bd72b2034d454a7a0f47cb95e01a5be11b805a367cbf50a189db866c1e7b972bd023e374f676761f4e32550028d82d19f613438df736ec1672e7cdeecd77edbca5c5ff6b55dc96963558be7f49c40b692322b292ac18e7294153f90ad8d1a6d94eb299115ab2b6a47f7c9beafb9af230b87bbd3684babe6bad83d506f99ce74b463bc499eddfeacc3c1299977cf0550197301fb73429b4900aa9804f8c52701a8cc01a772e28003328606aa094ff39a95eb6585f5af08d20962f55784e9e705e0af88d2cf8aaa8cb8a076ea63d0f6be76c06be3e487ab6ce4a99c91ff79f953607f36ec7f52a814c8bf03f99bfdda1f921472e0e3a382e2a89ca0a872e5889ca0c115c809f9fd720266c889276c38a9d5d18da680f9690a7a570316aae23a55c589a343d63406059af002d318f93f399d59ab14d0e7a8800af0f3d35505f6b9a8b502f91c17ab2eb2fc08e1c9285de302d03f08a39bfddaf72a20d838aa8060bdfcf18802aa5c83021abf5f01553214506a0feb46fd54f2533fc58aca55689fecd1e632cbcda7ce0d652d375f64699ca22f916b9edb2a0b42f326f4ecdb2a0b4af3a6f4dcdb2a0b46f366f4bcdb2a0b3ef3e6f3dc4af5122851f9a37456a5fa8fc1e866bff6fddbff602363fbdfdedec09df5ba6bd8fea7bc5fad56bfa7560fbe04582dd4ea2fae56a924d42bdbf6ee75faae83ec7be225e684a993375de7b75657809feb6a5d017f9eeb7505faf9add815d85fe99addc97a11d633bc5ee4d5a1483b14af82fd3ac8dfecd79e41b5d5b35fdaa28ea8b6ca35a836f5fdaaad96a1da8e7c8566a3dd6a67d26e708782a47aab95f3d86bb9a3930a09b7f91dbba4b3eee4a7cfeed019e304bcc44849c153776264007051f956009f9b742ba0cf4bb615c8e723d90adcf3926b05f2f988b502f7ffb754cb7a190e1e9569142cd347641a95fb565038a60e2cae794610e53252b0e3cb4a964403875e84e334c7f3b51f5b57cb807b2e2bb26f62873cac77c826e1bb81767104371f2fb54c27f1e952f27373fbd9e352fc85e31280902e81d5af042bca840240290150a2a91294c118d02548d37409c4cdd61f15ee35231fb024e9dd0ee2171df9ab01d4af7756f7f3dd7c4c817bc4e336339db7e79d3e7ee131d3425d8ef71eeb8a5dbf47fc9463a6fe6c6cf37a8fc74c1bf19d0165052f2df4dad2d907465f0c9f29c37b7e7aa9c94fd07ff9dabd473cf88c223f22e4c4210a7b6db4e8b5bb8fa2cd4de5270e8ce9aebef2df5ab66393ed7a52b29dd4312cc5867385160cb585162864ef11eff705898dda87bd21026d7867bc742c43ed345ebbec2a579d675a3adb1b8a92b06cd766c33edf183c3f09b3e7afc25c319907f9a93aedb6592ecaa1bd93ebf34e0e22d5c7e30e676bb6e58f9fa457468f7298257358b0c91c38752e77aa6b3c1806e1f1c3f61ca35c1e055b0a145ab076f3f8f469cd59abb922fd238f1926444334693c24028608798861d1f6f7e5f6f82fdc6dcb4f32da5206503fa3dac36bc37ffeaa072f5f59bff75a9dbe0c71a5d76afca13edd59a88de6e34e632ed35288f2fcb16a5f4208b574d48ecb77c3a88c42d442083db22805a85a023a66da3cdb61ef992483d4fd2bdb68f1ca337a43bddb1089d3f6b03ee0bb529711756e80a42e2beac280e1367f9901dfffb2b843c6caceecd95951e7ee10b865a7adb59ddbb30b832b8a9306abbe04ab85d8f61e586f0f4c2568f1ca7d0c16c34a9cf026f40db12aa6938bff0aa2f4b9cf52dcd03a6c177581ad8a4f60f82c3d4a87ed03861d5ac2f83b711e25cebf7c9c34588d182c9e6ded81f5ba0b160a072ce3897cb8e8b659bd37408b94f336d3e53b0ba1bdb6b7d95412bd474668fdb1b5f7d28c87158ebddfda0fc569eb228fb7f613e37cf77cd2608d63b04296dd030becf7ac0127768f321933de957823c31e316e886376d89304e128e3034ef084de42e2b2e27416ec1b278a178d93064b8ec1d259660facc503536924c11a5a83ce77c07a94ba2f9960f5fa0bb10ec4fb2cc6696ef826247b4eba6729d2973e97ec39178993064b890778c4727b60c1d498c51abac45b596089830e23b58e8e35a2def3181670e2978c93780dd9212ff1f50cb01e90d067a4f65b0658e7889300eba1d90aa3dbe074d061f83f515f4451b985a6838e309da1de735c5eda2b183da2b81c4e079d41cf413d1097f5e9a0d3d26df4e02dcbede8f801b0500f2146c7e387a8ac7ffa1469a5df6fbefd2f0000ffff485d59065e640000',1,'',1,NULL,'01a1c9b3-7c03-7976-a6b5-b3ad514b0790');
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
INSERT INTO events VALUES(1,'01a1c9b3-801a-7564-9b61-69f85ccafb4f',NULL,'2026-11-11T08:22:25.579Z','server_started','system',0,'','',NULL,'{"kind":"server_started","source":"system","level":"info","summary":{"version":"v0.13.0-alpha.1"},"time":"2026-11-11T09:22:25.579370186+01:00"}');
INSERT INTO events VALUES(2,'01a1c9b3-801a-76d5-b1f1-7a5fb16e023e',1,'2026-11-11T08:22:25.582Z','hello','cube',-4,'','',NULL,'{"kind":"hello","source":"cube","level":"debug","summary":{"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50},"time":"2026-11-11T09:22:25.582439586+01:00"}');
INSERT INTO events VALUES(3,'01a1c9b3-801a-776e-a6dd-a0f86c8dc394',1,'2026-11-11T08:22:25.582Z','metadata','cube',-4,'','',NULL,'{"kind":"metadata","source":"cube","level":"debug","summary":{"devices":7,"rooms":6},"time":"2026-11-11T09:22:25.582643886+01:00"}');
INSERT INTO events VALUES(4,'01a1c9b3-801a-77d1-8a79-66988e3ecf9c',1,'2026-11-11T08:22:25.583Z','config','cube',-4,'0a0107','KEQ0000100',NULL,'{"kind":"config","device":"0a0107","serial":"KEQ0000100","source":"cube","level":"debug","summary":{"portal_enabled":false,"tz_summer":"BST","tz_winter":"GMT"},"time":"2026-11-11T09:22:25.583499686+01:00"}');
INSERT INTO events VALUES(5,'01a1c9b3-801a-7833-bc3c-24e1eaf2d9b5',1,'2026-11-11T08:22:25.583Z','config','device',-4,'0a010d','MEQ0000106',NULL,'{"kind":"config","device":"0a010d","serial":"MEQ0000106","source":"device","level":"debug","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"summary":{"comfort_temp":21.5,"eco_temp":16.5,"temp_offset":0},"time":"2026-11-11T09:22:25.583537086+01:00"}');
INSERT INTO events VALUES(6,'01a1c9b3-801a-788b-bcd0-c38afb1893b4',1,'2026-11-11T08:22:25.585Z','config','device',-4,'0a0108','MEQ0000103',NULL,'{"kind":"config","device":"0a0108","serial":"MEQ0000103","source":"device","level":"debug","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:25.585141686+01:00"}');
INSERT INTO events VALUES(7,'01a1c9b3-801a-78d3-b058-d981cdc9e931',1,'2026-11-11T08:22:25.585Z','config','device',-4,'0a0109','MEQ0000102',NULL,'{"kind":"config","device":"0a0109","serial":"MEQ0000102","source":"device","level":"debug","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:25.585209286+01:00"}');
INSERT INTO events VALUES(8,'01a1c9b3-801a-791e-9289-370b7468616a',1,'2026-11-11T08:22:25.585Z','config','device',-4,'0a010b','MEQ0000104',NULL,'{"kind":"config","device":"0a010b","serial":"MEQ0000104","source":"device","level":"debug","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:25.585234886+01:00"}');
INSERT INTO events VALUES(9,'01a1c9b3-801a-7972-a488-d8c348e6fe45',1,'2026-11-11T08:22:25.585Z','config','device',-4,'0a010a','MEQ0000101',NULL,'{"kind":"config","device":"0a010a","serial":"MEQ0000101","source":"device","level":"debug","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:25.585252986+01:00"}');
INSERT INTO events VALUES(10,'01a1c9b3-801a-79fd-8f42-a4a05a899d93',1,'2026-11-11T08:22:25.585Z','config','device',-4,'0a010c','MEQ0000105',NULL,'{"kind":"config","device":"0a010c","serial":"MEQ0000105","source":"device","level":"debug","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:25.585271786+01:00"}');
INSERT INTO events VALUES(11,'01a1c9b3-801a-7a56-bb0f-88629c366e0b',1,'2026-11-11T08:22:25.585Z','state','device',0,'0a010d','MEQ0000106',NULL,'{"kind":"state","device":"0a010d","serial":"MEQ0000106","source":"device","level":"info","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"time":"2026-11-11T09:22:25.585295286+01:00"}');
INSERT INTO events VALUES(12,'01a1c9b3-801a-7af5-a108-8ca51a6dc256',1,'2026-11-11T08:22:25.585Z','state','device',0,'0a0108','MEQ0000103',NULL,'{"kind":"state","device":"0a0108","serial":"MEQ0000103","source":"device","level":"info","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"time":"2026-11-11T09:22:25.585437986+01:00"}');
INSERT INTO events VALUES(13,'01a1c9b3-801a-7b3e-ae0f-5869912abe35',1,'2026-11-11T08:22:25.585Z','state','device',0,'0a0109','MEQ0000102',NULL,'{"kind":"state","device":"0a0109","serial":"MEQ0000102","source":"device","level":"info","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"time":"2026-11-11T09:22:25.585470986+01:00"}');
INSERT INTO events VALUES(14,'01a1c9b3-801a-7b8e-bc14-cf35fcacfb79',1,'2026-11-11T08:22:25.585Z','state','device',0,'0a010b','MEQ0000104',NULL,'{"kind":"state","device":"0a010b","serial":"MEQ0000104","source":"device","level":"info","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"time":"2026-11-11T09:22:25.585499286+01:00"}');
INSERT INTO events VALUES(15,'01a1c9b3-801a-7bd5-a669-c5858e82266f',1,'2026-11-11T08:22:25.585Z','state','device',0,'0a010a','MEQ0000101',NULL,'{"kind":"state","device":"0a010a","serial":"MEQ0000101","source":"device","level":"info","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"time":"2026-11-11T09:22:25.585521686+01:00"}');
INSERT INTO events VALUES(16,'01a1c9b3-801a-7c1a-9236-92effcaa1929',1,'2026-11-11T08:22:25.585Z','state','device',0,'0a010c','MEQ0000105',NULL,'{"kind":"state","device":"0a010c","serial":"MEQ0000105","source":"device","level":"info","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"time":"2026-11-11T09:22:25.585543386+01:00"}');
INSERT INTO events VALUES(17,'01a1c9b3-801a-7c73-ae9b-87d12209e190',1,'2026-11-11T08:22:25.585Z','state','device',0,'0a1b2c','MEQ0000001',NULL,'{"kind":"state","device":"0a1b2c","serial":"MEQ0000001","source":"device","level":"info","state":{"rf_address":"0a1b2c","type":"shutter_contact","name":"Fenster Kind 1","room_id":5,"serial":"MEQ0000001","reported":true,"state":{"mode":"vacation","window_open":true,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:25.582311786+01:00"},"time":"2026-11-11T09:22:25.585564986+01:00"}');
INSERT INTO events VALUES(18,'01a1c9b3-801a-7ccb-96db-d4cb24493178',1,'2026-11-11T08:22:25.585Z','connected','cube',0,'','',NULL,'{"kind":"connected","source":"cube","level":"info","summary":{"cause":"start","devices":7,"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50,"rooms":6},"time":"2026-11-11T09:22:25.585588586+01:00"}');
INSERT INTO events VALUES(19,'01a1c9b3-801a-7d1c-bc1b-75b7a5e1d61f',NULL,'2026-11-11T08:22:25.808Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"settings.update","changes":{"stale_after":{"from":null,"to":"36h"}},"set":{"stale_after":"36h"}},"time":"2026-11-11T09:22:25.808339586+01:00"}');
INSERT INTO events VALUES(20,'01a1c9b3-801a-7d7b-a5a6-229cee89f6a4',1,'2026-11-11T08:22:25.831Z','change','config',0,'','MEQ0000101',1,'{"kind":"change","serial":"MEQ0000101","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":21.5},"target":"device:MEQ0000101"},"time":"2026-11-11T09:22:25.831957786+01:00"}');
INSERT INTO events VALUES(21,'01a1c9b3-801a-7dcd-b4b3-3341f012550a',1,'2026-11-11T08:22:25.858Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":20},"target":"room:1"},"time":"2026-11-11T09:22:25.858537486+01:00"}');
INSERT INTO events VALUES(22,'01a1c9b3-801a-7e2a-9efa-227665bbdfdc',1,'2026-11-11T08:22:26.237Z','metadata','cube',0,'','',NULL,'{"kind":"metadata","source":"cube","level":"info","list_changes":[{"what":"room_renamed","room":1,"name":"Fixture room","from":"Wohnzimmer"}],"time":"2026-11-11T09:22:26.237426585+01:00"}');
INSERT INTO events VALUES(23,'01a1c9b3-801a-7e74-912f-b907c88ef314',1,'2026-11-11T08:22:26.062Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"room.rename","set":{"name":"Fixture room"},"target":"room:1"},"time":"2026-11-11T09:22:26.062813685+01:00"}');
INSERT INTO events VALUES(24,'01a1c9b3-801a-7eca-b0c4-a966be247d38',1,'2026-11-11T08:22:26.444Z','backup','system',0,'','',1,'{"kind":"backup","source":"system","level":"info","summary":{"bytes":25694,"id":1,"reason":"manual"},"time":"2026-11-11T09:22:26.444065884+01:00"}');
INSERT INTO events VALUES(25,'01a1c9b3-8025-70b0-a190-f5c46c794903',NULL,'2026-11-11T08:22:26.596Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"user.add","set":{"password":true,"role":"viewer"},"subject":{"display_name":"","id":2,"name":"anna","role":"viewer","uuid":"01a1c9b3-8022-7565-bac6-ae27eefe3878"},"target":"user:anna","via":"cli"},"time":"2026-11-11T09:22:26.596692883+01:00"}');
INSERT INTO events VALUES(26,'01a1c9b3-8050-7ca0-925d-2569d3b2464c',NULL,'2026-11-11T08:22:26.640Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"token.add","set":{"label":"fixture","user":"anna"},"target":"token:2","via":"cli"},"time":"2026-11-11T09:22:26.640344683+01:00"}');
INSERT INTO events VALUES(27,'01a1c9b3-884e-7715-a053-f7b34456c5ab',NULL,'2026-11-11T08:22:28.685Z','server_stopping','system',0,'','',NULL,'{"kind":"server_stopping","source":"system","level":"info","summary":{"why":"stop"},"time":"2026-11-11T09:22:28.685972174+01:00"}');
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
CREATE INDEX tokens_user ON tokens (user_id);
CREATE INDEX audit_time ON audit (time);
CREATE INDEX audit_user ON audit (user_id, time);
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');
CREATE INDEX commands_key ON commands (user_id, idempotency_key);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);
CREATE INDEX events_cube ON events (cube_id);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX events_cube_kind ON events (cube_id, kind);
CREATE INDEX events_cube_source ON events (cube_id, source);
CREATE INDEX events_cube_serial ON events (cube_id, serial);
CREATE INDEX events_user ON events (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX push_subscriptions_user ON push_subscriptions (user_id);
CREATE INDEX push_subscriptions_token ON push_subscriptions (token_id);
CREATE INDEX events_cube_warn ON events (cube_id) WHERE level >= 4;
COMMIT;
