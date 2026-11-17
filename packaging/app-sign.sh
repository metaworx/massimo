#!/usr/bin/env bash
# Signs the Android app (AP MobileApp, Block 4, D14-A and D16-A): an APK
# with apksigner, the signature schemes v2 and v3 its minSdk 26 reads and
# neither the JAR signature nor v4's separate file; an AAB with jarsigner,
# as Google Play takes it from the upload key. It then reads the signer
# back and compares it with the certificate committed beside the app, so
# that a file signed by another key is never handed on: release.yml's
# signing jobs use it with the keys, the job Android app with a key made
# for the run.
#
#   app-sign.sh apk|aab <in> <out> <keystore> <alias> <certificate>
#
# The keystore is PKCS#12, its password in APP_KS_PASS and never on a
# command line. apksigner is build-tools 36.0.0's under ANDROID_HOME, or
# the one in BUILD_TOOLS; jarsigner and keytool the JDK's.
set -euo pipefail

die() {
	echo "app-sign: $*" >&2
	exit 1
}

if [ "$#" -ne 6 ]; then
	echo "usage: app-sign.sh apk|aab <in> <out> <keystore> <alias> <certificate>" >&2
	exit 2
fi
kind="$1" in="$2" out="$3" ks="$4" alias="$5" cert="$6"
case "$kind" in apk | aab) ;; *) die "$kind is neither apk nor aab" ;; esac
[ -n "${APP_KS_PASS:-}" ] || die "APP_KS_PASS is empty"
[ -f "$in" ] || die "$in is missing"
[ -f "$ks" ] || die "$ks is missing"
[ -f "$cert" ] || die "$cert is missing: the certificate the signer must be"

# hex: lower-case hex without colons, as both tools' digests are compared.
hex() { tr -d ': \t' | tr 'A-F' 'a-f'; }
want="$(openssl x509 -in "$cert" -outform DER | sha256sum | cut -d' ' -f1)"
[ "${#want}" -eq 64 ] || die "$cert is not a PEM certificate"

tmp="$out.signing"
trap 'rm -f "$tmp"' EXIT
case "$kind" in
apk)
	bt="${BUILD_TOOLS:-${ANDROID_HOME:?neither BUILD_TOOLS nor ANDROID_HOME is set}/build-tools/36.0.0}"
	"$bt/apksigner" sign --ks "$ks" --ks-type PKCS12 --ks-key-alias "$alias" --ks-pass env:APP_KS_PASS \
		--min-sdk-version 26 --v1-signing-enabled false --v2-signing-enabled true \
		--v3-signing-enabled true --v4-signing-enabled false --out "$tmp" "$in"
	report="$("$bt/apksigner" verify --min-sdk-version 26 --verbose --print-certs "$tmp")" ||
		die "apksigner does not verify what it signed"
	grep -qx 'Verified using v3 scheme (APK Signature Scheme v3): true' <<<"$report" ||
		die "$out carries no v3 signature"
	signers="$(grep -c '^Signer #[0-9]* certificate SHA-256 digest:' <<<"$report" || true)"
	got="$(sed -n 's/^Signer #1 certificate SHA-256 digest: //p' <<<"$report" | hex)"
	;;
aab)
	cp "$in" "$tmp"
	jarsigner -keystore "$ks" -storetype PKCS12 -storepass:env APP_KS_PASS \
		-sigalg SHA256withRSA -digestalg SHA-256 "$tmp" "$alias" >/dev/null
	jarsigner -verify "$tmp" | grep -qx 'jar verified.' || die "jarsigner does not verify what it signed"
	report="$(keytool -printcert -jarfile "$tmp")"
	signers="$(grep -c '^Signer #' <<<"$report" || true)"
	got="$(sed -n 's/^[[:space:]]*SHA256: //p' <<<"$report" | hex)"
	;;
esac
[ "$signers" = 1 ] || die "$out has ${signers:-no} signers; one is signed for"
[ "$got" = "$want" ] || die "$out is signed by the certificate ${got:-none}, not by $cert ($want)"
mv "$tmp" "$out"
trap - EXIT
echo "app-sign: $out, signed by the certificate $want"
