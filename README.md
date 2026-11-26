# tribrach-go

The Go instrument for [Tribrach](https://github.com/tribrach-dev/tribrach).
Pre-alpha; nothing is released yet.

Planned (see the core architecture):

- a `go vet` analyzer (`-vettool`) for links, purity and drift;
- a stamp command that records the manifest's stamps next to each linked
  function;
- a table-driven replay harness for exported vectors.

Licence: Apache-2.0. Contributions: see [CONTRIBUTING.md](CONTRIBUTING.md).
