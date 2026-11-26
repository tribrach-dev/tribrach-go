// Package payments is the payments service: it serves payments.authorize.
//
// The rules are core's Parking.Payments.authorize, written again in Go.
// The exchanges core exported from the Lean model check that they agree.
package payments

import (
	"io"
	"net/http"

	"example.com/parking-services/contracts"
)

// MaxAmount is the largest charge payments accepts, in cents. Meters relies
// on it: core's Meters.charge_within_limit proves the dearest session fits.
const MaxAmount = 20000

// Card stands in for the card network.
type Card interface {
	// Approve says whether the card behind token pays this amount, and
	// gives the authorization id.
	Approve(token string, amount int64, currency string) (authID string, ok bool)
}

// Service answers payments.authorize.
type Service struct{ Card Card }

// Authorize is the model's authorize. The decoder has already checked the
// amount and currency, so the first branch only matters to Go callers that
// skip the wire.
func (s Service) Authorize(r contracts.AuthorizeRequest) contracts.AuthorizeResponse {
	if r.Amount < 1 || r.Amount > MaxAmount || (r.Currency != "CAD" && r.Currency != "USD") {
		return contracts.AuthorizeResponseDeclined{Reason: "amount or currency not accepted"}
	}
	id, ok := s.Card.Approve(r.Token, r.Amount, r.Currency)
	if !ok {
		return contracts.AuthorizeResponseDeclined{Reason: "card declined"}
	}
	return contracts.AuthorizeResponseAuthorized{AuthID: id, Amount: r.Amount}
}

// ServeHTTP serves POST /payments/authorize.
func (s Service) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	Serve(w, req, contracts.DecodeAuthorizeRequest, s.Authorize, contracts.EncodeAuthorizeResponse)
}

// Serve is the JSON-over-HTTP shape every parking route shares: a request
// that doesn't decode is a 400 and never reaches the model.
func Serve[Q, A any](w http.ResponseWriter, req *http.Request,
	dec func([]byte) (Q, error), handle func(Q) A, enc func(A) ([]byte, error)) {
	if req.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<16))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	q, err := dec(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := enc(handle(q))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}
