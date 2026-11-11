-- The store that massimo v0.13.0-alpha.2 (fdd437f) wrote against the fake Cube,
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
INSERT INTO schema_version VALUES(1,1,'2026-11-11T08:22:37.566Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(2,1,'2026-11-11T08:22:37.574Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(3,1,'2026-11-11T08:22:37.575Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(4,1,'2026-11-11T08:22:37.576Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(5,1,'2026-11-11T08:22:37.580Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(6,1,'2026-11-11T08:22:37.581Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(7,1,'2026-11-11T08:22:37.584Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(8,1,'2026-11-11T08:22:37.585Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(9,1,'2026-11-11T08:22:37.590Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(10,1,'2026-11-11T08:22:37.591Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(11,1,'2026-11-11T08:22:37.592Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(12,1,'2026-11-11T08:22:37.593Z','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)');
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
INSERT INTO roles VALUES(1,'owner','',1,0,'2026-11-11T08:22:37.565Z','2026-11-11T08:22:37.565Z',NULL,NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO roles VALUES(2,'manager','',1,0,'2026-11-11T08:22:37.565Z','2026-11-11T08:22:37.565Z',NULL,NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO roles VALUES(3,'operator','',1,0,'2026-11-11T08:22:37.565Z','2026-11-11T08:22:37.565Z',NULL,NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO roles VALUES(4,'guest','',1,0,'2026-11-11T08:22:37.565Z','2026-11-11T08:22:37.565Z',NULL,NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO roles VALUES(5,'viewer','',1,0,'2026-11-11T08:22:37.565Z','2026-11-11T08:22:37.565Z',NULL,NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO roles VALUES(6,'integration','',1,0,'2026-11-11T08:22:37.565Z','2026-11-11T08:22:37.565Z',NULL,NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
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
INSERT INTO users VALUES(1,'01a1c9b3-ab1e-7ac4-b743-6cd2d6d4bf5a','owner','','',1,0,NULL,NULL,'2026-11-11T08:22:37.598Z','2026-11-11T08:22:37.598Z',NULL,NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4',1,NULL,'');
INSERT INTO users VALUES(2,'01a1c9b3-aef3-7192-a332-8123c8076aec','anna','','',5,0,NULL,NULL,'2026-11-11T08:22:38.579Z','2026-11-11T08:22:38.579Z',NULL,NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4',2,X'78cfbf0f5a662e29e5a97f3b638d1446fd9da8f0b18706cc7e6ba555703a161acf7b78b7f5b2588800c1e9c1444c63e4b564840dcd511ec439d4ec907c38339b11dbbfc6b90d551adf63b8b2a488754027a26d1e0b0192b026f6063deb47e382e5f935ff7f95467881fcd8d7597ed13d1dd8ae7b5e24c99ec119d3eef1d7c44ab292061319490b2688dc64130e156fd4a3c1e62aa7054c099dfcde622f9e411f96db721919524e6c227144ade6c2fde12e3a17e1ad451e7cc9f415c979d45545','m=19456,t=2,p=1');
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
INSERT INTO tokens VALUES(1,'01a1c9b3-ab1f-71ac-aad2-fca96792d30d',1,'api','3dbd5e608845edf5f2de485da50c0a11655a49b1b5a2188168036453ab3f1b64','MASSIMO_API_TOKEN','2026-11-11T08:22:37.599Z',NULL,'2026-11-11T08:22:37.773Z','127.0.0.1',NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO tokens VALUES(2,'01a1c9b3-af1d-74ff-b241-7278e76369c0',2,'api','25f1ea18ba6b74e47466734c6858c9eb6699b9b656a8a0c56bb818f13cab1e25','fixture','2026-11-11T08:22:38.621Z',NULL,'2026-11-11T08:22:38.651Z','127.0.0.1',NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
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
INSERT INTO settings VALUES('node.id','"01a1c9b3-aafe-765c-ae2a-1715eb6a78e4"','2026-11-11T08:22:37.566Z',NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO settings VALUES('tenant.id','"01a1c9b3-aafe-7667-8703-470c0e6fb1c9"','2026-11-11T08:22:37.566Z',NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO settings VALUES('log.server_version','"v0.13.0-alpha.2"','2026-11-11T08:22:37.610Z',NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO settings VALUES('stale_after','"36h"','2026-11-11T08:22:37.819Z',NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
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
INSERT INTO cubes VALUES(1,'01a1c9b3-ab1d-7718-b667-368e9758ce1d','KEQ0000100',1,'Fake Cube','127.0.0.1:47773',0,'','','[]','env','','','','',NULL,NULL,'2026-11-11T08:22:37.597Z','2026-11-11T08:22:37.597Z',NULL,1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
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
INSERT INTO audit VALUES(1,'01a1c9b3-abfd-7036-a240-b837926573fa','2026-11-11T08:22:37.818Z',1,1,'127.0.0.1','settings.update',NULL,'','{"body":{"stale_after":"36h"},"bytes":22,"method":"PUT","ms":2,"path":"/api/v1/settings","status":200}','ok',1);
INSERT INTO audit VALUES(2,'01a1c9b3-ac1a-7d2c-9051-515478bdfb76','2026-11-11T08:22:37.846Z',1,1,'127.0.0.1','setpoint',1,'device:MEQ0000101','{"body":{"mode":"manual","temp":21.5},"bytes":32,"method":"PUT","ms":3,"path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","status":200}','ok',2);
INSERT INTO audit VALUES(3,'01a1c9b3-ace3-7578-ae68-308b834c4c72','2026-11-11T08:22:37.870Z',1,1,'127.0.0.1','setpoint',1,'room:1','{"body":{"mode":"manual","temp":20},"bytes":30,"method":"PUT","ms":179,"path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","status":200}','ok',4);
INSERT INTO audit VALUES(4,'01a1c9b3-adac-7099-9767-c800f5c1659c','2026-11-11T08:22:38.068Z',1,1,'127.0.0.1','room.rename',1,'room:1','{"body":{"name":"Fixture room"},"bytes":24,"method":"PUT","ms":182,"path":"/api/v1/cubes/KEQ0000100/rooms/1","status":200}','ok',6);
INSERT INTO audit VALUES(5,'01a1c9b3-ae7b-7c5b-a512-ca309628a743','2026-11-11T08:22:38.269Z',1,1,'127.0.0.1','backup.take',1,'cube:KEQ0000100','{"body":{},"bytes":2,"method":"POST","ms":190,"path":"/api/v1/cubes/KEQ0000100/backups","status":201}','ok',8);
INSERT INTO audit VALUES(6,'01a1c9b3-aef3-7b19-9f6e-854e887899be','2026-11-11T08:22:38.579Z',NULL,NULL,'cli','user.add',NULL,'user:anna','{"password":true,"role":"viewer"}','ok',NULL);
INSERT INTO audit VALUES(7,'01a1c9b3-af1d-7b4e-b5c6-65f182c196cc','2026-11-11T08:22:38.621Z',NULL,NULL,'cli','token.add',NULL,'token:2','{"label":"fixture","user":"anna"}','ok',NULL);
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
INSERT INTO commands VALUES(1,'01a1c9b3-abfa-7aa4-8e6a-20a3cd277bc3',NULL,NULL,1,1,'','settings.update','','{"hash":"a1a56e0ab8f245f264cab562d3430faa07b670437af7d261b6497c8824be7018","method":"PUT","path":"/api/v1/settings","query":""}','acknowledged',200,replace('{\n  "auth.password_login": {\n    "value": "true",\n    "default": "true",\n    "set": false,\n    "in_effect": "true",\n    "applies": "now"\n  },\n  "backups.auto_max": {\n    "value": "120",\n    "default": "120",\n    "set": false,\n    "in_effect": "120",\n    "applies": "now"\n  },\n  "backups.auto_min": {\n    "value": "60",\n    "default": "60",\n    "set": false,\n    "in_effect": "60",\n    "applies": "now"\n  },\n  "backups.keep_days_auto": {\n    "value": "90",\n    "default": "90",\n    "set": false,\n    "in_effect": "90",\n    "applies": "now"\n  },\n  "backups.keep_days_manual": {\n    "value": "365",\n    "default": "365",\n    "set": false,\n    "in_effect": "365",\n    "applies": "now"\n  },\n  "backups.keep_newest": {\n    "value": "10",\n    "default": "10",\n    "set": false,\n    "in_effect": "10",\n    "applies": "now"\n  },\n  "commands.keep_days": {\n    "value": "0",\n    "default": "0",\n    "set": false,\n    "in_effect": "0",\n    "applies": "now"\n  },\n  "discover_interval": {\n    "value": "30m",\n    "default": "30m",\n    "set": false,\n    "in_effect": "30m",\n    "applies": "restart"\n  },\n  "log.keep_days": {\n    "value": "30",\n    "default": "30",\n    "set": false,\n    "in_effect": "30",\n    "applies": "now"\n  },\n  "node.id": {\n    "value": "01a1c9b3-aafe-765c-ae2a-1715eb6a78e4",\n    "set": true,\n    "read_only": true\n  },\n  "poll_interval": {\n    "value": "1m",\n    "default": "1m",\n    "set": false,\n    "in_effect": "1m",\n    "applies": "restart"\n  },\n  "refresh_interval": {\n    "value": "6h",\n    "default": "6h",\n    "set": false,\n    "in_effect": "6h",\n    "applies": "restart"\n  },\n  "stale_after": {\n    "value": "36h",\n    "default": "24h",\n    "set": true,\n    "in_effect": "36h",\n    "applies": "now"\n  },\n  "tenant.id": {\n    "value": "01a1c9b3-aafe-7667-8703-470c0e6fb1c9",\n    "set": true,\n    "read_only": true\n  },\n  "update.channel": {\n    "value": "alpha",\n    "default": "alpha",\n    "set": false,\n    "in_effect": "alpha",\n    "applies": "now"\n  }\n}\n','\n',char(10)),'2026-11-11T08:22:37.818Z','2026-11-11T08:22:37.821Z','2026-11-11T08:22:37.821Z',2,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO commands VALUES(2,'01a1c9b3-ac18-74b0-9e45-ba5012856262',NULL,1,1,1,'','setpoint','device:MEQ0000101','{"hash":"a4003acf94e9c73f51fc3c487de1a17dcdead8493e4d06e030efbfed5cf380e4","method":"PUT","path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-11T09:22:37.849675338+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T08:22:37.848Z','2026-11-11T08:22:37.851Z','2026-11-11T08:22:37.851Z',3,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO commands VALUES(3,'01a1c9b3-ac19-7941-b2f1-d5d210e0d9e5',2,1,1,1,'','step','device:0a010a','{"key":"setpoint:device:0a010a","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T09:22:37.849675338+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T08:22:37.849Z','2026-11-11T08:22:37.850Z','2026-11-11T08:22:37.850Z',3,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO commands VALUES(4,'01a1c9b3-ac2f-7145-8716-4a154c12b16e',NULL,1,1,1,'','setpoint','room:1','{"hash":"a668a611353079d05040a7b7117ab0201078669851996d42d0d1d7f761b42b96","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-11T09:22:38.050499437+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T08:22:37.871Z','2026-11-11T08:22:38.051Z','2026-11-11T08:22:38.051Z',3,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO commands VALUES(5,'01a1c9b3-ac30-71d8-be6a-6d2edeeb8123',4,1,1,1,'','step','','{"key":"setpoint:room:1","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T09:22:38.050499437+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T08:22:37.872Z','2026-11-11T08:22:38.051Z','2026-11-11T08:22:38.051Z',3,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO commands VALUES(6,'01a1c9b3-acf5-711c-896a-088ca3d1bd37',NULL,1,1,1,'','room.rename','room:1','{"hash":"0d3ae207663d7e2209c687cfb34abbb665b1c5e3cdee82c501293f945e264aeb","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1","query":""}','acknowledged',200,replace('{\n  "ok": true\n}\n','\n',char(10)),'2026-11-11T08:22:38.069Z','2026-11-11T08:22:38.252Z','2026-11-11T08:22:38.252Z',3,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO commands VALUES(7,'01a1c9b3-acf5-7d76-8742-92aa83b950fe',6,1,1,1,'','step','','{"key":"metadata","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T09:22:38.050499437+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T08:22:38.069Z','2026-11-11T08:22:38.251Z','2026-11-11T08:22:38.251Z',3,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO commands VALUES(8,'01a1c9b3-adbd-738a-b09e-dc582504a006',NULL,1,1,1,'','backup.take','cube:KEQ0000100','{"hash":"100f64b5013f924491098335cbb093208b009a8b0461887abc12f9c6cdb591d1","method":"POST","path":"/api/v1/cubes/KEQ0000100/backups","query":""}','acknowledged',201,replace('{\n  "id": 1,\n  "taken_at": "2026-11-11T08:22:38.457Z",\n  "reason": "manual",\n  "format": "json",\n  "tool": "massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)",\n  "size": 25694,\n  "sha256": "2be9163595a1cc9fea231c6b85939818a7cb6a699c8ba6766aab42fb893c95dc",\n  "user": "owner"\n}\n','\n',char(10)),'2026-11-11T08:22:38.269Z','2026-11-11T08:22:38.460Z','2026-11-11T08:22:38.460Z',3,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
INSERT INTO commands VALUES(9,'01a1c9b3-adbd-7f1e-af09-c8e6e509c689',8,1,1,1,'','step','','{"key":"","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-11T09:22:38.050499437+01:00","max_memory_slots":50},"discarded":false}','2026-11-11T08:22:38.269Z','2026-11-11T08:22:38.453Z','2026-11-11T08:22:38.453Z',3,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
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
INSERT INTO backups VALUES(1,'01a1c9b3-ae79-7d5c-93fc-971c97465953',1,'2026-11-11T08:22:38.457Z','manual','json','massimo v0.13.0-alpha.2 (go1.26.8, linux/amd64)',25694,'2be9163595a1cc9fea231c6b85939818a7cb6a699c8ba6766aab42fb893c95dc',X'1f8b08000000000000ffec5c5d73a2c81abecfafa0b83aa78e63ba411db56a2e1a05c7249a01854cb2b5652120b002edf22126a7e69fedddfeb153881f28cacc183d583b789374bfcdfbbe3c4fd3f07437fcf78620c8b9e67a2676c826014b51d997a79a33927db2499014a06a1f20fc00e110349a14d5a4ebff01b009001937c5d88a9ad9b2e7993626e6a00ce932f8205b33432e53c4bf740ccb54ad5c2f1196e9048b5bd9566b957fc7072bc158239b44940341909ee69af2d2d93dcb0300005c052108d29d8c64557535cf8bec4006107c5cdb26a66b87b21b792261199669f28620be2d0338fe8c6c12bfc5ed1c7f5636b0add9b26f2a6505db6b0769c30d41fcbec970a4606762eadb4467d8f5656ba439f2d8d254b2494c64cbd34a3bc6c05d9e89e1fbb3e6edad2d2fcada9f1fe8b2aa35ebe07679deebf681678cc681ef63671444d98203061587113bb5eacae6bf8d42d3f135779354741ab2bdc4a0d31bae9c13046963c7372262c1a62ad4b4a92abf6e43110469e020f2456f2af064e269fec8d314eca811e86069f9b64dc00b6cfb7002cce04002f48fc4a78ec7a76b6095c2865d1763dbdbf0bb49c354d71d79272bce5cf881ab11d151dbf40e742c994c9eeaae5b2ae596f9fb2f1767fa6b64f8a353feeefffe4b31b44c87f50c8795b443d351092ad3e138c361f5b04398e950c970584b391c2886254fdecc657fca72ab929b1e105f9eaa363715ed401f3888dac6b5ff3a5b067665d5947dec8e7c43736decf9b2bf6db41d8e7aebe1882653b90b2b17c470e362071b8cedd11ed389f10a6ec148a4b0bda4a21108db13ecfa235fb3a3d181daf44082203505afeb6135516fcb8b91a7f9336c3a91378a4eda4c2769ab94930746ce46f1a597bc3ca38bd674541c8ef04c73d621ab47ccb6e904fe92949da4c6187b7ec298b6cd656bae8d664a14bb9e0cae6a8a6c29a378d0203dd90fdce8ff748bd53002a93d30929e2148ba8e2dabd1266eb073da9a361dcd5cacbbb2bdc34b3cb2c519fd96a82576da6c408de27e2ced5b02c737973d0cd49a342077ccdf4a3fe49582195e3f36e9ea495eb37285f5e8e67fee5c61a309ce9f2b5549e79a28fd9e3c92f403cd2b083d57ae574068a8a94e41e9f972bd024a7d23700b46cf96eb15303a71cd82cf73e57a057c6e9ececec2e8c74ba0041a05a33fc36870bee7dc82cf5cf8bcd9affdb695b9beec6bbb92d3c6aa16cfe939816c2565565254821de5282b7e205b1b35da28d753222b5657d48eee937d5f735f47160e77a7d19656cd75b17ba0de329de968c778933cbbfd5987835332ef9e0ba032e602f6e784369300543109f08b4f025099034ee5c40107640c0d54139ee6352bd7cb0aeb5f11a413c4eaaf08d3cf0bc05f11a59f1555197141edd4c7a0ed7ded80d7c6c90f57d9c8533923fff3f2a7c0fe6cd8ffa45029907f07f237fbb53f2429e4c0c74705c551394151e5ca113941832b9013f2fbe504cc90134fd87052aba31b4d01f3d314f4ae062c54c575aa8a134787ac690c0a34e105a631f27f723ab35629a0cf510115e0e7a7ab0aec73516b05f2392e565d64f911c29351bac605a07f104637fbb5ef5540b0715401c17af9e3110554b90605347ebf02aa6428a0d41ed68dfaa9e4a77e8a1595abd03ed9a3cd65969b4f9d1bca5a6ebec8d238455f22d73cb7551684e64de8d9b7551694e64de9b9b755168ce6cde879b755167ce6cde7b995ea2550a2f247e9ac4af51f83d1cd7eedfbb7ffc146c6f6bfbdbd813beb75d7b0fd4f79bf5aad7e4fad1e7c09b05aa8d55f5cad5249a857b6edddebf45d07d9f7c44bcc0953276fbace6fadae003fd7d5ba02fe3cd7eb0af4f35bb12bb0bfd235bb93f522ac6778bdc8ab439176285e05fb7590bfd9af3d836aab67bfb4451d516d956b506deafb555b2d43b51df90acd46bbd5cea4dde00e0549f5562be7b1d772472715126ef33b764967ddc94f9fdda133c6097889919282a7eec4c800e0a2f2ad003e37e956409f976c2b90cf47b215b8e725d70ae4f3116b05eeff6fa996f5321c3c2ad32858a68fc8342af7ada0704c1d585cf38c20ca65a460c797952c89060ebd08c7698ee76b3fb6ae9601f75c5664dfc40e7958ef904dc27703ede2086e3e5e6a994ee2d3a5e4e7e6f6b3c7a5f80bc72500215d02ab5f0956940905805202a0445325288331a04b90a6e912889bad3f2adc6b463e6049d2bb1dc42f3af25703a85fefaceee7bbf99802f788c76d663a6fcf3b7dfcc263a685ba1cef3dd615bb7e8ff829c74cfdd9d8e6f51e8f9936e23b03ca0a5e5ae8b5a5b30f8cbe183e5386f7fcf452939fa0fff2b57b8f78f019457e44c8894314f6da68d16b771f459b9bca4f1c18d35d7de5bfb56cc726dbf5a4643ba963588a0de70a2d186a0b2d50c8de23deef0b121bb50f7b4304daf0ce78e95886da69bc76d955ae3acfb474b637142561d9aecd867dbe31787e1266cf5f85b962320ff25375da6db35c94437b27d7e79d1c44aa8fc71dced66ccb1f3f49af8c1ee5304be6b060933970ea5cee54d778300cc2e387ed3946b93c0ab61428b460ede6f1e9d39ab3567345fa471e334c888668d27848040c11f210c3a2edefcbedf15fb8db969f64b4a50ca07e46b587d786fffc550f5ebeb27eefb53a7d19e24aafd5f8437dbab3501bcdc79dc65ca6a510e5f963d5be84106ae9a81d97ef86511985a885107a64510a50b50474ccb479b6c3de334906a9fb57b6d1e29567f4867ab72112a7ed617dc077a52e23eadc00495d56d48501c36dfe3203beff6571878c959dd9b3b3a2cedd2170cb4e5b6b3bb76717065714270d567d09560bb1ed3db0de1e984ad0e295fb182c869538e14de81b62554c2717ff1544e9739fa5b8a175d82eea025b159fc0f0597a940edb070c3bb484f177e23c4a9c7ff93869b01a31583cdbda03eb75172c140e58c613f970d16db37a6f801629e76da6cb7716427b6d6fb3a9247a8f8cd0fa636befa5190f2b1c7bbfb51f8ad3d6451e6fed27c6f9eef9a4c11ac760852cbb0716d8ef59034eec1e653266bc2bf146863d62dc10c7ecb02709c251c6079ce009bd85c465c5e92cd8374e142f1a270d961c83a5b3cc1e588b07a6d2488235b4069def80f528755f32c1eaf517621d88f7598cd3dcf04d48f69c74cf52a42f7d2ed9732e12270d96120ff088e5f6c082a9318b357489b7b2c012071d466a1d1d6b44bde7312ce0c42f1927f11ab2435ee2eb19603d20a1cf48edb70cb0ce112701d643b31546b7c1e9a0c3f07fa2be88a2720b4d071d613a43bde7b8bcb457307a4471399c0e3a839e837a202eebd341a7a5dbe8c15b96dbd1f10360a11e428c8ec70f5159fff429d24abfdf7cfb5f000000ffffbe06b6975e640000',1,'',1,NULL,'01a1c9b3-aafe-765c-ae2a-1715eb6a78e4');
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
INSERT INTO events VALUES(1,'01a1c9b3-aef4-7c6b-927b-619bd9cbca37',NULL,'2026-11-11T08:22:38.580Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"user.add","set":{"password":true,"role":"viewer"},"subject":{"display_name":"","id":2,"name":"anna","role":"viewer","uuid":"01a1c9b3-aef3-7192-a332-8123c8076aec"},"target":"user:anna","via":"cli"},"time":"2026-11-11T09:22:38.580587836+01:00"}');
INSERT INTO events VALUES(2,'01a1c9b3-af16-78c6-a5ad-7129038d1d9d',NULL,'2026-11-11T08:22:37.611Z','server_started','system',0,'','',NULL,'{"kind":"server_started","source":"system","level":"info","summary":{"version":"v0.13.0-alpha.2"},"time":"2026-11-11T09:22:37.611153039+01:00"}');
INSERT INTO events VALUES(3,'01a1c9b3-af16-7b5d-a584-0de310157a49',1,'2026-11-11T08:22:37.612Z','hello','cube',-4,'','',NULL,'{"kind":"hello","source":"cube","level":"debug","summary":{"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50},"time":"2026-11-11T09:22:37.612717339+01:00"}');
INSERT INTO events VALUES(4,'01a1c9b3-af16-7c12-a57a-9fa043aa47f4',1,'2026-11-11T08:22:37.612Z','metadata','cube',-4,'','',NULL,'{"kind":"metadata","source":"cube","level":"debug","summary":{"devices":7,"rooms":6},"time":"2026-11-11T09:22:37.612845239+01:00"}');
INSERT INTO events VALUES(5,'01a1c9b3-af16-7c6f-bbaa-0d1a163aa496',1,'2026-11-11T08:22:37.613Z','config','cube',-4,'0a0107','KEQ0000100',NULL,'{"kind":"config","device":"0a0107","serial":"KEQ0000100","source":"cube","level":"debug","summary":{"portal_enabled":false,"tz_summer":"BST","tz_winter":"GMT"},"time":"2026-11-11T09:22:37.613472439+01:00"}');
INSERT INTO events VALUES(6,'01a1c9b3-af16-7cd6-ab22-cc115e8691fa',1,'2026-11-11T08:22:37.613Z','config','device',-4,'0a010d','MEQ0000106',NULL,'{"kind":"config","device":"0a010d","serial":"MEQ0000106","source":"device","level":"debug","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"summary":{"comfort_temp":21.5,"eco_temp":16.5,"temp_offset":0},"time":"2026-11-11T09:22:37.613497639+01:00"}');
INSERT INTO events VALUES(7,'01a1c9b3-af16-7d4a-8e90-57eb40f95c92',1,'2026-11-11T08:22:37.613Z','config','device',-4,'0a0108','MEQ0000103',NULL,'{"kind":"config","device":"0a0108","serial":"MEQ0000103","source":"device","level":"debug","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:37.613714839+01:00"}');
INSERT INTO events VALUES(8,'01a1c9b3-af16-7da3-8c20-5f59e3a7e847',1,'2026-11-11T08:22:37.613Z','config','device',-4,'0a0109','MEQ0000102',NULL,'{"kind":"config","device":"0a0109","serial":"MEQ0000102","source":"device","level":"debug","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:37.613741039+01:00"}');
INSERT INTO events VALUES(9,'01a1c9b3-af16-7e12-8c8b-20d12f9bc120',1,'2026-11-11T08:22:37.613Z','config','device',-4,'0a010b','MEQ0000104',NULL,'{"kind":"config","device":"0a010b","serial":"MEQ0000104","source":"device","level":"debug","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:37.613801539+01:00"}');
INSERT INTO events VALUES(10,'01a1c9b3-af16-7e6a-88da-54a50f969556',1,'2026-11-11T08:22:37.613Z','config','device',-4,'0a010a','MEQ0000101',NULL,'{"kind":"config","device":"0a010a","serial":"MEQ0000101","source":"device","level":"debug","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:37.613830739+01:00"}');
INSERT INTO events VALUES(11,'01a1c9b3-af16-7f01-a1ec-28833fa88421',1,'2026-11-11T08:22:37.613Z','config','device',-4,'0a010c','MEQ0000105',NULL,'{"kind":"config","device":"0a010c","serial":"MEQ0000105","source":"device","level":"debug","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-11T09:22:37.613854939+01:00"}');
INSERT INTO events VALUES(12,'01a1c9b3-af17-7017-a575-2dbf25e506d6',1,'2026-11-11T08:22:37.613Z','state','device',0,'0a010d','MEQ0000106',NULL,'{"kind":"state","device":"0a010d","serial":"MEQ0000106","source":"device","level":"info","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"time":"2026-11-11T09:22:37.613919139+01:00"}');
INSERT INTO events VALUES(13,'01a1c9b3-af17-707e-aa8e-3a6b1308eb21',1,'2026-11-11T08:22:37.614Z','state','device',0,'0a0108','MEQ0000103',NULL,'{"kind":"state","device":"0a0108","serial":"MEQ0000103","source":"device","level":"info","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"time":"2026-11-11T09:22:37.614044439+01:00"}');
INSERT INTO events VALUES(14,'01a1c9b3-af17-722a-87a5-f7f756f8dd3e',1,'2026-11-11T08:22:37.614Z','state','device',0,'0a0109','MEQ0000102',NULL,'{"kind":"state","device":"0a0109","serial":"MEQ0000102","source":"device","level":"info","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"time":"2026-11-11T09:22:37.614085539+01:00"}');
INSERT INTO events VALUES(15,'01a1c9b3-af17-72a5-b049-1178aa0b914b',1,'2026-11-11T08:22:37.614Z','state','device',0,'0a010b','MEQ0000104',NULL,'{"kind":"state","device":"0a010b","serial":"MEQ0000104","source":"device","level":"info","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"time":"2026-11-11T09:22:37.614122039+01:00"}');
INSERT INTO events VALUES(16,'01a1c9b3-af17-72f3-b41b-f7164d55c81d',1,'2026-11-11T08:22:37.614Z','state','device',0,'0a010a','MEQ0000101',NULL,'{"kind":"state","device":"0a010a","serial":"MEQ0000101","source":"device","level":"info","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"time":"2026-11-11T09:22:37.614152139+01:00"}');
INSERT INTO events VALUES(17,'01a1c9b3-af17-733c-b1d4-64692b2f8498',1,'2026-11-11T08:22:37.614Z','state','device',0,'0a010c','MEQ0000105',NULL,'{"kind":"state","device":"0a010c","serial":"MEQ0000105","source":"device","level":"info","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"time":"2026-11-11T09:22:37.614181439+01:00"}');
INSERT INTO events VALUES(18,'01a1c9b3-af17-7394-bbf2-5da65c161698',1,'2026-11-11T08:22:37.614Z','state','device',0,'0a1b2c','MEQ0000001',NULL,'{"kind":"state","device":"0a1b2c","serial":"MEQ0000001","source":"device","level":"info","state":{"rf_address":"0a1b2c","type":"shutter_contact","name":"Fenster Kind 1","room_id":5,"serial":"MEQ0000001","reported":true,"state":{"mode":"vacation","window_open":true,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-11T09:22:37.612334839+01:00"},"time":"2026-11-11T09:22:37.614208939+01:00"}');
INSERT INTO events VALUES(19,'01a1c9b3-af17-7400-9614-419da3ad4382',1,'2026-11-11T08:22:37.614Z','connected','cube',0,'','',NULL,'{"kind":"connected","source":"cube","level":"info","summary":{"cause":"start","devices":7,"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50,"rooms":6},"time":"2026-11-11T09:22:37.614238739+01:00"}');
INSERT INTO events VALUES(20,'01a1c9b3-af17-745e-96c6-ae9a085958ce',NULL,'2026-11-11T08:22:37.818Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"settings.update","changes":{"stale_after":{"from":null,"to":"36h"}},"set":{"stale_after":"36h"}},"time":"2026-11-11T09:22:37.818477138+01:00"}');
INSERT INTO events VALUES(21,'01a1c9b3-af17-751d-8c7b-ba332b9f393c',1,'2026-11-11T08:22:37.846Z','change','config',0,'','MEQ0000101',1,'{"kind":"change","serial":"MEQ0000101","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":21.5},"target":"device:MEQ0000101"},"time":"2026-11-11T09:22:37.846235538+01:00"}');
INSERT INTO events VALUES(22,'01a1c9b3-af17-7588-93f4-7fc287b7e10b',1,'2026-11-11T08:22:37.870Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":20},"target":"room:1"},"time":"2026-11-11T09:22:37.870846938+01:00"}');
INSERT INTO events VALUES(23,'01a1c9b3-af17-75f7-abd3-a0ac1084f6b2',1,'2026-11-11T08:22:38.251Z','metadata','cube',0,'','',NULL,'{"kind":"metadata","source":"cube","level":"info","list_changes":[{"what":"room_renamed","room":1,"name":"Fixture room","from":"Wohnzimmer"}],"time":"2026-11-11T09:22:38.251601237+01:00"}');
INSERT INTO events VALUES(24,'01a1c9b3-af17-7649-90c8-d9f31900c825',1,'2026-11-11T08:22:38.068Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"room.rename","set":{"name":"Fixture room"},"target":"room:1"},"time":"2026-11-11T09:22:38.068834537+01:00"}');
INSERT INTO events VALUES(25,'01a1c9b3-af17-769f-94f5-7114dd5d7fd4',1,'2026-11-11T08:22:38.458Z','backup','system',0,'','',1,'{"kind":"backup","source":"system","level":"info","summary":{"bytes":25694,"id":1,"reason":"manual"},"time":"2026-11-11T09:22:38.458762036+01:00"}');
INSERT INTO events VALUES(26,'01a1c9b3-af1e-78a3-bf38-9f8f25258bea',NULL,'2026-11-11T08:22:38.622Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"token.add","set":{"label":"fixture","user":"anna"},"target":"token:2","via":"cli"},"time":"2026-11-11T09:22:38.622083135+01:00"}');
INSERT INTO events VALUES(27,'01a1c9b3-b71a-7519-b132-d5a3cdeac806',NULL,'2026-11-11T08:22:40.665Z','server_stopping','system',0,'','',NULL,'{"kind":"server_stopping","source":"system","level":"info","summary":{"why":"stop"},"time":"2026-11-11T09:22:40.665924629+01:00"}');
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
