-- The store that massimo v0.15.0-alpha.1 (8a536fa) wrote against the fake Cube,
-- made by tools/store-fixture/make.sh on 2026-11-13: a text dump
-- (sqlite3 .dump) that TestMigratesReleasedStores loads and migrates.
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE schema_version (
    version    INTEGER NOT NULL,   -- the migration applied
    min_reader INTEGER NOT NULL,   -- the oldest schema version a binary may be built for and still open this file
    applied_at TEXT    NOT NULL,
    applied_by TEXT    NOT NULL    -- the massimo version string
);
INSERT INTO schema_version VALUES(1,1,'2026-11-13T01:57:28.694Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(2,1,'2026-11-13T01:57:28.699Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(3,1,'2026-11-13T01:57:28.700Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(4,1,'2026-11-13T01:57:28.700Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(5,1,'2026-11-13T01:57:28.703Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(6,1,'2026-11-13T01:57:28.703Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(7,1,'2026-11-13T01:57:28.705Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(8,1,'2026-11-13T01:57:28.705Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(9,1,'2026-11-13T01:57:28.707Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(10,1,'2026-11-13T01:57:28.708Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(11,1,'2026-11-13T01:57:28.709Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(12,1,'2026-11-13T01:57:28.709Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(13,1,'2026-11-13T01:57:28.711Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(14,1,'2026-11-13T01:57:28.717Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(15,1,'2026-11-13T01:57:28.718Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(16,1,'2026-11-13T01:57:28.719Z','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)');
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
INSERT INTO roles VALUES(1,'owner','',1,0,'2026-11-13T01:57:28.693Z','2026-11-13T01:57:28.693Z',NULL,NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO roles VALUES(2,'manager','',1,0,'2026-11-13T01:57:28.693Z','2026-11-13T01:57:28.693Z',NULL,NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO roles VALUES(3,'operator','',1,0,'2026-11-13T01:57:28.693Z','2026-11-13T01:57:28.693Z',NULL,NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO roles VALUES(4,'guest','',1,0,'2026-11-13T01:57:28.693Z','2026-11-13T01:57:28.693Z',NULL,NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO roles VALUES(5,'viewer','',1,0,'2026-11-13T01:57:28.693Z','2026-11-13T01:57:28.693Z',NULL,NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO roles VALUES(6,'integration','',1,0,'2026-11-13T01:57:28.693Z','2026-11-13T01:57:28.693Z',NULL,NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
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
INSERT INTO users VALUES(1,'01a1d29f-c613-7a7b-947e-6ae8541efa75','owner','','',1,0,NULL,NULL,'2026-11-13T01:57:28.723Z','2026-11-13T01:57:28.723Z',NULL,NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374',1,NULL,'');
INSERT INTO users VALUES(2,'01a1d29f-c9d2-704f-85bf-a6e818e3a108','anna','','',5,0,NULL,NULL,'2026-11-13T01:57:29.682Z','2026-11-13T01:57:29.682Z',NULL,NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374',2,X'3c561fd6a41ea252eb0ab2ae2243dbb904321fbd9b49cdcd1d5b56b1e154931d805903d54984ba237595beaab5d974cb911ba7ebcbb291c6673d1d219e20f10a09d96f4e5f336412aa3a558c3d22ba818a7246a9d4bdb2897dcc535c2a8ef5628469bc389654c9bdb1c00552ed95dd88142478c4c8731647144a2cd3db9acc025364c92101ed66e2677c9fde6375a7c81b38a302aaeae606de98408905f12930bd8e386c17d5313d4d105d8cca5187cc273e33e5d2bbdeb752fd606dd55565fa','m=19456,t=2,p=1');
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
INSERT INTO tokens VALUES(1,'01a1d29f-c614-713b-ab80-c08978212bf3',1,'api','3dbd5e608845edf5f2de485da50c0a11655a49b1b5a2188168036453ab3f1b64','MASSIMO_API_TOKEN','2026-11-13T01:57:28.724Z',NULL,'2026-11-13T01:57:28.891Z','127.0.0.1',NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO tokens VALUES(2,'01a1d29f-c9f3-7a0e-830d-3dc7a6ade9f1',2,'api','2d1e3a8af86fd6ecd5cde8c3331b630b79265ea08f16b728a9ce566f314a7209','fixture','2026-11-13T01:57:29.715Z',NULL,'2026-11-13T01:57:29.745Z','127.0.0.1',NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
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
INSERT INTO settings VALUES('node.id','"01a1d29f-c5f6-7037-9335-489186073374"','2026-11-13T01:57:28.694Z',NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO settings VALUES('tenant.id','"01a1d29f-c5f6-703e-becd-08eaa694d314"','2026-11-13T01:57:28.694Z',NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO settings VALUES('log.server_version','"v0.15.0-alpha.1"','2026-11-13T01:57:28.733Z',NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO settings VALUES('stale_after','"36h"','2026-11-13T01:57:28.929Z',NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
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
INSERT INTO cubes VALUES(1,'01a1d29f-c612-7b73-b74a-030121c6df28','KEQ0000100',1,'Fake Cube','127.0.0.1:43577',0,'','','[]','env','','','','',NULL,NULL,'2026-11-13T01:57:28.722Z','2026-11-13T01:57:28.722Z',NULL,1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
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
INSERT INTO audit VALUES(1,'01a1d29f-c6e3-70a5-b734-5d9b0d60a692','2026-11-13T01:57:28.928Z',1,1,'127.0.0.1','settings.update',NULL,'','{"body":{"stale_after":"36h"},"bytes":22,"method":"PUT","ms":2,"path":"/api/v1/settings","status":200}','ok',1);
INSERT INTO audit VALUES(2,'01a1d29f-c6f6-7541-af87-d81fda42dca3','2026-11-13T01:57:28.947Z',1,1,'127.0.0.1','setpoint',1,'device:MEQ0000101','{"body":{"mode":"manual","temp":21.5},"bytes":32,"method":"PUT","ms":2,"path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","status":200}','ok',2);
INSERT INTO audit VALUES(3,'01a1d29f-c7bf-75ad-997e-4df9c3b25cad','2026-11-13T01:57:28.962Z',1,1,'127.0.0.1','setpoint',1,'room:1','{"body":{"mode":"manual","temp":20},"bytes":30,"method":"PUT","ms":188,"path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","status":200}','ok',4);
INSERT INTO audit VALUES(4,'01a1d29f-c889-702b-be37-b9dfe550d0ce','2026-11-13T01:57:29.169Z',1,1,'127.0.0.1','room.rename',1,'room:1','{"body":{"name":"Fixture room"},"bytes":24,"method":"PUT","ms":183,"path":"/api/v1/cubes/KEQ0000100/rooms/1","status":200}','ok',6);
INSERT INTO audit VALUES(5,'01a1d29f-c958-7449-ae8e-67b948927c2d','2026-11-13T01:57:29.370Z',1,1,'127.0.0.1','backup.take',1,'cube:KEQ0000100','{"body":{},"bytes":2,"method":"POST","ms":190,"path":"/api/v1/cubes/KEQ0000100/backups","status":201}','ok',8);
INSERT INTO audit VALUES(6,'01a1d29f-c9d2-77c4-b9c5-ebc8d5ff9578','2026-11-13T01:57:29.682Z',NULL,NULL,'cli','user.add',NULL,'user:anna','{"password":true,"role":"viewer"}','ok',NULL);
INSERT INTO audit VALUES(7,'01a1d29f-c9f8-733b-8880-8b93022d292f','2026-11-13T01:57:29.720Z',NULL,NULL,'cli','token.add',NULL,'token:2','{"label":"fixture","user":"anna"}','ok',NULL);
CREATE TABLE auth_failures (
    key          TEXT    PRIMARY KEY,            -- 'addr:<ip>' or 'user:<name>'
    window_start TEXT    NOT NULL,
    count        INTEGER NOT NULL,
    locked_until TEXT
);
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
INSERT INTO backups VALUES(1,'01a1d29f-c956-766b-b0f5-d2964f8f12a6',1,'2026-11-13T01:57:29.558Z','manual','json','massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)',25694,'69c317083ca28c5028167e5a69dee1fb0d602799b0fe0710e76188fe685e1036',X'1f8b08000000000000ffec5c5d73a2c81abecfafa0b83aa78e63ba418d5a35178d8263129301854cb2b5652120b002edf22126a7e69fedddfeb153881f28cacc183d581bbc49badfe67d5f9ea76978ba1bfe7b4510e44c733d133b649380a5a8eccb13cd19ca3ed924480a50b54f107e82f40050cdea4d936afc07c0260064dc14632b6a66cb9e67da98988132ac96c127d99a1a721912ffd2312c53b572bd4458a613ccaf655bad55fe1d1fac04238d6c12510e04417a9a6bca0b67772c0f00007019842048773c9455d5d53c2fb203194070b3b28d4dd70e6537f244c2322cd3e415417c5f0470fc29d9247e8bdb39feb46c605bb365df54ca0ab6570ed2862b82f87d9de150c1ced8d437894eb1ebcbd65073e491a5a9649318cb96a795b68c81bb3813c3f7a7cdeb6b5b9e97b53f3fd165556bd6c1f5e2bc57ed03cf188e02dfc7ce3088b2057b0c2a0e23766ad5a5cd7f1b86a6e36bee3aa9e834647b8141a737583a2708d2c68e6f44c4827555a86913557edd842208d2c041e48b5e57e0f1d8d3fca1a729d85123d0c1c2f27d938017d8f6fe0498fe9e04e89f894f1d8e4fd7c0328535bb2ec6b6b7e6779d86a9ae3af256569c39f7035723a2a336e9ede95832993cd56db754ca2df3f75f2ecef4d7c8f047a7fcddfdfd976268990eeb190e2b6987a6a31254a6c35186c3ea7e8730d3a192e1b09672d8570c4b1ebf998bfe94e55625d73d20be3c556d662ada9e3eb017b5b56bff75ba08eccaaa29fbd81dfa86e6dad8f3657fd368331cf556c3114da67217962e88c1dac5163618dbc31da613e315dc809148617349452310b6c7d8f587be6647a303b5ee8104416a0a5ed5c36aa2de96e7434ff3a7d874226f149db4994ed25629270f8c9c0de34b2f79794617ade9a8381ce2a9e6ac42560f986dd309fc05295b498d30f6fc84316d9bc9d64c1b4e9528763d195cd514d95286f1a0417ab21fb8d1ffe916cb6104523b60243d4390741d5b96a34ddc60ebb4356d329cba5877657b8b9778648b33fa2d514b6cb559831ac5bd29ed5a02c737173d0cd49a3420b7ccdf4b3fe59582195e6f9a74f528af59b9c27a74f33f75aeb0d104a7cf95aaa4734d947e4f1e49fa81e615849e2ad70b2034d454a7a0f474b95e00a5be11b805a327cbf502181dbb66c1e7a972bd003ed74f672761f4e61c288146c1e8af301a9cee39b7e033173eaf766bbf6f64ae2ffbdab6e4b4b1aac5737a4e205b4999951495604b39ca8a1fc8d65a8d36caf594c88ad515b5a5fb64dfd7dcd7a185c3ed69b48555735deceea9b74c6732dc325e25cf6e77d661ef94ccbbe702a88cb980dd39a1f52400554c027cf049002a73c0a91c39e0808ca1816ac2e3bc66e57a5e61fd11413a42ac7e44987e5d007e44947e555465c405b5631f8336f7b53d5e1b473f5c65234fe58cfcafcb9f02fb9361ff8b42a540fe1dc85fedd6fe94a490031f1f141407e50445952b07e4040d2e404ec8ef971330434e3c61c349ad8eae3505cc4f53d0db1ab0501597a92a8e1c1db2a63128d0846798c6c8ffc9e9c45aa5803e470554809f9fae2ab0cf45ad15c8e7b8587596e547088f46e9121780fe41185dedd6be5701c1c6410504ebe59b030aa872090a68f47e0554c95040a93dac6bf553c94ffd142b2a17a17db2479bf32c371f3b3794b5dc7c96a5718a3e47ae796eab2c08cd9bd0936fab2c28cd9bd2536fab2c18cd9bd1d36eab2cf8cc9bcf532bd573a044e58fd24995ea3f06a3abdddaf76fff838d8ced7f3b7b03b7d6eb2e61fb9ff27eb55afd915addfb1260b550ab1f5cad5249a897b6cdddebf85d07d9f7c473cc0953476fbace6fadae003fd7d5ba02fe3cd7eb0af4f35bb12bb0bfd035bba3f522ac67783dcbab439176285e05fb38c85fedd69e40b5d5b35fdaa20ea8b6ca25a836f5fdaaad96a1da0e7c8566addd6a27d26e708b82a47aab95f3d86bb9a5930a09b7fe1dbaa4b3eee4c7cfeed019e3043cc74849c163776264007056f956009f9b742ba0cf4bb615c8e723d90adcf3926b05f2f988b502f7ffb754cb7a190e1e9469142cd307641a95fb565038a2f62cae794610e53254b0e3cb4a964403fb5e84e334c7f3b59f5b57cb807b262bb26f6287dcaf77c826e1bb81767604d71f2fb54c27f1e952f24b73f3d9e352fc85e31280902e81e5af042bca980240290150a2a91294c108d02548d37409c4cd561f15ee35231fb024e9dd0ee2e71df99b01d46fb756f7cbed6c44813bc4e3363399b5679d07fcc263a685ba1cef3dd615bb7e87f809c74cfce9c8e6f51e8f9936e23b7dca0a5e5ae8b5a5b3f78c3e1f3c5386f7fcf452939fa0fff2ad7b8778f005457e44c8890314f6da68de6b771f459b9bc84f1c18d15d7de9bfb568c726dbf5a4643ba963588a0d670a2d186a0bcd51c8de21de7f1024366a1ff60608b4e1adf1d2b10cb5d378edb2cb5c759e69e96c6f204ac2a25d9b0d1ff846fff949983e7f13668ac9dccb4fd549b7cd72510eedad5c9fb77210a9073cea70b6665bfee8497a65f42887693287399bcc81536772a7bac28361101edd6fce31cae551b0a540a1056b3b8fcf9f579cb59a4bd26f78cc30211aa071e33e113044c8430c8b36bfafd7877fe1765b7e9cd1963280fa05d5ee5f1bfef3373d78f9c6fabdd7eae465802bbd56e30ff5e9d6426d341b751a3399964294e78f551f2484504b47edb87c3b88ca28442d84d0238b5280aa25a063a6cdb31df68e493248ddbdb28d16af3ca337d4bb0e9138690fea7dbe2b751951e7fa48eab2a22ef4196efd97e9f30f5fe7b7c858da991d3b2beadc2d02d7eca4b5b2733b76a17f4171d260d51760b510dbde01ebed9ea9042d5eb98bc162588913de840743ac8ae9e4e2bf82287d7960296e60edb78bbac056c5273078961ea5fdf63ec30e2c61f483388f12e79f3f4e1aac460c16cfb676c07add060b857d96f1443e9c77dbacdeeba379ca799be9f29db9d05ed9db6c2a89de2323b4fed8d87b69c6c30ac7de6decfbe2b47591c71bfb91717e783e69b046315821cbee8005767b569f13bb07998c19ef4abc91618f1837c4113be849827090f13e2778426f2e7159713a73f68d13c5b3c6498325c760e92cb303d6fc9ea93492600dac7ee707603d4add974cb07a0f73b10ec4bb2cc6696ef026247b4eba6729d2d7072ed973ce12270d96120ff088e576c082a9318b357489b7b2c012fb1d466a1d1c6b44bde7312ce0c4af1927f11ab2035ee2eb1960dd23e18191da6f19609d224e02acfb662b8c6e83937e87e1ff440f228aca2d34e97784c914f59ee3f2c25ec1e811c5e570d2eff47b0eea81b8ac4ffa9d966ea37b6f516e47c7f781857a08313a1edd4765fdf3e7482bfd7ef5fd7f010000ffffc90f714a5e640000',1,'',1,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
CREATE TABLE IF NOT EXISTS "commands" (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,  -- never given out twice
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
INSERT INTO commands VALUES(1,'01a1d29f-c6e0-768c-9289-967974bbcc65',NULL,NULL,1,1,'','settings.update','','{"hash":"a1a56e0ab8f245f264cab562d3430faa07b670437af7d261b6497c8824be7018","method":"PUT","path":"/api/v1/settings","query":""}','acknowledged',200,replace('{\n  "audit.keep_days": {\n    "value": "365",\n    "default": "365",\n    "set": false,\n    "in_effect": "365",\n    "applies": "now"\n  },\n  "auth.password_login": {\n    "value": "true",\n    "default": "true",\n    "set": false,\n    "in_effect": "true",\n    "applies": "now"\n  },\n  "backups.auto_max": {\n    "value": "120",\n    "default": "120",\n    "set": false,\n    "in_effect": "120",\n    "applies": "now"\n  },\n  "backups.auto_min": {\n    "value": "60",\n    "default": "60",\n    "set": false,\n    "in_effect": "60",\n    "applies": "now"\n  },\n  "backups.keep_days_auto": {\n    "value": "90",\n    "default": "90",\n    "set": false,\n    "in_effect": "90",\n    "applies": "now"\n  },\n  "backups.keep_days_manual": {\n    "value": "365",\n    "default": "365",\n    "set": false,\n    "in_effect": "365",\n    "applies": "now"\n  },\n  "backups.keep_newest": {\n    "value": "10",\n    "default": "10",\n    "set": false,\n    "in_effect": "10",\n    "applies": "now"\n  },\n  "commands.keep_days": {\n    "value": "0",\n    "default": "0",\n    "set": false,\n    "in_effect": "0",\n    "applies": "now"\n  },\n  "discover_interval": {\n    "value": "30m",\n    "default": "30m",\n    "set": false,\n    "in_effect": "30m",\n    "applies": "restart"\n  },\n  "log.keep_days": {\n    "value": "30",\n    "default": "30",\n    "set": false,\n    "in_effect": "30",\n    "applies": "now"\n  },\n  "node.id": {\n    "value": "01a1d29f-c5f6-7037-9335-489186073374",\n    "set": true,\n    "read_only": true\n  },\n  "poll_interval": {\n    "value": "1m",\n    "default": "1m",\n    "set": false,\n    "in_effect": "1m",\n    "applies": "restart"\n  },\n  "refresh_interval": {\n    "value": "6h",\n    "default": "6h",\n    "set": false,\n    "in_effect": "6h",\n    "applies": "restart"\n  },\n  "stale_after": {\n    "value": "36h",\n    "default": "24h",\n    "set": true,\n    "in_effect": "36h",\n    "applies": "now"\n  },\n  "tenant.id": {\n    "value": "01a1d29f-c5f6-703e-becd-08eaa694d314",\n    "set": true,\n    "read_only": true\n  },\n  "update.channel": {\n    "value": "alpha",\n    "default": "alpha",\n    "set": false,\n    "in_effect": "alpha",\n    "applies": "now"\n  }\n}\n','\n',char(10)),'2026-11-13T01:57:28.928Z','2026-11-13T01:57:28.932Z','2026-11-13T01:57:28.932Z',2,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO commands VALUES(2,'01a1d29f-c6f3-781f-accc-3a5df095230e',NULL,1,1,1,'','setpoint','device:MEQ0000101','{"hash":"a4003acf94e9c73f51fc3c487de1a17dcdead8493e4d06e030efbfed5cf380e4","method":"PUT","path":"/api/v1/cubes/KEQ0000100/devices/0a010a/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-13T02:57:28.949508318+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-13T01:57:28.947Z','2026-11-13T01:57:28.950Z','2026-11-13T01:57:28.950Z',3,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO commands VALUES(3,'01a1d29f-c6f5-71cf-9acd-bd91c10a49a4',2,1,1,1,'','step','device:0a010a','{"key":"setpoint:device:0a010a","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-13T02:57:28.949508318+01:00","max_memory_slots":50},"discarded":false}','2026-11-13T01:57:28.949Z','2026-11-13T01:57:28.950Z','2026-11-13T01:57:28.950Z',3,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO commands VALUES(4,'01a1d29f-c702-73f1-8892-5853c62f8da1',NULL,1,1,1,'','setpoint','room:1','{"hash":"a668a611353079d05040a7b7117ab0201078669851996d42d0d1d7f761b42b96","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-13T02:57:29.150496012+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-13T01:57:28.962Z','2026-11-13T01:57:29.151Z','2026-11-13T01:57:29.151Z',3,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO commands VALUES(5,'01a1d29f-c702-7c9b-8495-644f499877d3',4,1,1,1,'','step','','{"key":"setpoint:room:1","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-13T02:57:29.150496012+01:00","max_memory_slots":50},"discarded":false}','2026-11-13T01:57:28.962Z','2026-11-13T01:57:29.151Z','2026-11-13T01:57:29.151Z',3,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO commands VALUES(6,'01a1d29f-c7d1-73f9-b05b-0f33f3cd7b84',NULL,1,1,1,'','room.rename','room:1','{"hash":"0d3ae207663d7e2209c687cfb34abbb665b1c5e3cdee82c501293f945e264aeb","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/1","query":""}','acknowledged',200,replace('{\n  "ok": true\n}\n','\n',char(10)),'2026-11-13T01:57:29.169Z','2026-11-13T01:57:29.353Z','2026-11-13T01:57:29.353Z',3,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO commands VALUES(7,'01a1d29f-c7d1-7eda-b5c3-e60b3cb027df',6,1,1,1,'','step','','{"key":"metadata","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-13T02:57:29.150496012+01:00","max_memory_slots":50},"discarded":false}','2026-11-13T01:57:29.169Z','2026-11-13T01:57:29.352Z','2026-11-13T01:57:29.352Z',3,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO commands VALUES(8,'01a1d29f-c89a-7696-a456-922a22eab173',NULL,1,1,1,'','backup.take','cube:KEQ0000100','{"hash":"100f64b5013f924491098335cbb093208b009a8b0461887abc12f9c6cdb591d1","method":"POST","path":"/api/v1/cubes/KEQ0000100/backups","query":""}','acknowledged',201,replace('{\n  "id": 1,\n  "taken_at": "2026-11-13T01:57:29.558Z",\n  "reason": "manual",\n  "format": "json",\n  "tool": "massimo v0.15.0-alpha.1 (go1.26.8, linux/amd64)",\n  "size": 25694,\n  "sha256": "69c317083ca28c5028167e5a69dee1fb0d602799b0fe0710e76188fe685e1036",\n  "user": "owner"\n}\n','\n',char(10)),'2026-11-13T01:57:29.370Z','2026-11-13T01:57:29.560Z','2026-11-13T01:57:29.560Z',3,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
INSERT INTO commands VALUES(9,'01a1d29f-c89b-779b-958e-84ec05b45461',8,1,1,1,'','step','','{"key":"","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-13T02:57:29.150496012+01:00","max_memory_slots":50},"discarded":false}','2026-11-13T01:57:29.371Z','2026-11-13T01:57:29.554Z','2026-11-13T01:57:29.554Z',3,NULL,'01a1d29f-c5f6-7037-9335-489186073374');
CREATE TABLE log (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    cube_id    INTEGER REFERENCES cubes (id) ON DELETE CASCADE, -- NULL: the server's
    time       TEXT    NOT NULL,
    kind       TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    source     TEXT    NOT NULL DEFAULT '',       -- device, cube, system, config, …; '' a row of a binary before 0009
    level      INTEGER NOT NULL DEFAULT 0,        -- slog's: -4 debug, 0 info, 4 warn, 8 error
    device     TEXT    NOT NULL DEFAULT '',       -- radio address, '' when none
    serial     TEXT    NOT NULL DEFAULT '',       -- the device's serial number, or the Cube's for its own lines
    user_id    INTEGER REFERENCES users (id),     -- who started it; NULL: the Cube, the server's own jobs, a server without users
    data       TEXT    NOT NULL,                  -- JSON: the event object as GET …/log returns it
    command_id INTEGER REFERENCES commands (id) ON DELETE SET NULL, -- the command the row belongs to
    visibility TEXT    NOT NULL DEFAULT 'read' CHECK (visibility IN ('read', 'audit'))
);
INSERT INTO log VALUES(1,'01a1d29f-c9d3-70ab-bce6-498c22852ee1',NULL,'2026-11-13T01:57:29.682Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"user.add","set":{"password":true,"role":"viewer"},"subject":{"display_name":"","id":2,"name":"anna","role":"viewer","uuid":"01a1d29f-c9d2-704f-85bf-a6e818e3a108"},"target":"user:anna","via":"cli"},"time":"2026-11-13T02:57:29.682843896+01:00"}',NULL,'read');
INSERT INTO log VALUES(2,'01a1d29f-c9f8-7bf6-8c2c-f8e7ee840d9d',NULL,'2026-11-13T01:57:29.720Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"token.add","set":{"label":"fixture","user":"anna"},"target":"token:2","via":"cli"},"time":"2026-11-13T02:57:29.720492394+01:00"}',NULL,'read');
INSERT INTO log VALUES(3,'01a1d29f-ca08-7f1b-bd8a-9c544bf75947',NULL,'2026-11-13T01:57:28.733Z','server_started','system',0,'','',NULL,'{"kind":"server_started","source":"system","level":"info","summary":{"version":"v0.15.0-alpha.1"},"time":"2026-11-13T02:57:28.733810425+01:00"}',NULL,'read');
INSERT INTO log VALUES(4,'01a1d29f-ca09-7130-aebf-47491902e9df',1,'2026-11-13T01:57:28.735Z','hello','cube',-4,'','',NULL,'{"kind":"hello","source":"cube","level":"debug","summary":{"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50},"time":"2026-11-13T02:57:28.735070225+01:00"}',NULL,'read');
INSERT INTO log VALUES(5,'01a1d29f-ca09-718d-afdf-034cbbb0ee5e',1,'2026-11-13T01:57:28.735Z','metadata','cube',-4,'','',NULL,'{"kind":"metadata","source":"cube","level":"debug","summary":{"devices":7,"rooms":6},"time":"2026-11-13T02:57:28.735113525+01:00"}',NULL,'read');
INSERT INTO log VALUES(6,'01a1d29f-ca09-71c2-be86-4fa7e47a3038',1,'2026-11-13T01:57:28.735Z','config','cube',-4,'0a0107','KEQ0000100',NULL,'{"kind":"config","device":"0a0107","serial":"KEQ0000100","source":"cube","level":"debug","summary":{"portal_enabled":false,"tz_summer":"BST","tz_winter":"GMT"},"time":"2026-11-13T02:57:28.735697225+01:00"}',NULL,'read');
INSERT INTO log VALUES(7,'01a1d29f-ca09-71fe-8687-a68c87eaa1c3',1,'2026-11-13T01:57:28.735Z','config','device',-4,'0a010d','MEQ0000106',NULL,'{"kind":"config","device":"0a010d","serial":"MEQ0000106","source":"device","level":"debug","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"summary":{"comfort_temp":21.5,"eco_temp":16.5,"temp_offset":0},"time":"2026-11-13T02:57:28.735771025+01:00"}',NULL,'read');
INSERT INTO log VALUES(8,'01a1d29f-ca09-7263-8487-039c96b29fe9',1,'2026-11-13T01:57:28.735Z','config','device',-4,'0a0108','MEQ0000103',NULL,'{"kind":"config","device":"0a0108","serial":"MEQ0000103","source":"device","level":"debug","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-13T02:57:28.735937025+01:00"}',NULL,'read');
INSERT INTO log VALUES(9,'01a1d29f-ca09-7292-86f6-748f99d3ce3d',1,'2026-11-13T01:57:28.735Z','config','device',-4,'0a0109','MEQ0000102',NULL,'{"kind":"config","device":"0a0109","serial":"MEQ0000102","source":"device","level":"debug","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-13T02:57:28.735962025+01:00"}',NULL,'read');
INSERT INTO log VALUES(10,'01a1d29f-ca09-72c3-82e3-20f5d0580eb0',1,'2026-11-13T01:57:28.736Z','config','device',-4,'0a010b','MEQ0000104',NULL,'{"kind":"config","device":"0a010b","serial":"MEQ0000104","source":"device","level":"debug","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-13T02:57:28.736021825+01:00"}',NULL,'read');
INSERT INTO log VALUES(11,'01a1d29f-ca09-731a-9637-7d59d330ae0f',1,'2026-11-13T01:57:28.736Z','config','device',-4,'0a010a','MEQ0000101',NULL,'{"kind":"config","device":"0a010a","serial":"MEQ0000101","source":"device","level":"debug","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-13T02:57:28.736069725+01:00"}',NULL,'read');
INSERT INTO log VALUES(12,'01a1d29f-ca09-735d-b6a7-a0ab66fc08c6',1,'2026-11-13T01:57:28.736Z','config','device',-4,'0a010c','MEQ0000105',NULL,'{"kind":"config","device":"0a010c","serial":"MEQ0000105","source":"device","level":"debug","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-13T02:57:28.736106225+01:00"}',NULL,'read');
INSERT INTO log VALUES(13,'01a1d29f-ca09-73a0-82ed-27b1fbeedae3',1,'2026-11-13T01:57:28.736Z','state','device',0,'0a010d','MEQ0000106',NULL,'{"kind":"state","device":"0a010d","serial":"MEQ0000106","source":"device","level":"info","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"time":"2026-11-13T02:57:28.736129025+01:00"}',NULL,'read');
INSERT INTO log VALUES(14,'01a1d29f-ca09-73f0-9738-42a184f7baaa',1,'2026-11-13T01:57:28.736Z','state','device',0,'0a0108','MEQ0000103',NULL,'{"kind":"state","device":"0a0108","serial":"MEQ0000103","source":"device","level":"info","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"time":"2026-11-13T02:57:28.736267625+01:00"}',NULL,'read');
INSERT INTO log VALUES(15,'01a1d29f-ca09-7443-9402-782077a0ba61',1,'2026-11-13T01:57:28.736Z','state','device',0,'0a0109','MEQ0000102',NULL,'{"kind":"state","device":"0a0109","serial":"MEQ0000102","source":"device","level":"info","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"time":"2026-11-13T02:57:28.736312825+01:00"}',NULL,'read');
INSERT INTO log VALUES(16,'01a1d29f-ca09-749e-875f-6ce71ebac779',1,'2026-11-13T01:57:28.736Z','state','device',0,'0a010b','MEQ0000104',NULL,'{"kind":"state","device":"0a010b","serial":"MEQ0000104","source":"device","level":"info","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"time":"2026-11-13T02:57:28.736345025+01:00"}',NULL,'read');
INSERT INTO log VALUES(17,'01a1d29f-ca09-74fc-834a-d0b0236186b2',1,'2026-11-13T01:57:28.736Z','state','device',0,'0a010a','MEQ0000101',NULL,'{"kind":"state","device":"0a010a","serial":"MEQ0000101","source":"device","level":"info","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"time":"2026-11-13T02:57:28.736364725+01:00"}',NULL,'read');
INSERT INTO log VALUES(18,'01a1d29f-ca09-7545-9a0e-a24dc901cdb7',1,'2026-11-13T01:57:28.736Z','state','device',0,'0a010c','MEQ0000105',NULL,'{"kind":"state","device":"0a010c","serial":"MEQ0000105","source":"device","level":"info","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"time":"2026-11-13T02:57:28.736383225+01:00"}',NULL,'read');
INSERT INTO log VALUES(19,'01a1d29f-ca09-7587-af3c-19d23287451e',1,'2026-11-13T01:57:28.736Z','state','device',0,'0a1b2c','MEQ0000001',NULL,'{"kind":"state","device":"0a1b2c","serial":"MEQ0000001","source":"device","level":"info","state":{"rf_address":"0a1b2c","type":"shutter_contact","name":"Fenster Kind 1","room_id":5,"serial":"MEQ0000001","reported":true,"state":{"mode":"vacation","window_open":true,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-13T02:57:28.734895025+01:00"},"time":"2026-11-13T02:57:28.736416025+01:00"}',NULL,'read');
INSERT INTO log VALUES(20,'01a1d29f-ca09-761d-9953-17ca7dea8c18',1,'2026-11-13T01:57:28.736Z','connected','cube',0,'','',NULL,'{"kind":"connected","source":"cube","level":"info","summary":{"cause":"start","devices":7,"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50,"rooms":6},"time":"2026-11-13T02:57:28.736483525+01:00"}',NULL,'read');
INSERT INTO log VALUES(21,'01a1d29f-ca09-765f-ba8a-02df0c9ad2ce',NULL,'2026-11-13T01:57:28.928Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"settings.update","changes":{"stale_after":{"from":null,"to":"36h"}},"set":{"stale_after":"36h"}},"time":"2026-11-13T02:57:28.928222319+01:00","command":1}',1,'read');
INSERT INTO log VALUES(22,'01a1d29f-ca09-76f9-84ff-39edc056f345',1,'2026-11-13T01:57:28.949Z','command','queue',-4,'0a010a','MEQ0000101',NULL,'{"kind":"command","device":"0a010a","serial":"MEQ0000101","source":"queue","level":"debug","summary":{"key":"setpoint:device:0a010a","lines":1,"radio":1,"state":"queued","step":3},"time":"2026-11-13T02:57:28.949622918+01:00","command":2}',2,'read');
INSERT INTO log VALUES(23,'01a1d29f-ca09-7749-b8ea-b558f7586e86',1,'2026-11-13T01:57:28.949Z','command','queue',-4,'0a010a','MEQ0000101',NULL,'{"kind":"command","device":"0a010a","serial":"MEQ0000101","source":"queue","level":"debug","summary":{"key":"setpoint:device:0a010a","state":"sent","step":3},"time":"2026-11-13T02:57:28.949975018+01:00","command":2}',2,'read');
INSERT INTO log VALUES(24,'01a1d29f-ca09-77cf-9b96-bcfac0bc1b92',1,'2026-11-13T01:57:28.950Z','command','queue',0,'0a010a','MEQ0000101',NULL,'{"kind":"command","device":"0a010a","serial":"MEQ0000101","source":"queue","level":"info","summary":{"duty_pct":0,"free_slots":49,"key":"setpoint:device:0a010a","state":"acknowledged","step":3},"time":"2026-11-13T02:57:28.950209718+01:00","command":2}',2,'read');
INSERT INTO log VALUES(25,'01a1d29f-ca09-7818-9a35-aace1d752096',1,'2026-11-13T01:57:28.947Z','change','config',0,'','MEQ0000101',1,'{"kind":"change","serial":"MEQ0000101","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":21.5},"target":"device:MEQ0000101"},"time":"2026-11-13T02:57:28.947293218+01:00","command":2}',2,'read');
INSERT INTO log VALUES(26,'01a1d29f-ca09-787e-9310-1156593d5d9b',1,'2026-11-13T01:57:28.963Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"setpoint:room:1","lines":1,"radio":1,"state":"queued","step":5},"time":"2026-11-13T02:57:28.963145818+01:00","command":4}',4,'read');
INSERT INTO log VALUES(27,'01a1d29f-ca09-78e8-b40c-b6f084fe9ac7',1,'2026-11-13T01:57:29.150Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"setpoint:room:1","state":"sent","step":5},"time":"2026-11-13T02:57:29.150913612+01:00","command":4}',4,'read');
INSERT INTO log VALUES(28,'01a1d29f-ca09-793e-aa2f-b4b669edc416',1,'2026-11-13T01:57:29.151Z','command','queue',0,'','',NULL,'{"kind":"command","source":"queue","level":"info","summary":{"duty_pct":0,"free_slots":49,"key":"setpoint:room:1","state":"acknowledged","step":5},"time":"2026-11-13T02:57:29.151248612+01:00","command":4}',4,'read');
INSERT INTO log VALUES(29,'01a1d29f-ca09-7991-b779-fe782df8e1fd',1,'2026-11-13T01:57:28.962Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":20},"target":"room:1"},"time":"2026-11-13T02:57:28.962124218+01:00","command":4}',4,'read');
INSERT INTO log VALUES(30,'01a1d29f-ca09-79d9-bbe4-c31978e0e677',1,'2026-11-13T01:57:29.170Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"metadata","lines":1,"radio":0,"state":"queued","step":7},"time":"2026-11-13T02:57:29.170247012+01:00","command":6}',6,'read');
INSERT INTO log VALUES(31,'01a1d29f-ca09-7a42-b25d-881c85c1aa69',1,'2026-11-13T01:57:29.352Z','metadata','cube',0,'','',NULL,'{"kind":"metadata","source":"cube","level":"info","list_changes":[{"what":"room_renamed","room":1,"name":"Fixture room","from":"Wohnzimmer"}],"time":"2026-11-13T02:57:29.352229806+01:00"}',NULL,'read');
INSERT INTO log VALUES(32,'01a1d29f-ca09-7a83-a7ba-0831e373519d',1,'2026-11-13T01:57:29.352Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"metadata","state":"sent","step":7},"time":"2026-11-13T02:57:29.352616006+01:00","command":6}',6,'read');
INSERT INTO log VALUES(33,'01a1d29f-ca09-7acc-a389-e3fa674b2d02',1,'2026-11-13T01:57:29.352Z','command','queue',0,'','',NULL,'{"kind":"command","source":"queue","level":"info","summary":{"duty_pct":0,"free_slots":49,"key":"metadata","state":"acknowledged","step":7},"time":"2026-11-13T02:57:29.352931906+01:00","command":6}',6,'read');
INSERT INTO log VALUES(34,'01a1d29f-ca09-7b0d-b92d-0da0b4fd1970',1,'2026-11-13T01:57:29.169Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"room.rename","set":{"name":"Fixture room"},"target":"room:1"},"time":"2026-11-13T02:57:29.169061912+01:00","command":6}',6,'read');
INSERT INTO log VALUES(35,'01a1d29f-ca09-7b60-abb6-6502ef3d3cfe',1,'2026-11-13T01:57:29.371Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"","lines":1,"radio":0,"state":"queued","step":9},"time":"2026-11-13T02:57:29.371986705+01:00","command":8}',8,'read');
INSERT INTO log VALUES(36,'01a1d29f-ca09-7bbd-b882-4dbac5b4c93a',1,'2026-11-13T01:57:29.554Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"","state":"sent","step":9},"time":"2026-11-13T02:57:29.554268+01:00","command":8}',8,'read');
INSERT INTO log VALUES(37,'01a1d29f-ca09-7c16-a018-b73e5ebfa779',1,'2026-11-13T01:57:29.554Z','command','queue',0,'','',NULL,'{"kind":"command","source":"queue","level":"info","summary":{"duty_pct":0,"free_slots":49,"key":"","state":"acknowledged","step":9},"time":"2026-11-13T02:57:29.5545719+01:00","command":8}',8,'read');
INSERT INTO log VALUES(38,'01a1d29f-ca09-7c58-97d1-637ce79c2318',1,'2026-11-13T01:57:29.559Z','backup','system',0,'','',1,'{"kind":"backup","source":"system","level":"info","summary":{"bytes":25694,"id":1,"reason":"manual"},"time":"2026-11-13T02:57:29.559312599+01:00"}',NULL,'read');
INSERT INTO log VALUES(39,'01a1d29f-d1e7-75a3-ac73-fef889df08e0',NULL,'2026-11-13T01:57:31.750Z','server_stopping','system',0,'','',NULL,'{"kind":"server_stopping","source":"system","level":"info","summary":{"why":"stop"},"time":"2026-11-13T02:57:31.750739831+01:00"}',NULL,'read');
CREATE TABLE auth_providers (
    id            INTEGER PRIMARY KEY,
    uuid          TEXT    NOT NULL UNIQUE,
    key           TEXT    NOT NULL,                -- in the routes, as a role's key
    name          TEXT    NOT NULL DEFAULT '',     -- on the sign-in button; '' shows the key
    issuer        TEXT    NOT NULL,                -- https://…, its discovery under /.well-known/openid-configuration
    client_id     TEXT    NOT NULL,
    client_secret TEXT    NOT NULL DEFAULT '',
    scopes        TEXT    NOT NULL DEFAULT 'openid email profile',
    auto_create   INTEGER NOT NULL DEFAULT 0,      -- 1: a sign-in without an identity creates the user
    auto_role_id  INTEGER REFERENCES roles (id),   -- the role of a user created so; NULL: viewer
    auto_domains  TEXT    NOT NULL DEFAULT '',     -- the e-mail domains it creates users for, comma-separated; '': any
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id),
    version       INTEGER NOT NULL DEFAULT 1,
    deleted_at    TEXT,
    origin        TEXT    NOT NULL DEFAULT ''
);
CREATE TABLE user_identities (
    id           INTEGER PRIMARY KEY,
    uuid         TEXT    NOT NULL UNIQUE,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id  INTEGER NOT NULL REFERENCES auth_providers (id) ON DELETE CASCADE,
    subject      TEXT,                             -- the provider's sub; NULL until the first sign-in binds it
    email        TEXT    NOT NULL DEFAULT '',      -- lower case: the e-mail assigned, or the one of the sign-in that bound it
    created_at   TEXT    NOT NULL,
    created_by   INTEGER REFERENCES users (id),
    bound_at     TEXT,
    last_used_at TEXT
);
DELETE FROM sqlite_sequence;
INSERT INTO sqlite_sequence VALUES('backups',1);
INSERT INTO sqlite_sequence VALUES('commands',9);
CREATE INDEX tokens_user ON tokens (user_id);
CREATE INDEX audit_time ON audit (time);
CREATE INDEX audit_user ON audit (user_id, time);
CREATE INDEX push_subscriptions_user ON push_subscriptions (user_id);
CREATE INDEX push_subscriptions_token ON push_subscriptions (token_id);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_key ON commands (user_id, idempotency_key);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');
CREATE INDEX audit_command ON audit (command_id) WHERE command_id IS NOT NULL;
CREATE INDEX log_cube ON log (cube_id);
CREATE INDEX log_cube_time ON log (cube_id, time);
CREATE INDEX log_cube_kind ON log (cube_id, kind);
CREATE INDEX log_cube_source ON log (cube_id, source);
CREATE INDEX log_cube_serial ON log (cube_id, serial);
CREATE INDEX log_cube_warn ON log (cube_id) WHERE level >= 4;
CREATE INDEX log_user ON log (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX log_command ON log (command_id) WHERE command_id IS NOT NULL;
CREATE UNIQUE INDEX auth_providers_key ON auth_providers (key) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX user_identities_subject ON user_identities (provider_id, subject) WHERE subject IS NOT NULL;
CREATE UNIQUE INDEX user_identities_email ON user_identities (provider_id, email) WHERE subject IS NULL;
CREATE INDEX user_identities_user ON user_identities (user_id);
CREATE VIEW events AS
SELECT id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data FROM log;
CREATE TRIGGER events_insert INSTEAD OF INSERT ON events
BEGIN
    INSERT INTO log (id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data)
    VALUES (NEW.id, NEW.uuid, NEW.cube_id, NEW.time, NEW.kind, coalesce(NEW.source, ''), coalesce(NEW.level, 0),
            coalesce(NEW.device, ''), coalesce(NEW.serial, ''), NEW.user_id, NEW.data);
END;
CREATE TRIGGER events_update INSTEAD OF UPDATE ON events
BEGIN
    UPDATE log SET uuid = NEW.uuid, cube_id = NEW.cube_id, time = NEW.time, kind = NEW.kind, source = NEW.source,
                   level = NEW.level, device = NEW.device, serial = NEW.serial, user_id = NEW.user_id, data = NEW.data
    WHERE id = OLD.id;
END;
CREATE TRIGGER events_delete INSTEAD OF DELETE ON events
BEGIN
    DELETE FROM log WHERE id = OLD.id;
END;
COMMIT;
