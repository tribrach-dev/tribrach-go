// Package contracts holds the parking DTOs, generated from the JSON Schemas
// that core's examples/dto-parking exports (vendored in ../core).
//
// Regenerate after updating ../core:
//
//go:generate go tool tribrach-go gen -pkg contracts -o contracts.gen.go AuthorizeRequest=../core/schemas/payments.authorize.request.json AuthorizeResponse=../core/schemas/payments.authorize.response.json StartRequest=../core/schemas/meters.start.v1.request.json StartResponse=../core/schemas/meters.start.v1.response.json StartRequestV2=../core/schemas/meters.start.v2.request.json StartResponseV2=../core/schemas/meters.start.v2.response.json
package contracts
