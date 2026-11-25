# apitypes

`make api-types` writes the API's shapes as TypeScript:
it reads the schemas of `docs/api-v1-openapi.yaml` (`components.schemas`)
and writes `internal/server/ui/src/api.d.ts`,
one exported declaration per schema, in the file's order,
which `make ui-check` holds the web UI's parts to
(`docs/ui.md`, "How the page is built").
git keeps the file, since it changes only with the API;
a change to a schema writes it again.

```sh
go run ./tools/apitypes          # from the repository's top: writes the file
go run ./tools/apitypes -check   # exits 1 where the file is not current
```

`go test ./...` fails too while the file is not current
(`TestGeneratedFileCurrent`), naming the command.

The mapping covers what the OpenAPI file uses, and nothing more:

| OpenAPI | TypeScript |
|---------|------------|
| an object with `properties` | an interface; a property not in `required` is optional |
| `allOf` of `$ref`s and at most one object, at the top | an interface that `extends` the named ones |
| any other `allOf` | an intersection, `A & { … }` |
| `$ref` | the named type |
| `enum` of strings or integers | a union of literals |
| `type: array` with `items` | `T[]` |
| `type: object` without `properties` | `Record<string, unknown>`, or `Record<string, T>` for `additionalProperties: T` |
| `string`, `integer`, `number`, `boolean` | `string`, `number`, `number`, `boolean` |
| no type | `unknown` |
| `nullable: true` | `T \| null` |
| `description`, `format` | a JSDoc comment, the format as `@format` |

`example`, `minimum`, `maximum`, `multipleOf`, `minLength`, `maxLength`,
`maxItems` and `pattern` constrain a value without changing its shape and are left out;
an example stays out of the output.
Any other keyword stops the program with an error naming the schema and the keyword,
and the file is left as it was;
so does a description with a comma left unquoted inside a flow mapping
(`{type: string, description: A, B}`), which YAML reads as a key `B`.
The routes' inline schemas, a response's list or a request's body,
get no names: a part reads them through the named schemas they hold.
