package gen

import (
	"strings"
	"testing"
)

func TestRefuses(t *testing.T) {
	obj := func(field string) string {
		return `{"$id":"x","title":"t","type":"object","additionalProperties":false,"required":["a"],"properties":{"a":` + field + `}}`
	}
	cases := map[string]struct{ schema, want string }{
		"open object":     {`{"type":"object","properties":{}}`, "closed"},
		"unknown pattern": {obj(`{"type":"string","pattern":"^a+$"}`), "TRG0403"},
		"nested object":   {obj(`{"type":"object"}`), "unsupported"},
		"unknown keyword": {obj(`{"type":"string","contentEncoding":"base64"}`), "contentEncoding"},
		"untagged union":  {`{"oneOf":[{"type":"object","properties":{}}]}`, "tag"},
		"format only":     {obj(`{"type":"string","format":"date-time"}`), "format"},
	}
	for name, c := range cases {
		_, err := Generate("p", []Spec{{Name: "T", Source: name, Schema: []byte(c.schema)}})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error mentioning %q", name, err, c.want)
		}
	}
}

func TestGoName(t *testing.T) {
	for in, want := range map[string]string{"sessionId": "SessionID", "ev-charging": "EvCharging", "a_b": "AB", "authId": "AuthID"} {
		if got := GoName(in); got != want {
			t.Errorf("GoName(%q) = %q, want %q", in, got, want)
		}
	}
}
