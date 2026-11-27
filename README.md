# tribrach-go

The Go instrument for [Tribrach](https://github.com/tribrach-dev/tribrach).
Pre-alpha; nothing is released yet.

It reads the manifest and vectors a project exports from Lean
(`Tribrach.Export` in core) and checks Go code against them.

## Annotate

```go
// Step applies one operation, never going above max.
//
//tribrach:link Counter.counterStep
//tribrach:stamp 0123456789abcdef
func Step(max uint, op Op, n uint) uint { … }
```

- `//tribrach:link <LeanName>` links the function to a Lean definition. A
  linked function must be pure.
- `//tribrach:pure` marks a helper that linked code may call.
- `//tribrach:stamp` records the Lean stamp the function was last checked
  against. `tribrach-go stamp` writes it; don't edit it by hand.

Symbols are written `importpath.Func` or `importpath.Type.Method`.

## Check: `go vet`

```sh
go install github.com/tribrach-dev/tribrach-go/cmd/tribrach-vet@latest
go vet -vettool=$(which tribrach-vet) ./...
```

The analyzer reads the nearest `tribrach.manifest.json` above each package
(or `-manifest path`). `-allow TRG0306,time.Now` allows codes or functions
in pure code. Every diagnostic names the Lean definition, the reason and the
fix.

| Code | Meaning |
| --- | --- |
| TRG0101 | A directive can't be read |
| TRG0102 | A directive isn't in a function's doc comment |
| TRG0103 | A function links two Lean definitions |
| TRG0104 | A stamp without a link |
| TRG0200 | No manifest, or it can't be read |
| TRG0201 | Drift: the Lean definition changed since the recorded stamp |
| TRG0202 | A link with no recorded stamp |
| TRG0203 | The manifest has no Go link for this Lean name |
| TRG0204 | The manifest links this Lean name to another symbol |
| TRG0205 | The manifest links a symbol in this package that isn't annotated |
| TRG0301 | Clock reads in pure code |
| TRG0302 | Randomness in pure code |
| TRG0303 | I/O in pure code (os, net, log, fmt printing) |
| TRG0304 | Package-level variables in pure code |
| TRG0305 | Goroutines, channels, select, sync |
| TRG0306 | Iterating over a map (unordered) |
| TRG0307 | Calling a same-package function that isn't marked pure |

## Stamp deliberately

```sh
tribrach-go stamp -n     # list stale stamps; exit 1 if any
tribrach-go stamp        # record the manifest's stamps
```

Stamps sit next to each function, never in a shared file, so stamping one
package can't disturb another.

## Generate DTOs: `tribrach-go gen`

```sh
tribrach-go gen -pkg contracts -o contracts.gen.go \
  StartRequest=schemas/meters.start.v1.request.json ...
```

Writes Go types, `DecodeX` and `EncodeX` for each JSON Schema core exported
with `Tribrach.Schema`. Decoders read JSON as core's Lean decoders do
(package `wire`): closed objects, money as decimal strings into `int64`,
RFC 3339 instants into `time.Time`, tagged unions as sealed interfaces.
Encoders refuse to write what their own schema would reject. `-n` checks
the file is up to date. Anything outside the subset core emits is an error,
never a guess.

| Code | Meaning |
| --- | --- |
| TRG0401 | A generated file is out of date with its schemas |
| TRG0402 | A schema uses something the generator doesn't support |
| TRG0403 | A pattern isn't one of core's (`Tribrach.Schema.Pat`) |

`examples/parking-services` checks the generated decoders against verdicts
from Lean and replays exchanges recorded from the Lean model.

## Replay

```go
func TestStepMatchesModel(t *testing.T) {
	replay.Run(t, "testdata/counter.vectors.json", func() replay.Harness { return &harness{max: 3} })
}
```

A `Harness` starts from a state, applies operations (`{"op": name, …}`)
and reports its state. Each case is a subtest, and a failure names the
step, the operation, and the model's and the code's states. `replay.Check`
returns the results instead, for checks such as "the vectors catch this
bug" (see `examples/counter`).

## Conformance

`conformance/` runs the core conformance cases. They are a copy of
`tribrach/tests/conformance` at the commit in `conformance/testdata/CORE_REV`,
and CI checks the copy.

Licence: Apache-2.0. Contributions: see [CONTRIBUTING.md](CONTRIBUTING.md).
