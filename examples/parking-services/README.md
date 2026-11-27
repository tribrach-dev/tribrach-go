# Parking services

The Go side of core's `examples/dto-parking`: a payments service and a
meters service that calls it, with every DTO generated from the JSON Schemas
core exports from Lean.

| Directory | What |
| --- | --- |
| `core/` | core's `examples/dto-parking/exported`, vendored at `core/CORE_REV`. CI checks it matches. |
| `contracts/` | `contracts.gen.go`, from `go generate` (`tribrach-go gen`, run as a Go tool). Don't edit it. |
| `payments/` | `payments.authorize` over HTTP |
| `meters/` | `meters.start` v2, v1 on the v1 route, calling payments over HTTP |

## What the tests check

- **Decoders against Lean** (`contracts`): about 2,400 mutated messages from
  `core/verdicts`. Each generated decoder accepts exactly the ones Lean's
  schema accepts.
- **Round trips** (`contracts`): every message in `core/exchanges` decodes
  and writes back as the same JSON.
- **Handlers against the model** (`payments`, `meters`): every exchange core
  recorded from the Lean model is replayed over HTTP, against a real
  payments service. The status, the response and the call meters makes to
  payments must all match. v1 exchanges go to the v1 route of the v2 server,
  which checks `Meters.v2_serves_v1` in Go.

The handlers are Go rewrites of the Lean rules, so they are tested against
the model, not proved. Changing a zone's rate, for example, fails the replay
with the call and the answer that differ.

## Updating to a new core

```sh
cp -r ../../../tribrach/examples/dto-parking/exported/* core/
git -C ../../../tribrach rev-parse HEAD > core/CORE_REV
go generate ./... && go test ./...
```
