#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The DNS-01 hook of massimo serve (-acme-dns-hook) for a zone hosted at
# Infomaniak:
#
#   infomaniak.sh set   _acme-challenge.<name> <value>
#   infomaniak.sh clear _acme-challenge.<name> <value>
#
# It finds the zone by walking up the name (GET /2/zones/<zone>/exists),
# adds the TXT record (POST /2/zones/<zone>/records, the source relative
# to the zone) and removes the records of that name and value (GET
# /2/zones/<zone>/records, DELETE .../records/<id>): the calls the certbot
# plugin certbot-dns-infomaniak makes. It needs curl and jq, and a token
# with the DNS scope from the API section of the Infomaniak manager in
# INFOMANIAK_TOKEN_FILE (/etc/massimo/infomaniak.token, owned by the
# service user, mode 0600). INFOMANIAK_ENDPOINT overrides the API's
# address (the test's fake), INFOMANIAK_TTL the record's TTL (300).
# The token reaches curl on its standard input (-K -), never on its
# command line, which every local user can read; every call ends after
# 30 seconds, within the server's two minutes for one run of the hook.
# Untested against the real API until the owner's day; the test runs it
# against a fake one (internal/certs/hooks_test.go).
set -eu

verb=${1:?usage: $0 set|clear <record> <value>}
record=${2:?usage: $0 set|clear <record> <value>}
value=${3:?usage: $0 set|clear <record> <value>}
endpoint=${INFOMANIAK_ENDPOINT:-https://api.infomaniak.com}
token_file=${INFOMANIAK_TOKEN_FILE:-/etc/massimo/infomaniak.token}
ttl=${INFOMANIAK_TTL:-300}
token=$(cat "$token_file")
# The token as a quoted value of curl's configuration: \ and " escaped.
header=$(printf '%s' "$token" | sed 's/[\\"]/\\&/g')

# api <method> <path> [<json body>]: the API's answer, an envelope of
# {"result": "success", "data": ...} or {"result": "error", "error": ...}.
# printf is the shell's own, so the header shows in no process's
# arguments.
api() {
	if [ $# -ge 3 ]; then
		printf 'header = "Authorization: Bearer %s"\n' "$header" |
			curl -sS --max-time 30 -K - -X "$1" "$endpoint$2" -H 'Content-Type: application/json' --data "$3"
	else
		printf 'header = "Authorization: Bearer %s"\n' "$header" |
			curl -sS --max-time 30 -K - -X "$1" "$endpoint$2"
	fi
}

# ok <answer>: the answer is a success, else said on stderr.
ok() {
	if [ "$(printf '%s' "$1" | jq -r '.result')" = success ]; then
		return 0
	fi
	printf 'infomaniak: %s\n' "$(printf '%s' "$1" | jq -r '.error.description // .error.code // .')" >&2
	return 1
}

# The zone: the longest suffix of the record that is a zone here.
zone=
candidate=${record#*.}
while [ -n "$candidate" ]; do
	if [ "$(api GET "/2/zones/$candidate/exists" | jq -r '.data')" = true ]; then
		zone=$candidate
		break
	fi
	case $candidate in
	*.*) candidate=${candidate#*.} ;;
	*) candidate= ;;
	esac
done
if [ -z "$zone" ]; then
	printf 'infomaniak: no zone of %s at Infomaniak\n' "$record" >&2
	exit 1
fi
source=${record%".$zone"}
if [ "$source" = "$record" ]; then
	source=.
fi

case $verb in
set)
	body=$(jq -cn --arg s "$source" --arg t "$value" --argjson ttl "$ttl" '{type: "TXT", source: $s, target: $t, ttl: $ttl}')
	answer=$(api POST "/2/zones/$zone/records" "$body")
	ok "$answer"
	printf '%s\n' "$answer" | jq -r '.data.id // .data'
	;;
clear)
	answer=$(api GET "/2/zones/$zone/records")
	ok "$answer"
	ids=$(printf '%s' "$answer" | jq -r --arg s "$source" --arg t "$value" '.data[] | select(.type == "TXT" and .source == $s and (.target == $t or .target == "\"" + $t + "\"")) | .id')
	for id in $ids; do
		ok "$(api DELETE "/2/zones/$zone/records/$id")"
	done
	;;
*)
	printf 'usage: %s set|clear <record> <value>\n' "$0" >&2
	exit 2
	;;
esac
