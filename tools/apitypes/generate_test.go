// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// spec wraps schemas, indented as components.schemas' entries, into an
// OpenAPI document.
func spec(schemas string) []byte {
	return []byte("openapi: 3.0.3\ncomponents:\n  schemas:\n" + schemas)
}

// body is the output of generate without the file's header.
func body(t *testing.T, out []byte) string {
	t.Helper()
	s, ok := strings.CutPrefix(string(out), header)
	if !ok {
		t.Fatalf("the output does not open with the header:\n%s", out)
	}
	return s
}

// TestMapping checks each keyword's TypeScript, one small schema at a time.
func TestMapping(t *testing.T) {
	tests := []struct {
		name, schemas, want string
	}{
		{"required and optional", `
    Room:
      type: object
      required: [id]
      properties:
        id: {type: integer}
        name: {type: string, description: The room's name}
        ok: {type: boolean}
        temp: {type: number, minimum: 4.5, maximum: 30.5}
`, `
export interface Room {
  id: number;
  /** The room's name */
  name?: string;
  ok?: boolean;
  temp?: number;
}
`},
		{"enum", `
    Mode:
      type: string
      enum: [auto, manual]
      description: How the room runs
`, `
/** How the room runs */
export type Mode = "auto" | "manual";
`},
		{"array, ref and a quoted name", `
    List:
      type: object
      properties:
        rooms: {type: array, items: {$ref: "#/components/schemas/Room"}}
        modes: {type: array, items: {type: string, enum: [a, b]}}
        x-y: {type: string}
`, `
export interface List {
  rooms?: Room[];
  modes?: ("a" | "b")[];
  "x-y"?: string;
}
`},
		{"nullable, format and allOf of one", `
    Me:
      type: object
      properties:
        user:
          nullable: true
          allOf: [{$ref: "#/components/schemas/User"}]
        at: {type: string, format: date-time, nullable: true}
`, `
export interface Me {
  user?: User | null;
  /** @format date-time */
  at?: string | null;
}
`},
		{"allOf extends", `
    Detail:
      description: A device with more
      allOf:
        - {$ref: "#/components/schemas/Device"}
        - type: object
          properties:
            firmware: {type: integer}
`, `
/** A device with more */
export interface Detail extends Device {
  firmware?: number;
}
`},
		{"allOf as an intersection, and records", `
    Cube:
      type: object
      properties:
        problems:
          type: array
          items:
            allOf:
              - {$ref: "#/components/schemas/Device"}
              - {type: object, properties: {room: {type: string}}}
        rows: {type: object, additionalProperties: {type: integer}}
        detail: {type: object}
        free: {description: Anything}
`, `
export interface Cube {
  problems?: (Device & {
    room?: string;
  })[];
  rows?: Record<string, number>;
  detail?: Record<string, unknown>;
  /** Anything */
  free?: unknown;
}
`},
		{"a long description, wrapped, and no example", `
    Long:
      type: string
      example: KEQ0000001
      description: >-
        one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen
`, `
/**
 * one two three four five six seven eight nine ten eleven twelve thirteen
 * fourteen fifteen
 */
export type Long = string;
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, n, err := generate(spec(tt.schemas))
			if err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Errorf("%d declarations, want 1", n)
			}
			if got := body(t, out); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// TestUnmapped checks that a keyword the generator does not map stops it,
// naming the schema and the keyword.
func TestUnmapped(t *testing.T) {
	tests := []struct {
		name, schemas, want string
	}{
		{"oneOf", `
    Pick:
      oneOf: [{type: string}, {type: integer}]
`, "schema Pick: keyword oneOf is not mapped"},
		{"nested", `
    Room:
      type: object
      properties:
        id: {type: integer, readOnly: true}
`, "schema Room.id: keyword readOnly is not mapped"},
		{"a stray key of a comma in a flow mapping", `
    Room:
      type: object
      properties:
        name: {type: string, description: The name, lower case}
`, "schema Room.name: keyword lower case is not mapped"},
		{"properties beside additionalProperties", `
    Map:
      type: object
      properties: {a: {type: string}}
      additionalProperties: {type: string}
`, "schema Map: additionalProperties beside properties is not mapped"},
		{"required names no property", `
    Room:
      type: object
      required: [id]
      properties: {name: {type: string}}
`, `schema Room: required names "id", which is not a property`},
		{"a $ref outside the schemas", `
    Room:
      type: object
      properties: {a: {$ref: "#/components/responses/NotFound"}}
`, `schema Room.a: $ref "#/components/responses/NotFound" is not a schema of components.schemas`},
		{"an unknown type", `
    File:
      type: file
`, `schema File: type "file" is not mapped`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := generate(spec(tt.schemas))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %v, want one with %q", err, tt.want)
			}
		})
	}
}

// TestGeneratedFileCurrent fails when internal/server/ui/src/api.d.ts is
// not what the OpenAPI file gives, and checks that the file has one
// declaration per schema and that the same input gives the same bytes.
func TestGeneratedFileCurrent(t *testing.T) {
	in, err := os.ReadFile("../../" + specPath)
	if err != nil {
		t.Fatal(err)
	}
	want, n, err := generate(in)
	if err != nil {
		t.Fatal(err)
	}
	again, _, _ := generate(in)
	if !bytes.Equal(want, again) {
		t.Fatal("two runs over the same OpenAPI file differ")
	}
	if c := bytes.Count(want, []byte("\nexport ")); c != n {
		t.Errorf("%d exported declarations for %d schemas", c, n)
	}
	if !bytes.HasSuffix(want, []byte(";\n")) && !bytes.HasSuffix(want, []byte("}\n")) {
		t.Error("the output does not end with a newline after a declaration")
	}
	have, err := os.ReadFile("../../" + outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(have, want) {
		t.Fatalf("%s is not current for %s: run make api-types (go run ./tools/apitypes) from the repository's top", outPath, specPath)
	}
}
