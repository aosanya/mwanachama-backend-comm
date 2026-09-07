// address.go — the two address operations the survey found portable:
// listMyAddresses and retireAddress, both a plain call against
// mwanachamacomm.AddressRepository with the caller's own id from [Identity] and no
// gateway-only policy composed into the handler body. Every other address
// route (publish, resolve, block, settings, the public-address cap, the
// directory search) reaches phonesalt and/or orgpolicy in-body and stays in
// the gateway — see doc.go.
package routes

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/aosanya/mwanachama-backend-comm"
)

func addressStatusFor(err error) int {
	switch {
	case errors.Is(err, mwanachamacomm.ErrAddressNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func writeAddressErr(w http.ResponseWriter, err error) {
	writeErr(w, addressStatusFor(err), err.Error())
}

// AddressRoutes is listMyAddresses and retireAddress, at the same paths the
// gateway already serves them at.
func AddressRoutes(addr mwanachamacomm.AddressRepository, identity Identity) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/v1/dm/addresses", Handler: ListMyAddresses(addr, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/addresses/{index}/retire", Handler: RetireAddress(addr, identity)},
	}
}

// ListMyAddresses handles GET /v1/dm/addresses — the caller's own view of
// every address they hold, retired included.
func ListMyAddresses(addr mwanachamacomm.AddressRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := addr.ListFor(r.Context(), identity.CallerID(r))
		if err != nil {
			writeAddressErr(w, err)
			return
		}
		mine := make([]mwanachamacomm.AddressMine, len(out))
		for i, a := range out {
			mine[i] = a.Mine()
		}
		writeJSON(w, http.StatusOK, mine)
	}
}

// RetireAddress handles POST /v1/dm/addresses/{index}/retire — stops one of
// the caller's own addresses accepting new threads. Threads already open
// under it keep working.
func RetireAddress(addr mwanachamacomm.AddressRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		index, err := strconv.Atoi(r.PathValue("index"))
		if err != nil || index < 0 {
			writeErr(w, http.StatusBadRequest, "that is not an address index")
			return
		}
		if err := addr.Retire(r.Context(), identity.CallerID(r), index, time.Now().UTC()); err != nil {
			if errors.Is(err, mwanachamacomm.ErrAddressNotFound) {
				writeErr(w, http.StatusNotFound, "no live address of yours at that index")
				return
			}
			writeAddressErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
