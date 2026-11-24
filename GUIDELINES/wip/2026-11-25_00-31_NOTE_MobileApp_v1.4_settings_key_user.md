# NOTE MobileApp v1.4: settings key user

D89, the decision the pull request of D86-A and D87-A left open,
built on 2026-11-25 as one pull request,
metaworx/massimo#420 (`claude/mobile-settings-user`), of one commit:
"fix(api): The user that holds the settings file's key is not removed either";
merged as "merge: MobileApp, D89-A: The user that holds the settings file's key
kept (#420)".
It was built by a sub-agent from a brief;
the session read the diff itself.

## The decisions

- **D89-A, the user that holds the settings file's key**,
  the owner's "EXEC D89-A" on 2026-11-25:
  A, "refuse removing that user, 409".
  B, "Bootstrap makes the file's key live again",
  and C, "leave it", were not taken.
  `DELETE /api/v1/users/{name}` answers `409` for the user
  that holds the settings file's live `MASSIMO_API_TOKEN`,
  as the key's revoke has since D86-A:
  "user <name> holds the settings file's MASSIMO_API_TOKEN:
  change or remove it in the settings file first".
  The removal revoked the key with the user's tokens,
  and `store.Bootstrap`, which looks at live keys alone,
  then failed at every start on the key's hash,
  taken by the revoked row, until the file's token changed.
  The server's `settingsKey` returns the key with its user's id,
  where `settingsKeyID` returned the id alone,
  so that `deleteUser` checks it before `store.DeleteUser`
  without a second lookup.
  The unreleased Fixed bullet of D86-A names the removal too,
  no release having been cut since.

## How it ran

- **Seen failing first**: `TestSettingsFileKeyHolder`,
  the holder removed by another owner with `200`.
- **Checked**, each exit 0:
  `make ui`;
  `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/... ./internal/store/...`;
  `TestOpenAPICoversRoutes`;
  `gofmt -l .` and `go vet` for linux, windows and darwin;
  `test_users.py` in the four cases, 68 passed;
  `go test ./tools/identifiers/... ./tools/apitypes/...`;
  `GUIDELINES/shared/tools/check`, its 12 checks;
  `commit-msg.sh` and `changelog.sh check --staged --message`
  on the commit's state.
  No emulator run: nothing under `app/` changes.
- **The page** needed no change:
  a refused removal says the server's words in the users' line,
  as the last owner's `409` does,
  and no test covered it.
  `test_users.py::test_a_refused_removal_says_the_servers_words` does now,
  answering the DELETE itself,
  so that nothing is removed and the suite's own owner,
  which holds the file's key, is never asked for.
- **The other paths that end a token**, read for D89-A,
  end none of the file's key:
  a logout, a password set by another user and a key's next session
  end sessions alone;
  an integration's approval and revoke end that integration's own keys,
  whose user's role is never the owner's;
  Bootstrap revokes a key labelled as the file's whose hash is not;
  and no migration ends a token.

## Departures

- None from the decision.

## Not done here

- **Disabling the holder, an end date that passes, or another role**
  leave the file's key live, so the next start meets no error,
  but take the owner's rights from it,
  and the file then recovers no owner until that is undone:
  left as they are.
- **A store whose key was revoked before D86-A or this change**
  keeps the error at each start until the file's token changes,
  Bootstrap staying as it is.
