# @gogomail/api-types

Auto-generated TypeScript types from the GoGoMail OpenAPI spec.

**`index.ts` is not committed to git.** Generate it before building:

```bash
make gen-ts-client
```

This runs:
```bash
npx openapi-typescript docs/openapi.yaml \
  -o clients/typescript/index.ts \
  --enum \
  --export-type \
  --alphabetize
```

## CI / first-time setup

Run `make gen-ts-client` before any TypeScript build that imports from
`@gogomail/api-types`. The `apps/console` Next.js config transpiles this
workspace package via `transpilePackages`.

## Why not commit the generated file?

The generated file is 17,500+ lines and changes on every OpenAPI spec update.
This creates noisy diffs and inflates git history. Generating at build time
is the standard practice for auto-generated type clients.
