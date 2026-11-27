package payments

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/parking-services/internal/exchanges"
)

// givenCard answers as an exchange's given says.
type givenCard struct{ Given exchanges.Given }

func (c givenCard) Approve(string, int64, string) (string, bool) {
	return c.Given.AuthID, c.Given.Card == "approve"
}

// TestExchanges replays core's payments.authorize exchanges over HTTP.
func TestExchanges(t *testing.T) {
	var f exchanges.File
	if err := exchanges.Load("../core/exchanges/payments.authorize.json", "tribrach.exchanges.v1", &f); err != nil {
		t.Fatal(err)
	}
	for _, k := range f.Cases {
		t.Run(k.ID, func(t *testing.T) {
			srv := httptest.NewServer(Service{Card: givenCard{k.Given}})
			defer srv.Close()
			resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(k.Request))
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != k.Status {
				t.Fatalf("status %d, core says %d (%s)", resp.StatusCode, k.Status, body)
			}
			if k.Status == 200 && !exchanges.SameJSON(body, k.Response) {
				t.Fatalf("answered %s, core says %s", body, k.Response)
			}
		})
	}
}
