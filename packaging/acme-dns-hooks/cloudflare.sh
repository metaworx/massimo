#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The DNS-01 hook of massimo serve (-acme-dns-hook) for a zone hosted at
# Cloudflare:
#
#   cloudflare.sh set   _acme-challenge.<name> <value>
#   cloudflare.sh clear _acme-challenge.<name> <value>
#
# It finds the zone by walking up the name (GET /zones?name=<zone>), or
# takes CLOUDFLARE_ZONE_ID, adds the TXT record (POST
# /zones/<id>/dns_records, the full name) and removes the records of that
# name and value (GET /zones/<id>/dns_records?type=TXT&name=<record>,
# DELETE .../dns_records/<id>): the calls the Cloudflare plugins of
# certbot and lego make. It needs curl and jq, which the Debian package
# suggests and the script checks for before any call, and an API token
# with the permission Zone - DNS - Edit on the zone in
# CLOUDFLARE_TOKEN_FILE (/etc/massimo/cloudflare.token, owned by the
# service user, mode 0600); where the token may not list the zone,
# CLOUDFLARE_ZONE_ID (the zone's id, on its overview page) names it.
# CLOUDFLARE_ENDPOINT overrides the API's address (the test's fake),
# CLOUDFLARE_TTL the record's TTL (120).
# The token reaches curl on its standard input (-K -), never on its
# command line, which every local user can read; every call ends after
# 30 seconds, within the server's two minutes for one run of the hook.
# The test runs it against a fake API (internal/certs/hooks_test.go).
set -eu

verb=${1:?usage: $0 set|clear <record> <value>}
record=${2:?usage: $0 set|clear <record> <value>}
value=${3:?usage: $0 set|clear <record> <value>}
# curl and jq: without one, every call fails with an error that names
# neither, so the missing one is named first.
for tool in curl jq; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		printf 'cloudflare: %s is not installed (on Debian: sudo apt install curl jq)\n' "$tool" >&2
		exit 1
	fi
done
endpoint=${CLOUDFLARE_ENDPOINT:-https://api.cloudflare.com/client/v4}
token_file=${CLOUDFLARE_TOKEN_FILE:-/etc/massimo/cloudflare.token}
ttl=${CLOUDFLARE_TTL:-120}
token=$(cat "$token_file")
# The token as a quoted value of curl's configuration: \ and " escaped.
header=$(printf '%s' "$token" | sed 's/[\\"]/\\&/g')

# api <method> <path> [<json body>]: the API's answer, an envelope of
# {"success": true, "result": ...} or {"success": false, "errors": [...]}.
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

# ok <answer>: the answer is a success, else its errors said on stderr.
ok() {
	if [ "$(printf '%s' "$1" | jq -r '.success' 2>/dev/null)" = true ]; then
		return 0
	fi
	why=$(printf '%s' "$1" | jq -r '[.errors[]?.message] | join("; ")' 2>/dev/null) || why=
	printf 'cloudflare: %s\n' "${why:-not an answer of the API}" >&2
	return 1
}

# uri <text>: text as a query parameter's value.
uri() {
	printf '%s' "$1" | jq -sRr @uri
}

# The zone: CLOUDFLARE_ZONE_ID, else the longest suffix of the record that
# is a zone this token sees.
zone=${CLOUDFLARE_ZONE_ID:-}
candidate=${record#*.}
while [ -z "$zone" ] && [ -n "$candidate" ]; do
	answer=$(api GET "/zones?name=$(uri "$candidate")")
	if ! ok "$answer"; then
		printf 'cloudflare: the zones of %s cannot be listed; CLOUDFLARE_ZONE_ID names the zone\n' "$record" >&2
		exit 1
	fi
	zone=$(printf '%s' "$answer" | jq -r '.result[0].id // empty')
	case $candidate in
	*.*) candidate=${candidate#*.} ;;
	*) candidate= ;;
	esac
done
if [ -z "$zone" ]; then
	printf 'cloudflare: no zone of %s that this token sees; CLOUDFLARE_ZONE_ID names it\n' "$record" >&2
	exit 1
fi

case $verb in
set)
	body=$(jq -cn --arg n "$record" --arg c "$value" --argjson ttl "$ttl" '{type: "TXT", name: $n, content: $c, ttl: $ttl}')
	answer=$(api POST "/zones/$zone/dns_records" "$body")
	ok "$answer"
	printf '%s\n' "$answer" | jq -r '.result.id'
	;;
clear)
	answer=$(api GET "/zones/$zone/dns_records?type=TXT&name=$(uri "$record")&per_page=100")
	ok "$answer"
	# The content as it was given, or in the quotes Cloudflare may add.
	ids=$(printf '%s' "$answer" | jq -r --arg n "$record" --arg c "$value" '.result[] | select(.type == "TXT" and (.name | ascii_downcase) == ($n | ascii_downcase) and (.content == $c or .content == "\"" + $c + "\"")) | .id')
	for id in $ids; do
		ok "$(api DELETE "/zones/$zone/dns_records/$id")"
	done
	;;
*)
	printf 'usage: %s set|clear <record> <value>\n' "$0" >&2
	exit 2
	;;
esac
