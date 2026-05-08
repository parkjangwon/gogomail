# TypeScript client generation from OpenAPI spec
.PHONY: gen-ts-client
gen-ts-client:
	@mkdir -p clients/typescript
	npx openapi-typescript docs/openapi.yaml \
		-o clients/typescript/index.ts \
		--enum \
		--export-type \
		--alphabetize
