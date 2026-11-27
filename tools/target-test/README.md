# Target test

`make target-test` runs `run.sh`:
the server backup's targets whose protocol no Go server of ours can stand in for,
tested against real servers in Docker (AP ServerBackup, Blocks 8 and 9):
the SMB target against Samba, the S3 target against MinIO.
The Go tests of `internal/serverbackup` test the targets' logic
against a fake of the client,
and an S3 fake proves only our reading of the protocol;
here the bytes go to a server that others use.
WebDAV and SFTP need no container:
`golang.org/x/net/webdav` and `pkg/sftp`'s server run in the Go tests.

It needs Docker with Compose v2 and Go, and takes under a minute,
the Samba image built and MinIO's pulled the first time.
It runs in WSL 2 as well.

## What it runs

`compose.yaml` brings up two servers,
each on a port of the loopback that Docker picks,
so that two runs side by side do not collide:

- **samba**:
  Debian's samba (`Dockerfile`), a standalone server with one share,
  `backups`, for the user `samba.sh` makes at the start (`smb.conf`);
  no guest, every message signed, the share's traffic encrypted (SMB 3),
  as a NAS can be set to ask;
- **minio**:
  MinIO, the community's fork `pgsty/minio`, pinned at a release,
  since MinIO publishes no image of its own any more;
  its region is `eu-test-1`, not AWS's default,
  so that a signature for another region is refused.

`run.sh` names the project `massimo-target-<pid>`,
waits until each server answers on its own loopback
(Docker's port on the host accepts a connection before either does),
makes the bucket `massimo-test` with MinIO's `mc`,
and runs the live tests with their variables set:

```sh
MASSIMO_ENV_FILE=/dev/null go test ./internal/serverbackup -run Live -count=1 -v
```

| Variable | Value |
|----------|-------|
| `MASSIMO_TARGET_TEST_SMB_ADDR` | Samba's `host:port` |
| `MASSIMO_TARGET_TEST_SMB_SHARE` | `backups` |
| `MASSIMO_TARGET_TEST_SMB_USER`, `_SMB_PASS` | the share's user and its password |
| `MASSIMO_TARGET_TEST_S3_ENDPOINT` | MinIO's URL, `http://host:port` |
| `MASSIMO_TARGET_TEST_S3_BUCKET` | `massimo-test` |
| `MASSIMO_TARGET_TEST_S3_REGION` | `eu-test-1` |
| `MASSIMO_TARGET_TEST_S3_KEY`, `_S3_SECRET` | MinIO's access key and its secret |

The accounts are test values set in `run.sh`, which open nothing else.
A live test skips without its variables, as `go test ./...` runs it;
here, where all of them are set, a skip fails the run,
as a test reading another name than these would,
and so does a run in which no live test passed.

- **TestSMBLive** (`smb_live_test.go`) works in a folder of its own,
  removed at its end:
  the card's Test making the folder;
  a copy of a few MiB, in several writes, listed and read back by ranges;
  a put broken off leaving nothing, under its name or beside it;
  a backup found by its manifest over ranged reads (`Scan`);
  a delete, twice;
  the share's root, and the user written `WORKGROUP\user`;
  a wrong password, a share not there and a closed port refused,
  each saying what refused, never with the password.
- **TestS3Live**, Block 9's, tests the S3 target against MinIO.

The servers are taken down whatever happened,
their logs printed first where the run failed.
`KEEP=1 make target-test` leaves them up
and prints the variables to export and the command that takes them down,
so that a test can be run again by hand against them.

## In CI

CI's job Backup targets runs `make target-test`,
as `GUIDELINES/project/CI.md` says in its section 2:
on a pull request that changes `internal/serverbackup/`, this directory,
the Go module or `ci.yml`, and on a push of code that had not passed before.
The runner has Docker with Compose v2;
the Samba image is built at each run.
