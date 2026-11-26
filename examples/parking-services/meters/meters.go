// Package meters is the meters service: it serves meters.start v2, and v1
// on the v1 route, and calls payments to authorize each charge.
//
// The rules are core's Parking.Meters, written again in Go. The exchanges
// core exported from the Lean model check that they agree, including the
// call meters makes to payments.
package meters

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"example.com/parking-services/contracts"
	"example.com/parking-services/payments"
)

// Rates are cents per 15 minutes, by zone.
var Rates = map[string]int64{"A1": 125, "B2": 75, "C3": 50}

// Charge prices a session. The decoder guarantees 15 to 240 minutes in
// steps of 15, so the dearest is 125 × 16 = 2,000 cents, under
// payments.MaxAmount (core proves this: Meters.charge_within_limit).
func Charge(rate, minutes int64) int64 { return rate * (minutes / 15) }

// Payments is how meters reaches payments.authorize.
type Payments interface {
	Authorize(contracts.AuthorizeRequest) (contracts.AuthorizeResponse, error)
}

// Service answers meters.start.
type Service struct {
	Now          func() time.Time
	NewSessionID func() string
	Token        string // the payment token on file
	Pay          Payments
	ChargersFree func() int
}

// Start is the model's start (v1).
func (s Service) Start(r contracts.StartRequest) contracts.StartResponse {
	rate, ok := Rates[r.Zone]
	if !ok {
		return contracts.StartResponseRejected{Reason: "unknown zone"}
	}
	amount := Charge(rate, r.Minutes)
	ans, err := s.Pay.Authorize(contracts.AuthorizeRequest{Amount: amount, Currency: "CAD", Token: s.Token})
	if err != nil {
		return contracts.StartResponseRejected{Reason: "payments unavailable"}
	}
	switch a := ans.(type) {
	case contracts.AuthorizeResponseAuthorized:
		if a.Amount != amount {
			return contracts.StartResponseRejected{Reason: "payment did not match the charge"}
		}
		now := s.Now().UTC().Truncate(time.Second)
		return contracts.StartResponseStarted{
			SessionID: s.NewSessionID(),
			StartsAt:  now,
			ExpiresAt: now.Add(time.Duration(r.Minutes) * time.Minute),
			Charged:   amount,
		}
	case contracts.AuthorizeResponseDeclined:
		return contracts.StartResponseRejected{Reason: a.Reason}
	}
	panic("unreachable")
}

// StartV2 is the model's startV2.
func (s Service) StartV2(r contracts.StartRequestV2) contracts.StartResponseV2 {
	if r.EvCharging != nil && *r.EvCharging && s.ChargersFree() == 0 {
		return contracts.StartResponseV2Queued{SessionID: s.NewSessionID(), Position: 1}
	}
	switch x := s.Start(contracts.StartRequest{Zone: r.Zone, Plate: r.Plate, Minutes: r.Minutes}).(type) {
	case contracts.StartResponseStarted:
		return contracts.StartResponseV2Started(x)
	case contracts.StartResponseRejected:
		return contracts.StartResponseV2Rejected(x)
	}
	panic("unreachable")
}

// Upgrade reads a v1 request as v2: no EV charging asked.
func Upgrade(r contracts.StartRequest) contracts.StartRequestV2 {
	return contracts.StartRequestV2{Zone: r.Zone, Plate: r.Plate, Minutes: r.Minutes}
}

// Downgrade is how the v1 route sends a v2 answer. Core proves queued can't
// happen for an upgraded v1 request (Meters.v1_never_queued).
func Downgrade(x contracts.StartResponseV2) contracts.StartResponse {
	switch x := x.(type) {
	case contracts.StartResponseV2Started:
		return contracts.StartResponseStarted(x)
	case contracts.StartResponseV2Queued:
		return contracts.StartResponseRejected{Reason: "waiting for a free charger"}
	case contracts.StartResponseV2Rejected:
		return contracts.StartResponseRejected(x)
	}
	panic("unreachable")
}

// Routes serves POST /v1/start through the v2 rules, as core's
// Meters.v2_serves_v1 proves is safe, and POST /v2/start.
func (s Service) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/start", func(w http.ResponseWriter, req *http.Request) {
		payments.Serve(w, req, contracts.DecodeStartRequest,
			func(r contracts.StartRequest) contracts.StartResponse { return Downgrade(s.StartV2(Upgrade(r))) },
			contracts.EncodeStartResponse)
	})
	mux.HandleFunc("/v2/start", func(w http.ResponseWriter, req *http.Request) {
		payments.Serve(w, req, contracts.DecodeStartRequestV2, s.StartV2, contracts.EncodeStartResponseV2)
	})
	return mux
}

// Client calls payments over HTTP.
type Client struct {
	URL  string // such as http://payments/authorize
	HTTP *http.Client
}

// Authorize sends one payments.authorize request.
func (c Client) Authorize(r contracts.AuthorizeRequest) (contracts.AuthorizeResponse, error) {
	body, err := contracts.EncodeAuthorizeRequest(r)
	if err != nil {
		return nil, err
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Post(c.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("payments: %s: %s", resp.Status, bytes.TrimSpace(data))
	}
	return contracts.DecodeAuthorizeResponse(data)
}
