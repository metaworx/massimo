-- The store that massimo v0.16.0-alpha.1 (f0d2cd4) wrote against the fake Cube,
-- made by tools/store-fixture/make.sh on 2026-11-15: a text dump
-- (sqlite3 .dump) that TestMigratesReleasedStores loads and migrates.
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE schema_version (
    version    INTEGER NOT NULL,   -- the migration applied
    min_reader INTEGER NOT NULL,   -- the oldest schema version a binary may be built for and still open this file
    applied_at TEXT    NOT NULL,
    applied_by TEXT    NOT NULL    -- the massimo version string
);
INSERT INTO schema_version VALUES(1,1,'2026-11-15T18:54:12.667Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(2,1,'2026-11-15T18:54:12.674Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(3,1,'2026-11-15T18:54:12.675Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(4,1,'2026-11-15T18:54:12.676Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(5,1,'2026-11-15T18:54:12.678Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(6,1,'2026-11-15T18:54:12.679Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(7,1,'2026-11-15T18:54:12.680Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(8,1,'2026-11-15T18:54:12.680Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(9,1,'2026-11-15T18:54:12.683Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(10,1,'2026-11-15T18:54:12.683Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(11,1,'2026-11-15T18:54:12.684Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(12,1,'2026-11-15T18:54:12.684Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(13,1,'2026-11-15T18:54:12.688Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(14,1,'2026-11-15T18:54:12.694Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(15,1,'2026-11-15T18:54:12.695Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(16,1,'2026-11-15T18:54:12.696Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(17,1,'2026-11-15T18:54:12.697Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(18,1,'2026-11-15T18:54:12.697Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(19,1,'2026-11-15T18:54:12.699Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(20,1,'2026-11-15T18:54:12.701Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
INSERT INTO schema_version VALUES(21,1,'2026-11-15T18:54:12.706Z','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)');
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
INSERT INTO roles VALUES(1,'owner','',1,0,'2026-11-15T18:54:12.667Z','2026-11-15T18:54:12.667Z',NULL,NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO roles VALUES(2,'manager','',1,0,'2026-11-15T18:54:12.667Z','2026-11-15T18:54:12.667Z',NULL,NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO roles VALUES(3,'operator','',1,0,'2026-11-15T18:54:12.667Z','2026-11-15T18:54:12.667Z',NULL,NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO roles VALUES(4,'guest','',1,0,'2026-11-15T18:54:12.667Z','2026-11-15T18:54:12.667Z',NULL,NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO roles VALUES(5,'viewer','',1,0,'2026-11-15T18:54:12.667Z','2026-11-15T18:54:12.667Z',NULL,NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO roles VALUES(6,'integration','',1,0,'2026-11-15T18:54:12.667Z','2026-11-15T18:54:12.705Z',NULL,NULL,2,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
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
INSERT INTO role_capabilities VALUES(2,'preferences');
INSERT INTO role_capabilities VALUES(3,'preferences');
INSERT INTO role_capabilities VALUES(5,'preferences');
INSERT INTO role_capabilities VALUES(2,'restart');
INSERT INTO role_capabilities VALUES(2,'passwords');
INSERT INTO role_capabilities VALUES(6,'read');
INSERT INTO role_capabilities VALUES(6,'preferences');
INSERT INTO role_capabilities VALUES(6,'setpoint');
INSERT INTO role_capabilities VALUES(6,'lock');
INSERT INTO role_capabilities VALUES(6,'configure');
INSERT INTO role_capabilities VALUES(6,'rename');
INSERT INTO role_capabilities VALUES(6,'structure');
INSERT INTO role_capabilities VALUES(6,'pair');
INSERT INTO role_capabilities VALUES(6,'discover');
INSERT INTO role_capabilities VALUES(6,'backup');
INSERT INTO role_capabilities VALUES(6,'restore');
INSERT INTO role_capabilities VALUES(6,'guests');
INSERT INTO role_capabilities VALUES(6,'cubes');
INSERT INTO role_capabilities VALUES(6,'cubes.reboot');
INSERT INTO role_capabilities VALUES(6,'settings');
INSERT INTO role_capabilities VALUES(6,'update');
INSERT INTO role_capabilities VALUES(6,'restart');
INSERT INTO role_capabilities VALUES(6,'reboot');
INSERT INTO role_capabilities VALUES(6,'audit');
INSERT INTO role_capabilities VALUES(2,'integrations');
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
INSERT INTO users VALUES(1,'01a1e08f-56e6-78c9-b074-35bc1be24d37','owner','','',1,0,NULL,NULL,'2026-11-15T18:54:12.710Z','2026-11-15T18:54:12.710Z',NULL,NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792',1,NULL,'');
INSERT INTO users VALUES(2,'01a1e08f-5a9f-7852-924a-31731fd5d9a8','anna','','',5,0,NULL,NULL,'2026-11-15T18:54:13.663Z','2026-11-15T18:54:13.663Z',NULL,NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792',2,X'ea626359c552635495e27cf8d9e8b2ae8bf14d5e3e245490c7b93d8a5447a254e01d0ec7211640245d5d9f94eb8fa1303e04a3817a6867b738fc0bded8d413efbdeff9f76a8f836e1cf2281a03cf7598a5382f546024692231c079fe9d93cef50ca7dec521fcdfe10bf508eb275557ec4facc9b5d1a4a220c5b449491764ed91c0713e097494ab520b3a60599984f3c183f640afcf9f3e98f4c4b40d180dc9eeb7434832f83b4262c76f73d750743b62bf1862f2b6a010466224bf7fd0f914d7','m=19456,t=2,p=1');
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
INSERT INTO tokens VALUES(1,'01a1e08f-56e6-7c86-91b2-1ce1c3d68abd',1,'api','3dbd5e608845edf5f2de485da50c0a11655a49b1b5a2188168036453ab3f1b64','MASSIMO_API_TOKEN','2026-11-15T18:54:12.710Z',NULL,'2026-11-15T18:54:12.859Z','127.0.0.1',NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO tokens VALUES(2,'01a1e08f-5ac5-780b-81d1-fcf74972a95c',2,'api','563f7a926df9c49e4c3d5ae6caf9cc926cef579a17c2677f475947f708bf5854','fixture','2026-11-15T18:54:13.701Z',NULL,'2026-11-15T18:54:13.730Z','127.0.0.1',NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
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
INSERT INTO settings VALUES('node.id','"01a1e08f-56bb-76f9-bfee-95a9f8674792"','2026-11-15T18:54:12.667Z',NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO settings VALUES('tenant.id','"01a1e08f-56bb-7700-84a0-537c109d11bc"','2026-11-15T18:54:12.667Z',NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO settings VALUES('log.server_version','"v0.16.0-alpha.1"','2026-11-15T18:54:12.719Z',NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO settings VALUES('stale_after','"36h"','2026-11-15T18:54:12.898Z',NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
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
INSERT INTO cubes VALUES(1,'01a1e08f-56e5-7af1-abd9-eecd821b7576','KEQ0000100',1,'Fake Cube','127.0.0.1:54283',0,'','','[]','env','','','','',NULL,NULL,'2026-11-15T18:54:12.709Z','2026-11-15T18:54:12.709Z',NULL,1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
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
INSERT INTO audit VALUES(1,'01a1e08f-57a3-7e3f-8757-b274a2782e6d','2026-11-15T18:54:12.897Z',1,1,'127.0.0.1','settings.update',NULL,'','{"body":{"stale_after":"36h"},"bytes":22,"method":"PUT","ms":2,"path":"/api/v1/settings","status":200}','ok',1);
INSERT INTO audit VALUES(2,'01a1e08f-57b5-7330-bf33-2c2603578af9','2026-11-15T18:54:12.914Z',1,1,'127.0.0.1','setpoint',1,'room:1','{"body":{"mode":"manual","temp":21.5},"bytes":32,"method":"PUT","ms":1,"path":"/api/v1/cubes/KEQ0000100/devices/000001/setpoint","status":200}','ok',2);
INSERT INTO audit VALUES(3,'01a1e08f-587e-7794-941d-9df3f4df3130','2026-11-15T18:54:12.930Z',1,1,'127.0.0.1','setpoint',1,'room:5','{"body":{"mode":"manual","temp":20},"bytes":30,"method":"PUT","ms":186,"path":"/api/v1/cubes/KEQ0000100/rooms/5/setpoint","status":200}','ok',4);
INSERT INTO audit VALUES(4,'01a1e08f-5947-76e3-9df2-db1ea65b932c','2026-11-15T18:54:13.131Z',1,1,'127.0.0.1','room.rename',1,'room:5','{"body":{"name":"Fixture room"},"bytes":24,"method":"PUT","ms":186,"path":"/api/v1/cubes/KEQ0000100/rooms/5","status":200}','ok',6);
INSERT INTO audit VALUES(5,'01a1e08f-5a14-7bd7-802d-24844f53a2fa','2026-11-15T18:54:13.334Z',1,1,'127.0.0.1','backup.take',1,'cube:KEQ0000100','{"body":{},"bytes":2,"method":"POST","ms":190,"path":"/api/v1/cubes/KEQ0000100/backups","status":201}','ok',8);
INSERT INTO audit VALUES(6,'01a1e08f-5aa0-7120-af72-a9c409b5feae','2026-11-15T18:54:13.664Z',NULL,NULL,'cli','user.add',NULL,'user:anna','{"password":true,"role":"viewer"}','ok',NULL);
INSERT INTO audit VALUES(7,'01a1e08f-5aca-71c4-aa55-6ad0aa7e6692','2026-11-15T18:54:13.706Z',NULL,NULL,'cli','token.add',NULL,'token:2','{"label":"fixture","user":"anna"}','ok',NULL);
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
INSERT INTO backups VALUES(1,'01a1e08f-5a12-7ecf-965c-7dfc00adff4a',1,'2026-11-15T18:54:13.522Z','manual','json','massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)',25702,'53dbccefadf7d18df4c641cd4021984c7be516345fdbc2c6b80ea9a79694a6e4',X'1f8b08000000000000ffec5cdd929aca16be9fa7a0b83aa78e71ba411db52a178d82711413549c9f5d290b018108b49b1fd1399537db77fbc54e21338aa22471f460edc0cd4cf76ad75a7c5fd3f07537fcf78620c885eab806b6c93a010b61d99366aa3d963cb24e9014a02a1f20fc00cb4358ab974b7548ff07c03a0064d41463336c6649ae6b5898588022ac14c107c99ceb521112ffd2302c529562b5409886ed2f6f254ba994fe1dfd58f6272a5927c21c08827455c790d6ce3aac000000f035084190ce742c298aa3ba6e68071280e0eecd36351c2b909cd013098bb048933704f17d1dc0f6e6649df8236a677bf3a28e2dd5923c432ecad87a739034dc10c4d74d866319db5343db263ac78e279963d59626a6aa9075622a99ae5ad831facefa4c74cf9bd76f6f2d695954fffc401715b55e05b7ebf37e6befbbfa78e27b1eb6c77e982d3860507010b25329bfdabc977160d89eea6c920a4f43b2d618b4f8e1ab7382202d6c7b7a482cd85405aa3a53a4d5361441903af6435ff4a6024fa7aeea8d5d55c6b612820ed696efdb045cdfb20e27c00c0e2440ff4c7cea787cba025e53d8b0eb606cb91b7e376918ca5b47dec9ea01ebf68bb1ce79633bd0ad24327ea2bb4ea98453e6efbf1c9ceaaf96e28f4ef8ebfcfd97acaba90eab290e4b498786ad1054aac3498ac372c221672c3ddf518910fc54b7728adb4ac2ed40d64d69fa13fc28e4a6174497a8a22e0c593dd00f0e62b771edade6ebc08ea41892879db1a7ab8e855d4ff2b68db64312ff3624d16422f7feab0b62b87141c0d849606c8df7f88e8d59700b462c85ed65158e42d89a62c71b7baa158e10d4a61f1204a9caf8ad1e9663f596b41cbbaa37c7861d7aa3e8b8cdb0e3b65231fec3d0d938bafce2976878e11ab68283319eabf65bc8f211b365d8beb7266527a909c6ae1733266d0bc95ca8e3b91cc6aec6832baa2c99f2381a384857f27c27fc3fd9e2752881d41e1871cf10c45d4796d711276ab073daaa3a1bcf1dac3992b5c34b34ba4519fd11ab2576da6c400de3de15f62dbeed19eb1e062a751a903be6ef859ff24ac114af7775ba7c92d7b45c61357c003877aeb05607e7cf952a25738d95bec67f497abeeae6849e2bd72b203450153ba7f47cb95e01a59eee3b39a367cbf50a189d3a46cee7b972bd023e374f676761f4ee1228815acee8af30ea9fef3937e733133e6ff66bbf6f65ae2779eaaee4b4b0a246f37ab62f9971991517956047394ab2e74be6468dd68ad584c88ad415b5a3fb24cf539dd5d8c4c1ee54dadaaa3a0e760ed49b863d1bef186fe267b73feb707062e6dd730154ca5cc0feccd0661280ca27017ef349002a75c0299d38e08094a181aac3d3bca6e57a5961fd3b82748258fd1d61fa7501f83ba2f4aba22a252ea89cfa18b4bdaf1df05a3bf9e12a1d792a63e47f5dfee4d89f0dfb5f142a39f2ef40fe66bff6a72485e47bf8a8a0382a2728aa583a222768700572427abf9c802972e2e0eaf54653c0ec3405bdab017355719daae2c4d1216d1a8302757881698cec9f9cceac5572e833544039f8d9e9aa1cfb4cd45a8e7c868b5517597e84f06494ae7101e81f84d1cd7eed7b1510ac1d5540b05abc3ba2804ad7a08026ef5740a5140594d8c9ba513fa5ecd44fbea27215da277db4b9cc72f3a9734369cbcd17591aa7e84be49ae5b6ca9cd0ac093dfbb6ca9cd2ac293df7b6ca9cd1ac193defb6ca9ccfacf93cb752bd044a54f6289d55a9fe6330bad9af7dfff63f584bd9feb7b7377067bdee1ab6ffc9ef57abe51fa9d5832f019673b5fa9bab552a0ef5ab6d7bf73a7dd741fa3df11273c2d4c99baeb35babcbc1cf74b52e873fcbf5ba1cfdec56ec72ecaf74cdee64bd08ab295e2ff2ea50a81df257c17e1fe46ff66bcfa0daaae92f6d5147545be91a549bf27ed55649516d47be42b3d16e95336937b843415cbd558a59ecb5dcd149b984db1cc72ee9b43bf9e9b33b74ca38012f315252f0d49d1829005c54bee5c06726dd72e8b3926d39f2d948b61cf7ace45a8e7c36622dc7fdff2dd5d25e868347651a058bf411994665be15144ea8038b6baeee87b98c656c7b929c26d1c0a117e138d5763df5e7d6d552e05e48b2e419d8260feb1db24e788eaf5e1cc1cdc74b4dc38e7dba94fc54df7efab8107de5b80020a40be0f528c0923ca500900b001468aa002530017401d2345d0051b3b70f0bf3f5d0072c8cb4760b09f3c709a5fbaa657a9387d1aaa1b11d143c7783bbe537a9257690a03518b1db5c7cba5f3434f69e11962de95107cae3bdd9fe74bf9850a08384806566de7c62091aaf61a6113cf122d5c3931667c5fcf69880ed20c1ecf5476cc03751c00fd1aa69deebcf2d53575ab5559b8de2a03007418fb77be146b1765c1f3f3fde7b13ba079e1e058d1778a6a1b1dda138eaafdb37d9a0876a83a787fefce9b1bf900da62b3d9467ed669b4502669a2ce4c461d80e2df9a6f845b4b899f4c08109ddd60694e93f37d09211c21c603c076a3787de37a9b5d49fedf97cf2004d597b0a73e8c473e0fbbd780e3b382341e8b456cbadff265a3667cfe6c4ee81e7c79d3c3ebef1d6a8bf127f27608609d0104d6bdd588201422e6258b43dbedc1e3f82ddb6c234a52da503e513aa745735efe951f39f1f598f5f9567cf435ce21bb56fcac3bd899a683169d516123d0a509607abf44608a186869a51f97e189651801a08a1cf2cfab80fa852001a669a02db623bcc0ee39d155b6b08f2137a41fc6d80c45973581d08ed519b11356e80466d56d4fa0386dbfc650642efcbf21ee9af7666cfce8a1a778fc02d3b6bbcd9b93d7b7f704571926055d7603510dbdc03eba5cb94fc86207722b01876c4f55ffa3d5d2c8bc9e4a2bf7d71f4a9c752dcd03c6c17b53e5b161fc0f069f47974d83e60d8a1d99ffc20cee711e75d3e4e12ac5a0496c036f6c05aed82858201cbb8a2102cdb4d56e307689970de64da426bd96fbed99b6c2209fe33d36f7cdbdaf924e34189633b5bfba1384d4d14f0d67e629c1f9e4f12ac490456c0b27b6081fd9e35e0c4f6512623c6db23414fb1878cebe2841df2a37eff28e303aeeff6f9e5884b8bd35ab22f9c285e344e122c29024b63993db0965da6548b83353407ad1f80f579d47e4e058bef2dc52a103b698cd3dcf0a51fef39c99e258fbef4b878cfb9489c24587234c02396db030b26c62c56d74682990696386831a3c6d1b146d478976101277e49398955c00e8591504d01ab8bfa3d66d47c4901eb1c71626075eb8d20bc0dce062d46f813f54414961b683668f56773c43f45e5b5bd84d167149583d9a035e06dc483a8accd06ad8666a1aebb2e37c3df0f80897884180d4fba6159fbf831d44b5f6fbeff2f0000ffff6fa3ba9266640000',1,'',1,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
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
INSERT INTO commands VALUES(1,'01a1e08f-57a1-7f01-8562-b124743cb75d',NULL,NULL,1,1,'','settings.update','','{"hash":"a1a56e0ab8f245f264cab562d3430faa07b670437af7d261b6497c8824be7018","method":"PUT","path":"/api/v1/settings","query":""}','acknowledged',200,replace('{\n  "announce": {\n    "value": "auto",\n    "default": "auto",\n    "set": false,\n    "in_effect": "off",\n    "applies": "restart"\n  },\n  "audit.keep_days": {\n    "value": "365",\n    "default": "365",\n    "set": false,\n    "in_effect": "365",\n    "applies": "now"\n  },\n  "auth.password_login": {\n    "value": "true",\n    "default": "true",\n    "set": false,\n    "in_effect": "true",\n    "applies": "now"\n  },\n  "backups.auto_max": {\n    "value": "120",\n    "default": "120",\n    "set": false,\n    "in_effect": "120",\n    "applies": "now"\n  },\n  "backups.auto_min": {\n    "value": "60",\n    "default": "60",\n    "set": false,\n    "in_effect": "60",\n    "applies": "now"\n  },\n  "backups.keep_days_auto": {\n    "value": "90",\n    "default": "90",\n    "set": false,\n    "in_effect": "90",\n    "applies": "now"\n  },\n  "backups.keep_days_manual": {\n    "value": "365",\n    "default": "365",\n    "set": false,\n    "in_effect": "365",\n    "applies": "now"\n  },\n  "backups.keep_newest": {\n    "value": "10",\n    "default": "10",\n    "set": false,\n    "in_effect": "10",\n    "applies": "now"\n  },\n  "commands.keep_days": {\n    "value": "0",\n    "default": "0",\n    "set": false,\n    "in_effect": "0",\n    "applies": "now"\n  },\n  "discover_interval": {\n    "value": "30m",\n    "default": "30m",\n    "set": false,\n    "in_effect": "30m",\n    "applies": "restart"\n  },\n  "duty_wait": {\n    "value": "10m",\n    "default": "10m",\n    "set": false,\n    "in_effect": "10m",\n    "applies": "restart"\n  },\n  "integrations.requests": {\n    "value": "true",\n    "default": "true",\n    "set": false,\n    "in_effect": "true",\n    "applies": "now"\n  },\n  "log.keep_days": {\n    "value": "30",\n    "default": "30",\n    "set": false,\n    "in_effect": "30",\n    "applies": "now"\n  },\n  "node.id": {\n    "value": "01a1e08f-56bb-76f9-bfee-95a9f8674792",\n    "set": true,\n    "read_only": true\n  },\n  "poll_interval": {\n    "value": "10s",\n    "default": "10s",\n    "set": false,\n    "in_effect": "10s",\n    "applies": "restart"\n  },\n  "refresh_interval": {\n    "value": "6h",\n    "default": "6h",\n    "set": false,\n    "in_effect": "6h",\n    "applies": "restart"\n  },\n  "stale_after": {\n    "value": "36h",\n    "default": "50h",\n    "set": true,\n    "in_effect": "36h",\n    "applies": "now"\n  },\n  "stale_after_wall": {\n    "value": "24h",\n    "default": "24h",\n    "set": false,\n    "in_effect": "24h",\n    "applies": "now"\n  },\n  "tenant.id": {\n    "value": "01a1e08f-56bb-7700-84a0-537c109d11bc",\n    "set": true,\n    "read_only": true\n  },\n  "update.channel": {\n    "value": "alpha",\n    "default": "alpha",\n    "set": false,\n    "in_effect": "alpha",\n    "applies": "now"\n  },\n  "window_alert_after": {\n    "value": "10m",\n    "default": "10m",\n    "set": false,\n    "in_effect": "10m",\n    "applies": "now"\n  },\n  "window_warn_after": {\n    "value": "5m",\n    "default": "5m",\n    "set": false,\n    "in_effect": "5m",\n    "applies": "now"\n  }\n}\n','\n',char(10)),'2026-11-15T18:54:12.897Z','2026-11-15T18:54:12.900Z','2026-11-15T18:54:12.900Z',2,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO commands VALUES(2,'01a1e08f-57b3-7399-8af3-2410031ee49f',NULL,1,1,1,'','setpoint','room:1','{"hash":"5f293fe771d4140c4d78bd5ca35c947081e85a71e01c32ef3516aa9c167b428a","method":"PUT","path":"/api/v1/cubes/KEQ0000100/devices/000001/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-15T19:54:12.916700774+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-15T18:54:12.915Z','2026-11-15T18:54:12.917Z','2026-11-15T18:54:12.917Z',3,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO commands VALUES(3,'01a1e08f-57b4-74df-a709-37b7dccf38c9',2,1,1,1,'','step','','{"key":"setpoint:room:1","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"answers":["S:00,0,31"],"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-15T19:54:12.916700774+01:00","max_memory_slots":50},"discarded":false}','2026-11-15T18:54:12.916Z','2026-11-15T18:54:12.916Z','2026-11-15T18:54:12.916Z',3,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO commands VALUES(4,'01a1e08f-57c3-7e08-980b-70d4d7ce0f65',NULL,1,1,1,'','setpoint','room:5','{"hash":"a9a58674ca9751b99aedd555853431c98e615da74c321f416726add642100bb3","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/5/setpoint","query":""}','acknowledged',200,replace('{\n  "budget": {\n    "duty_cycle_pct": 0,\n    "free_memory_slots": 49,\n    "updated_at": "2026-11-15T19:54:13.117816076+01:00",\n    "max_memory_slots": 50\n  },\n  "discarded": false,\n  "ok": true\n}\n','\n',char(10)),'2026-11-15T18:54:12.931Z','2026-11-15T18:54:13.118Z','2026-11-15T18:54:13.118Z',3,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO commands VALUES(5,'01a1e08f-57c5-7718-bf9e-257c24ad9f57',4,1,1,1,'','step','','{"key":"setpoint:room:5","lines":1,"priority":2,"radio":1,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"answers":["S:00,0,31"],"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-15T19:54:13.117816076+01:00","max_memory_slots":50},"discarded":false}','2026-11-15T18:54:12.933Z','2026-11-15T18:54:13.118Z','2026-11-15T18:54:13.118Z',3,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO commands VALUES(6,'01a1e08f-588c-7186-897f-85701949a5fc',NULL,1,1,1,'','room.rename','room:5','{"hash":"7d19666eb667d8055bcb96ccb00bb6094c6e85e3933dcb23c7b62574be0875e0","method":"PUT","path":"/api/v1/cubes/KEQ0000100/rooms/5","query":""}','acknowledged',200,replace('{\n  "ok": true\n}\n','\n',char(10)),'2026-11-15T18:54:13.132Z','2026-11-15T18:54:13.319Z','2026-11-15T18:54:13.319Z',3,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO commands VALUES(7,'01a1e08f-588c-7d23-a426-590fe067f0d6',6,1,1,1,'','step','','{"key":"metadata","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"answers":["A:"],"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-15T19:54:13.117816076+01:00","max_memory_slots":50},"discarded":false}','2026-11-15T18:54:13.132Z','2026-11-15T18:54:13.319Z','2026-11-15T18:54:13.319Z',3,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO commands VALUES(8,'01a1e08f-5956-7a16-ac5f-1aa8c1ce7de2',NULL,1,1,1,'','backup.take','cube:KEQ0000100','{"hash":"100f64b5013f924491098335cbb093208b009a8b0461887abc12f9c6cdb591d1","method":"POST","path":"/api/v1/cubes/KEQ0000100/backups","query":""}','acknowledged',201,replace('{\n  "id": 1,\n  "taken_at": "2026-11-15T18:54:13.522Z",\n  "reason": "manual",\n  "format": "json",\n  "tool": "massimo v0.16.0-alpha.1 (go1.26.8, linux/amd64)",\n  "size": 25702,\n  "sha256": "53dbccefadf7d18df4c641cd4021984c7be516345fdbc2c6b80ea9a79694a6e4",\n  "user": "owner"\n}\n','\n',char(10)),'2026-11-15T18:54:13.334Z','2026-11-15T18:54:13.525Z','2026-11-15T18:54:13.525Z',3,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
INSERT INTO commands VALUES(9,'01a1e08f-5957-75c6-ae7f-88e1c84f0b57',8,1,1,1,'','step','','{"key":"","lines":1,"priority":2,"radio":0,"reconnect":false,"room":0,"wakeup":false}','acknowledged',0,'{"answers":["F:ntp.homematic.com,ntp.homematic.com"],"budget":{"duty_cycle_pct":0,"free_memory_slots":49,"updated_at":"2026-11-15T19:54:13.117816076+01:00","max_memory_slots":50},"discarded":false}','2026-11-15T18:54:13.335Z','2026-11-15T18:54:13.519Z','2026-11-15T18:54:13.519Z',3,NULL,'01a1e08f-56bb-76f9-bfee-95a9f8674792');
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
INSERT INTO log VALUES(1,'01a1e08f-5aa0-7d8c-aeba-54a89422b2c9',NULL,'2026-11-15T18:54:13.664Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"user.add","set":{"password":true,"role":"viewer"},"subject":{"display_name":"","id":2,"name":"anna","role":"viewer","uuid":"01a1e08f-5a9f-7852-924a-31731fd5d9a8"},"target":"user:anna","via":"cli"},"time":"2026-11-15T19:54:13.66456798+01:00"}',NULL,'read');
INSERT INTO log VALUES(2,'01a1e08f-5aca-7c50-81d3-867db18d5654',NULL,'2026-11-15T18:54:13.706Z','change','config',0,'','',NULL,'{"kind":"change","level":"info","source":"config","summary":{"action":"token.add","set":{"label":"fixture","user":"anna"},"target":"token:2","via":"cli"},"time":"2026-11-15T19:54:13.70648618+01:00"}',NULL,'read');
INSERT INTO log VALUES(3,'01a1e08f-5adc-7625-a4b7-83bd061a1fc7',NULL,'2026-11-15T18:54:12.720Z','server_started','system',0,'','',NULL,'{"kind":"server_started","source":"system","level":"info","summary":{"version":"v0.16.0-alpha.1"},"time":"2026-11-15T19:54:12.720253873+01:00"}',NULL,'read');
INSERT INTO log VALUES(4,'01a1e08f-5adc-78fa-9471-e361541a9ca0',1,'2026-11-15T18:54:12.721Z','hello','cube',-4,'','',NULL,'{"kind":"hello","source":"cube","level":"debug","summary":{"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50},"time":"2026-11-15T19:54:12.721652973+01:00"}',NULL,'read');
INSERT INTO log VALUES(5,'01a1e08f-5adc-7994-a155-60536fb8ae7b',1,'2026-11-15T18:54:12.722Z','metadata','cube',-4,'','',NULL,'{"kind":"metadata","source":"cube","level":"debug","summary":{"devices":7,"rooms":6},"time":"2026-11-15T19:54:12.722321473+01:00"}',NULL,'read');
INSERT INTO log VALUES(6,'01a1e08f-5adc-79ee-8df9-0bbbd5bc31e3',1,'2026-11-15T18:54:12.723Z','config','cube',-4,'0a0107','KEQ0000100',NULL,'{"kind":"config","device":"0a0107","serial":"KEQ0000100","source":"cube","level":"debug","summary":{"portal_enabled":false,"tz_summer":"BST","tz_winter":"GMT"},"time":"2026-11-15T19:54:12.723463173+01:00"}',NULL,'read');
INSERT INTO log VALUES(7,'01a1e08f-5adc-7a2c-8319-afd4ae822a78',1,'2026-11-15T18:54:12.723Z','config','device',-4,'0a010d','MEQ0000106',NULL,'{"kind":"config","device":"0a010d","serial":"MEQ0000106","source":"device","level":"debug","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"summary":{"comfort_temp":21.5,"eco_temp":16.5,"temp_offset":0},"time":"2026-11-15T19:54:12.723647373+01:00"}',NULL,'read');
INSERT INTO log VALUES(8,'01a1e08f-5adc-7a63-a869-30ed76b601bc',1,'2026-11-15T18:54:12.723Z','config','device',-4,'0a0108','MEQ0000103',NULL,'{"kind":"config","device":"0a0108","serial":"MEQ0000103","source":"device","level":"debug","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-15T19:54:12.723888473+01:00"}',NULL,'read');
INSERT INTO log VALUES(9,'01a1e08f-5adc-7adc-aa68-dec0a8edb573',1,'2026-11-15T18:54:12.724Z','config','device',-4,'0a0109','MEQ0000102',NULL,'{"kind":"config","device":"0a0109","serial":"MEQ0000102","source":"device","level":"debug","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-15T19:54:12.724066173+01:00"}',NULL,'read');
INSERT INTO log VALUES(10,'01a1e08f-5adc-7b08-97d3-987280b6f624',1,'2026-11-15T18:54:12.724Z','config','device',-4,'0a010b','MEQ0000104',NULL,'{"kind":"config","device":"0a010b","serial":"MEQ0000104","source":"device","level":"debug","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-15T19:54:12.724349473+01:00"}',NULL,'read');
INSERT INTO log VALUES(11,'01a1e08f-5adc-7b48-9847-7574d7aebf06',1,'2026-11-15T18:54:12.724Z','config','device',-4,'0a010a','MEQ0000101',NULL,'{"kind":"config","device":"0a010a","serial":"MEQ0000101","source":"device","level":"debug","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-15T19:54:12.724652373+01:00"}',NULL,'read');
INSERT INTO log VALUES(12,'01a1e08f-5adc-7bf0-a9ef-8beec2b6649c',1,'2026-11-15T18:54:12.724Z','config','device',-4,'0a010c','MEQ0000105',NULL,'{"kind":"config","device":"0a010c","serial":"MEQ0000105","source":"device","level":"debug","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"summary":{"comfort_temp":22,"eco_temp":15,"temp_offset":0},"time":"2026-11-15T19:54:12.724896373+01:00"}',NULL,'read');
INSERT INTO log VALUES(13,'01a1e08f-5adc-7c27-aed8-ae333aa42b09',1,'2026-11-15T18:54:12.725Z','state','device',0,'0a010d','MEQ0000106',NULL,'{"kind":"state","device":"0a010d","serial":"MEQ0000106","source":"device","level":"info","state":{"rf_address":"0a010d","type":"radiator_thermostat","name":"Radiator Schlafzimmer","room_id":6,"serial":"MEQ0000106","reported":true,"state":{"mode":"auto","setpoint":21,"actual_temp":21.3,"valve_pct":20,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"time":"2026-11-15T19:54:12.725175873+01:00"}',NULL,'read');
INSERT INTO log VALUES(14,'01a1e08f-5adc-7c58-9e8a-7d8403ce6329',1,'2026-11-15T18:54:12.727Z','state','device',0,'0a0108','MEQ0000103',NULL,'{"kind":"state","device":"0a0108","serial":"MEQ0000103","source":"device","level":"info","state":{"rf_address":"0a0108","type":"radiator_thermostat","name":"Radiator Thermostat 1","room_id":3,"serial":"MEQ0000103","reported":true,"state":{"mode":"manual","setpoint":20.5,"actual_temp":19.8,"valve_pct":25,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"time":"2026-11-15T19:54:12.727602673+01:00"}',NULL,'read');
INSERT INTO log VALUES(15,'01a1e08f-5adc-7c85-a2ea-a4f00b15dfd2',1,'2026-11-15T18:54:12.728Z','state','device',0,'0a0109','MEQ0000102',NULL,'{"kind":"state","device":"0a0109","serial":"MEQ0000102","source":"device","level":"info","state":{"rf_address":"0a0109","type":"radiator_thermostat","name":"Radiator Büro","room_id":2,"serial":"MEQ0000102","reported":true,"state":{"mode":"auto","setpoint":20,"actual_temp":22.4,"valve_pct":30,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"time":"2026-11-15T19:54:12.728000873+01:00"}',NULL,'read');
INSERT INTO log VALUES(16,'01a1e08f-5adc-7cfd-8f4c-24ec1a17de1f',1,'2026-11-15T18:54:12.728Z','state','device',0,'0a010b','MEQ0000104',NULL,'{"kind":"state","device":"0a010b","serial":"MEQ0000104","source":"device","level":"info","state":{"rf_address":"0a010b","type":"radiator_thermostat","name":"Radiator Kind 2","room_id":4,"serial":"MEQ0000104","reported":true,"state":{"mode":"manual","setpoint":19.5,"actual_temp":20.5,"valve_pct":35,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"time":"2026-11-15T19:54:12.728317873+01:00"}',NULL,'read');
INSERT INTO log VALUES(17,'01a1e08f-5adc-7d2b-87e0-a13399187f51',1,'2026-11-15T18:54:12.728Z','state','device',0,'0a010a','MEQ0000101',NULL,'{"kind":"state","device":"0a010a","serial":"MEQ0000101","source":"device","level":"info","state":{"rf_address":"0a010a","type":"radiator_thermostat","name":"Radiator Wohnzimmer","room_id":1,"serial":"MEQ0000101","reported":true,"state":{"mode":"auto","setpoint":19,"actual_temp":18.7,"valve_pct":40,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"time":"2026-11-15T19:54:12.728594973+01:00"}',NULL,'read');
INSERT INTO log VALUES(18,'01a1e08f-5adc-7db6-ba7e-649f530e42bf',1,'2026-11-15T18:54:12.728Z','state','device',0,'0a010c','MEQ0000105',NULL,'{"kind":"state","device":"0a010c","serial":"MEQ0000105","source":"device","level":"info","state":{"rf_address":"0a010c","type":"radiator_thermostat","name":"Radiator Kind 1","room_id":5,"serial":"MEQ0000105","reported":true,"state":{"mode":"manual","setpoint":18.5,"actual_temp":19.2,"valve_pct":45,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","battery":"ok"},"time":"2026-11-15T19:54:12.728915973+01:00"}',NULL,'read');
INSERT INTO log VALUES(19,'01a1e08f-5adc-7de2-af5b-e10ba950749d',1,'2026-11-15T18:54:12.729Z','state','device',0,'0a1b2c','MEQ0000001',NULL,'{"kind":"state","device":"0a1b2c","serial":"MEQ0000001","source":"device","level":"info","state":{"rf_address":"0a1b2c","type":"shutter_contact","name":"Fenster Kind 1","room_id":5,"serial":"MEQ0000001","reported":true,"state":{"mode":"vacation","window_open":true,"battery_low":false,"error":false,"link_error":false},"last_seen":"2026-11-15T19:54:12.721431773+01:00","window":{"open_since":"2026-11-15T19:54:12.721431773+01:00","warn_at":"2026-11-15T19:59:12.721431773+01:00","alert_at":"2026-11-15T20:04:12.721431773+01:00"},"battery":"ok"},"time":"2026-11-15T19:54:12.729183473+01:00"}',NULL,'read');
INSERT INTO log VALUES(20,'01a1e08f-5adc-7e17-93ce-b48920dcdde9',1,'2026-11-15T18:54:12.730Z','connected','cube',0,'','',NULL,'{"kind":"connected","source":"cube","level":"info","summary":{"cause":"start","devices":7,"duty_cycle_pct":0,"firmware":"1.1.3","free_memory_slots":50,"rooms":6},"time":"2026-11-15T19:54:12.730148173+01:00"}',NULL,'read');
INSERT INTO log VALUES(21,'01a1e08f-5adc-7e47-b83a-bc6ab0a45342',NULL,'2026-11-15T18:54:12.897Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"settings.update","changes":{"stale_after":{"from":null,"to":"36h"}},"set":{"stale_after":"36h"}},"time":"2026-11-15T19:54:12.897796674+01:00","command":1}',1,'read');
INSERT INTO log VALUES(22,'01a1e08f-5adc-7ec3-900b-a4c604361fbc',1,'2026-11-15T18:54:12.916Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"setpoint:room:1","lines":1,"radio":1,"state":"queued","step":3},"time":"2026-11-15T19:54:12.916646074+01:00","command":2}',2,'read');
INSERT INTO log VALUES(23,'01a1e08f-5add-701b-aade-ef8a6c2a1bdb',1,'2026-11-15T18:54:12.916Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"setpoint:room:1","state":"sent","step":3},"time":"2026-11-15T19:54:12.916956474+01:00","command":2}',2,'read');
INSERT INTO log VALUES(24,'01a1e08f-5add-70b2-97fb-827fc9e5c02a',1,'2026-11-15T18:54:12.917Z','command','queue',0,'','',NULL,'{"kind":"command","source":"queue","level":"info","summary":{"duty_pct":0,"free_slots":49,"key":"setpoint:room:1","state":"acknowledged","step":3},"time":"2026-11-15T19:54:12.917148674+01:00","command":2}',2,'read');
INSERT INTO log VALUES(25,'01a1e08f-5add-70fb-979b-398b28512c4b',1,'2026-11-15T18:54:12.914Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":21.5},"target":"room:1"},"time":"2026-11-15T19:54:12.914772774+01:00","command":2}',2,'read');
INSERT INTO log VALUES(26,'01a1e08f-5add-7136-bb5a-517a8c383808',1,'2026-11-15T18:54:12.933Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"setpoint:room:5","lines":1,"radio":1,"state":"queued","step":5},"time":"2026-11-15T19:54:12.933692474+01:00","command":4}',4,'read');
INSERT INTO log VALUES(27,'01a1e08f-5add-7168-9386-1e32509c5c4f',1,'2026-11-15T18:54:13.118Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"setpoint:room:5","state":"sent","step":5},"time":"2026-11-15T19:54:13.118143376+01:00","command":4}',4,'read');
INSERT INTO log VALUES(28,'01a1e08f-5add-71e5-a954-ac3ea9d94ca7',1,'2026-11-15T18:54:13.118Z','command','queue',0,'','',NULL,'{"kind":"command","source":"queue","level":"info","summary":{"duty_pct":0,"free_slots":49,"key":"setpoint:room:5","state":"acknowledged","step":5},"time":"2026-11-15T19:54:13.118421276+01:00","command":4}',4,'read');
INSERT INTO log VALUES(29,'01a1e08f-5add-7212-9099-55cd1153fbf5',1,'2026-11-15T18:54:12.930Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"setpoint","set":{"mode":"manual","temp":20},"target":"room:5"},"time":"2026-11-15T19:54:12.930992674+01:00","command":4}',4,'read');
INSERT INTO log VALUES(30,'01a1e08f-5add-728d-859b-ba5dd70f6dd7',1,'2026-11-15T18:54:13.133Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"metadata","lines":1,"radio":0,"state":"queued","step":7},"time":"2026-11-15T19:54:13.133102776+01:00","command":6}',6,'read');
INSERT INTO log VALUES(31,'01a1e08f-5add-72ba-a3d9-a68cce52754a',1,'2026-11-15T18:54:13.318Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"metadata","state":"sent","step":7},"time":"2026-11-15T19:54:13.318823277+01:00","command":6}',6,'read');
INSERT INTO log VALUES(32,'01a1e08f-5add-72e5-a398-6299fc9c6a62',1,'2026-11-15T18:54:13.319Z','metadata','cube',0,'','',NULL,'{"kind":"metadata","source":"cube","level":"info","list_changes":[{"what":"room_renamed","room":5,"name":"Fixture room","from":"Kind 1"}],"time":"2026-11-15T19:54:13.319100677+01:00"}',NULL,'read');
INSERT INTO log VALUES(33,'01a1e08f-5add-730f-97be-e981ccff68ea',1,'2026-11-15T18:54:13.319Z','command','queue',0,'','',NULL,'{"kind":"command","source":"queue","level":"info","summary":{"duty_pct":0,"free_slots":49,"key":"metadata","state":"acknowledged","step":7},"time":"2026-11-15T19:54:13.319399677+01:00","command":6}',6,'read');
INSERT INTO log VALUES(34,'01a1e08f-5add-733c-84ca-08512d51165e',1,'2026-11-15T18:54:13.131Z','change','config',0,'','',1,'{"kind":"change","source":"config","level":"info","summary":{"action":"room.rename","set":{"name":"Fixture room"},"target":"room:5"},"time":"2026-11-15T19:54:13.131888776+01:00","command":6}',6,'read');
INSERT INTO log VALUES(35,'01a1e08f-5add-7369-a257-08a9338020a6',1,'2026-11-15T18:54:13.335Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"","lines":1,"radio":0,"state":"queued","step":9},"time":"2026-11-15T19:54:13.335631077+01:00","command":8}',8,'read');
INSERT INTO log VALUES(36,'01a1e08f-5add-7396-b2bf-c56f6495620d',1,'2026-11-15T18:54:13.519Z','command','queue',-4,'','',NULL,'{"kind":"command","source":"queue","level":"debug","summary":{"key":"","state":"sent","step":9},"time":"2026-11-15T19:54:13.519477979+01:00","command":8}',8,'read');
INSERT INTO log VALUES(37,'01a1e08f-5add-73cc-80e0-4c9b0e51acef',1,'2026-11-15T18:54:13.519Z','command','queue',0,'','',NULL,'{"kind":"command","source":"queue","level":"info","summary":{"duty_pct":0,"free_slots":49,"key":"","state":"acknowledged","step":9},"time":"2026-11-15T19:54:13.519679179+01:00","command":8}',8,'read');
INSERT INTO log VALUES(38,'01a1e08f-5add-73fa-b800-4511220093de',1,'2026-11-15T18:54:13.523Z','backup','system',0,'','',1,'{"kind":"backup","source":"system","level":"info","summary":{"bytes":25702,"id":1,"reason":"manual"},"time":"2026-11-15T19:54:13.523721679+01:00"}',NULL,'read');
INSERT INTO log VALUES(39,'01a1e08f-62ba-7512-b4cb-a597260574ed',NULL,'2026-11-15T18:54:15.737Z','server_stopping','system',0,'','',NULL,'{"kind":"server_stopping","source":"system","level":"info","summary":{"why":"stop"},"time":"2026-11-15T19:54:15.737871296+01:00"}',NULL,'read');
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
, groups_claim TEXT NOT NULL DEFAULT 'groups', allowed_groups TEXT NOT NULL DEFAULT '[]');
CREATE TABLE rooms (
    id          INTEGER PRIMARY KEY,
    uuid        TEXT    NOT NULL UNIQUE,            -- the room's uid in the API (room_uid)
    cube_id     INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    room_id     INTEGER NOT NULL,                   -- the Cube's room number, 1..254
    temp_method TEXT    NOT NULL DEFAULT 'median' CHECK (temp_method IN ('median', 'mean')),
    rf_address  TEXT    NOT NULL DEFAULT '',        -- the virtual wall thermostat's radio address, 6 hex digits; '' its room number
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL,
    updated_by  INTEGER REFERENCES users (id),
    version     INTEGER NOT NULL DEFAULT 1,
    deleted_at  TEXT,
    origin      TEXT    NOT NULL DEFAULT ''
);
INSERT INTO rooms VALUES(1,'01a1e08f-56f1-7cd5-9ae1-126f2cfe0e4e',1,1,'median','','2026-11-15T18:54:12.721Z','2026-11-15T18:54:12.721Z',NULL,1,NULL,'');
INSERT INTO rooms VALUES(2,'01a1e08f-56f1-7da4-88f2-2c83742f274d',1,3,'median','','2026-11-15T18:54:12.721Z','2026-11-15T18:54:12.721Z',NULL,1,NULL,'');
INSERT INTO rooms VALUES(3,'01a1e08f-56f1-7e5c-a442-4ecb4073da19',1,2,'median','','2026-11-15T18:54:12.721Z','2026-11-15T18:54:12.721Z',NULL,1,NULL,'');
INSERT INTO rooms VALUES(4,'01a1e08f-56f1-7f1e-b613-6a2d00c6a27f',1,5,'median','','2026-11-15T18:54:12.721Z','2026-11-15T18:54:12.721Z',NULL,1,NULL,'');
INSERT INTO rooms VALUES(5,'01a1e08f-56f2-7096-9b8b-dd34d983b17e',1,4,'median','','2026-11-15T18:54:12.721Z','2026-11-15T18:54:12.721Z',NULL,1,NULL,'');
INSERT INTO rooms VALUES(6,'01a1e08f-56f2-713b-a51f-dc69bff12403',1,6,'median','','2026-11-15T18:54:12.721Z','2026-11-15T18:54:12.721Z',NULL,1,NULL,'');
CREATE TABLE cube_options (
    cube_id       INTEGER PRIMARY KEY REFERENCES cubes (id) ON DELETE CASCADE,
    proxy_virtual INTEGER NOT NULL DEFAULT 0,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id)
);
CREATE TABLE integrations (
    id               INTEGER PRIMARY KEY,
    uuid             TEXT    NOT NULL UNIQUE,
    kind             TEXT    NOT NULL CHECK (kind IN ('homeassistant')),
    key              TEXT    NOT NULL,                -- in user names and routes: lower case, from the name
    name             TEXT    NOT NULL DEFAULT '',     -- the installation's name (Home Assistant's location_name)
    instance_id      TEXT    NOT NULL DEFAULT '',     -- the installation id as accepted by the owner (32 hex)
    pending_instance TEXT    NOT NULL DEFAULT '',     -- an installation id seen and not accepted yet
    url              TEXT    NOT NULL DEFAULT '',     -- the installation's address, for display
    app_version      TEXT    NOT NULL DEFAULT '',     -- the installation's version, for display
    max_caps         TEXT    NOT NULL DEFAULT '[]',   -- JSON list: the key's maximum, within the role integration
    user_id          INTEGER REFERENCES users (id),   -- the integration's own user, holding its key
    system_user_id   INTEGER REFERENCES users (id),   -- the user "system": calls no user started
    state            TEXT    NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'active', 'disabled', 'refused')),
    request_code     TEXT    NOT NULL DEFAULT '',     -- the short code both sides show while pending
    request_expires  TEXT,                            -- a pending request's end
    request_remote   TEXT    NOT NULL DEFAULT '',     -- the address the request came from
    key_issued_at    TEXT,                            -- when its key was handed out
    merged_into      INTEGER REFERENCES integrations (id),  -- a request the owner attached to an integration it was before
    created_at       TEXT    NOT NULL,
    updated_at       TEXT    NOT NULL,
    updated_by       INTEGER REFERENCES users (id),
    version          INTEGER NOT NULL DEFAULT 1,
    deleted_at       TEXT,
    origin           TEXT    NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS "user_identities" (
    id               INTEGER PRIMARY KEY,
    uuid             TEXT    NOT NULL UNIQUE,
    user_id          INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id      INTEGER REFERENCES auth_providers (id) ON DELETE CASCADE,
    integration_id   INTEGER REFERENCES integrations (id) ON DELETE CASCADE,
    subject          TEXT,                             -- the provider's sub, the integration's user id, or system; NULL until a sign-in binds an e-mail
    email            TEXT    NOT NULL DEFAULT '',      -- lower case: the e-mail assigned, or the one of the sign-in that bound it
    created_at       TEXT    NOT NULL,
    created_by       INTEGER REFERENCES users (id),
    bound_at         TEXT,
    last_used_at     TEXT,
    name             TEXT    NOT NULL DEFAULT '',
    username         TEXT    NOT NULL DEFAULT '',
    groups           TEXT    NOT NULL DEFAULT '[]',
    own_user_id      INTEGER REFERENCES users (id),   -- an integration's: the user made at its first call, kept when linked
    system_generated INTEGER NOT NULL DEFAULT 0,      -- an integration's own system user (Home Assistant's Supervisor, Cloud, …)
    last_level       TEXT    NOT NULL DEFAULT '',     -- the level of its last call, for display
    CHECK ((provider_id IS NULL) <> (integration_id IS NULL))
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
CREATE UNIQUE INDEX rooms_room ON rooms (cube_id, room_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX integrations_key ON integrations (key) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX user_identities_email ON user_identities (provider_id, email) WHERE subject IS NULL AND provider_id IS NOT NULL;
CREATE UNIQUE INDEX user_identities_subject ON user_identities (provider_id, subject) WHERE subject IS NOT NULL AND provider_id IS NOT NULL;
CREATE UNIQUE INDEX user_identities_integration ON user_identities (integration_id, subject) WHERE integration_id IS NOT NULL;
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
