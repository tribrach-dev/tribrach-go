package contracts

import (
	"path/filepath"
	"testing"

	"example.com/parking-services/internal/exchanges"
)

type codec struct {
	decode func([]byte) error
	encode func([]byte) ([]byte, error) // decode then encode again
}

func c[T any](dec func([]byte) (T, error), enc func(T) ([]byte, error)) codec {
	return codec{
		decode: func(b []byte) error { _, err := dec(b); return err },
		encode: func(b []byte) ([]byte, error) {
			v, err := dec(b)
			if err != nil {
				return nil, err
			}
			return enc(v)
		},
	}
}

var codecs = map[string]codec{
	"payments.authorize.request":  c(DecodeAuthorizeRequest, EncodeAuthorizeRequest),
	"payments.authorize.response": c(DecodeAuthorizeResponse, EncodeAuthorizeResponse),
	"meters.start.v1.request":     c(DecodeStartRequest, EncodeStartRequest),
	"meters.start.v1.response":    c(DecodeStartResponse, EncodeStartResponse),
	"meters.start.v2.request":     c(DecodeStartRequestV2, EncodeStartRequestV2),
	"meters.start.v2.response":    c(DecodeStartResponseV2, EncodeStartResponseV2),
}

// TestVerdicts checks the generated decoders against Lean: each accepts
// exactly the messages core's schema accepts.
func TestVerdicts(t *testing.T) {
	total := 0
	for name, cd := range codecs {
		var v exchanges.Verdicts
		if err := exchanges.Load(filepath.Join("../core/verdicts", name+".json"), "tribrach.verdicts.v1", &v); err != nil {
			t.Fatal(err)
		}
		for _, k := range v.Cases {
			total++
			err := cd.decode(k.Doc)
			if (err == nil) != k.Accept {
				t.Errorf("%s: Go accepts=%v, Lean accepts=%v on %s (%v)", name, err == nil, k.Accept, k.Doc, err)
			}
		}
	}
	if total < 1000 {
		t.Fatalf("only %d verdicts; is ../core vendored?", total)
	}
	t.Logf("%d verdicts agree", total)
}

// TestRoundTrips decodes every message in the exchanges and writes it again:
// the JSON must come back the same, since core writes the canonical form.
func TestRoundTrips(t *testing.T) {
	files := map[string][2]string{
		"payments.authorize.json": {"payments.authorize.request", "payments.authorize.response"},
		"meters.start.v1.json":    {"meters.start.v1.request", "meters.start.v1.response"},
		"meters.start.v2.json":    {"meters.start.v2.request", "meters.start.v2.response"},
	}
	for file, names := range files {
		var f exchanges.File
		if err := exchanges.Load(filepath.Join("../core/exchanges", file), "tribrach.exchanges.v1", &f); err != nil {
			t.Fatal(err)
		}
		for _, k := range f.Cases {
			check := func(name string, doc []byte) {
				out, err := codecs[name].encode(doc)
				if err != nil {
					t.Errorf("%s %s: %v", file, k.ID, err)
				} else if !exchanges.SameJSON(doc, out) {
					t.Errorf("%s %s: %s came back as %s", file, k.ID, doc, out)
				}
			}
			if k.Status != 200 {
				if codecs[names[0]].decode(k.Request) == nil {
					t.Errorf("%s %s: status %d but the request decodes", file, k.ID, k.Status)
				}
				continue
			}
			check(names[0], k.Request)
			check(names[1], k.Response)
			for _, call := range k.Calls {
				check(call.Contract+".request", call.Request)
				check(call.Contract+".response", call.Response)
			}
		}
	}
}
