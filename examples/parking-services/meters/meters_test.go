package meters

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"example.com/parking-services/internal/exchanges"
	"example.com/parking-services/payments"
)

type card struct{ given exchanges.Given }

func (c card) Approve(string, int64, string) (string, bool) {
	return c.given.AuthID, c.given.Card == "approve"
}

// TestExchanges replays core's meters.start exchanges against a real
// payments service over HTTP. v1 cases go to the v1 route of this v2
// server, so they also check v2_serves_v1 in Go. Each call meters makes to
// payments must be exactly the call core recorded, with the same answer.
func TestExchanges(t *testing.T) {
	for file, route := range map[string]string{"meters.start.v1.json": "/v1/start", "meters.start.v2.json": "/v2/start"} {
		var f exchanges.File
		if err := exchanges.Load(filepath.Join("../core/exchanges", file), "tribrach.exchanges.v1", &f); err != nil {
			t.Fatal(err)
		}
		for _, k := range f.Cases {
			t.Run(file+"/"+k.ID, func(t *testing.T) { replay(t, route, k) })
		}
	}
}

func replay(t *testing.T, route string, k exchanges.Case) {
	var calls []exchanges.Call
	pay := payments.Service{Card: card{k.Given}}
	paySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(req))
		rec := httptest.NewRecorder()
		pay.ServeHTTP(rec, r)
		calls = append(calls, exchanges.Call{Contract: "payments.authorize", Request: req, Response: rec.Body.Bytes()})
		for key, v := range rec.Header() {
			w.Header()[key] = v
		}
		w.WriteHeader(rec.Code)
		w.Write(rec.Body.Bytes())
	}))
	defer paySrv.Close()

	now, err := time.Parse(time.RFC3339, k.Given.Now)
	if err != nil {
		t.Fatal(err)
	}
	free := 1
	if k.Given.ChargersFree != nil {
		free = *k.Given.ChargersFree
	}
	svc := Service{
		Now:          func() time.Time { return now },
		NewSessionID: func() string { return k.Given.SessionID },
		Token:        k.Given.Token,
		Pay:          Client{URL: paySrv.URL},
		ChargersFree: func() int { return free },
	}
	srv := httptest.NewServer(svc.Routes())
	defer srv.Close()

	resp, err := http.Post(srv.URL+route, "application/json", bytes.NewReader(k.Request))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != k.Status {
		t.Fatalf("status %d, core says %d (%s)", resp.StatusCode, k.Status, body)
	}
	if k.Status == 200 && !exchanges.SameJSON(body, k.Response) {
		t.Errorf("answered %s, core says %s", body, k.Response)
	}
	if len(calls) != len(k.Calls) {
		got, _ := json.Marshal(calls)
		t.Fatalf("made %d calls, core says %d: %s", len(calls), len(k.Calls), got)
	}
	for i, c := range k.Calls {
		if calls[i].Contract != c.Contract || !exchanges.SameJSON(calls[i].Request, c.Request) || !exchanges.SameJSON(calls[i].Response, c.Response) {
			t.Errorf("call %d: sent %s and got %s; core says %s and %s", i, calls[i].Request, calls[i].Response, c.Request, c.Response)
		}
	}
}

// TestChargeWithinLimit is the Go side of core's Meters.charge_within_limit:
// every valid session prices between 1 cent and payments.MaxAmount.
func TestChargeWithinLimit(t *testing.T) {
	for zone, rate := range Rates {
		for m := int64(15); m <= 240; m += 15 {
			if c := Charge(rate, m); c < 1 || c > payments.MaxAmount {
				t.Errorf("zone %s, %d minutes: %d cents is outside 1..%d", zone, m, c, payments.MaxAmount)
			}
		}
	}
}
