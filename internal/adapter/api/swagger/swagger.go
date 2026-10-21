// Package swagger holds the hand-written OpenAPI 3.1 documents for the
// Automata API. They are embedded into the binary and served (behind the
// standard auth middleware) at:
//
//	GET /api/swagger.yaml        — main API, incl. the API-keys reference
//	GET /api/swagger-public.yaml — public tracking endpoints (no auth)
package swagger

import _ "embed"

// OpenAPIMain is the main API document (openapi.yaml).
//
//go:embed openapi.yaml
var OpenAPIMain []byte

// OpenAPIPublic is the public tracking document (openapi-public.yaml).
//
//go:embed openapi-public.yaml
var OpenAPIPublic []byte