// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/portal/portalcrypto"
)

func portalCryptoUsage() {
	fmt.Fprint(os.Stderr, `usage: massimo portal-crypto <subcommand> [flags]

The Cube key is read from the environment variable `+env.Key+` or, if that
is unset, prompted for on the terminal. It is never written anywhere.

subcommands:
  iv -addr <cube ip>
        Connect to the Cube, encrypt 32 zero bytes with e: and recover the
        IV from the answer (and verify CBC chaining).
  iv -plain <base64> -cipher <E-line>
        The same from an e:/E: exchange done by hand.
  check -iv <hex> -plain <base64> -cipher <E-line>
        Verify that encrypting plain with key and IV gives cipher.
  encrypt -iv <hex> [-text <string> | -plain <base64>]
        Print the base64 ciphertext (what the Cube would answer in E:).
  decrypt -iv <hex> -cipher <base64>
        Print the decrypted text (zero padding removed).
  derive -serial <serial> [-mac <hex>] [-rf <hex>] [-extra a,b] -cipher <E-line> -plain <base64>
        Try a few obvious key derivations (MD5 of serial, MAC, RF address)
        against a known exchange, and report whether any of them works.
`)
}

func runPortalCrypto(args []string) error {
	if len(args) == 0 {
		portalCryptoUsage()
		return fmt.Errorf("missing subcommand")
	}
	sub, args := args[0], args[1:]
	fs := flag.NewFlagSet("portal-crypto "+sub, flag.ContinueOnError)
	ivHex := fs.String("iv", "", "IV as 32 hex digits")
	plainB64 := fs.String("plain", "", "plaintext, base64 (as given to the e: command)")
	text := fs.String("text", "", "plaintext as a literal string")
	cipherB64 := fs.String("cipher", "", "ciphertext, base64 (the E: line, with or without the E: prefix)")
	serial := fs.String("serial", "", "Cube serial number")
	mac := fs.String("mac", "", "Cube MAC address, hex")
	rf := fs.String("rf", "", "Cube RF address, hex")
	extra := fs.String("extra", "", "additional identifiers to try, comma-separated (e.g. the chip ID)")
	addr := fs.String("addr", "", "Cube address, host[:port]: do the e: exchange over the LAN (iv)")
	snFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if sub == "iv" && *plainB64 == "" && *cipherB64 == "" {
		if err := defaultAddr(addr, "iv"); err != nil {
			return fmt.Errorf("%w, or -plain and -cipher", err)
		}
		zero := make([]byte, 2*portalcrypto.BlockSize)
		reply, serial, err := lanExchange(*addr, "e:"+base64.StdEncoding.EncodeToString(zero), 'E')
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "cube %s answered %s\n", serial, reply)
		*plainB64 = base64.StdEncoding.EncodeToString(zero)
		*cipherB64 = reply
	}

	var plain, ct, iv []byte
	var err error
	if *text != "" {
		plain = []byte(*text)
	} else if *plainB64 != "" {
		if plain, err = base64.StdEncoding.DecodeString(*plainB64); err != nil {
			return fmt.Errorf("-plain: %w", err)
		}
	}
	if *cipherB64 != "" {
		s := strings.TrimSpace(*cipherB64)
		s = strings.TrimPrefix(s, "E:")
		if ct, err = base64.StdEncoding.DecodeString(s); err != nil {
			return fmt.Errorf("-cipher: %w", err)
		}
	}
	if *ivHex != "" {
		if iv, err = portalcrypto.ParseIV(*ivHex); err != nil {
			return err
		}
	}

	if sub == "derive" {
		return deriveKey(*serial, *mac, *rf, *extra, plain, ct)
	}

	key, err := readKey()
	if err != nil {
		return err
	}

	switch sub {
	case "iv":
		if plain == nil || ct == nil {
			return fmt.Errorf("iv: need -plain and -cipher")
		}
		iv, err := portalcrypto.RecoverIV(key, ct, portalcrypto.Pad(plain))
		if err != nil {
			return err
		}
		fmt.Printf("iv: %s\n", hex.EncodeToString(iv))
		if len(ct) >= 2*portalcrypto.BlockSize {
			if err := portalcrypto.CheckCBC(key, plain, ct); err != nil {
				return fmt.Errorf("chaining check FAILED: %w (wrong key, or not CBC)", err)
			}
			fmt.Println("chaining check: OK (AES-128-CBC with this key)")
		} else {
			fmt.Println("chaining check: skipped (need at least 32 bytes of data)")
		}
		return nil
	case "check":
		if iv == nil || plain == nil || ct == nil {
			return fmt.Errorf("check: need -iv, -plain and -cipher")
		}
		got, err := portalcrypto.Encrypt(key, iv, plain)
		if err != nil {
			return err
		}
		if base64.StdEncoding.EncodeToString(got) != base64.StdEncoding.EncodeToString(ct) {
			return fmt.Errorf("MISMATCH: computed %s", base64.StdEncoding.EncodeToString(got))
		}
		fmt.Println("OK: key and IV reproduce the Cube's output")
		return nil
	case "encrypt":
		if iv == nil || plain == nil {
			return fmt.Errorf("encrypt: need -iv and -plain or -text")
		}
		got, err := portalcrypto.Encrypt(key, iv, plain)
		if err != nil {
			return err
		}
		fmt.Println(base64.StdEncoding.EncodeToString(got))
		return nil
	case "decrypt":
		if iv == nil || ct == nil {
			return fmt.Errorf("decrypt: need -iv and -cipher")
		}
		got, err := portalcrypto.Decrypt(key, iv, ct)
		if err != nil {
			return err
		}
		os.Stdout.Write(portalcrypto.Unpad(got))
		fmt.Println()
		return nil
	default:
		portalCryptoUsage()
		return fmt.Errorf("unknown subcommand %q", sub)
	}
}

// readKey takes the key from the environment or prompts for it.
func readKey() ([]byte, error) {
	s := cubeEnv(env.Key)
	if s == "" {
		fmt.Fprint(os.Stderr, "Cube key (from the QR code, k + 32 hex digits): ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return nil, fmt.Errorf("no key given")
		}
		s = line
	}
	return portalcrypto.ParseKey(s)
}
