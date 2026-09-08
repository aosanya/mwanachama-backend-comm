package routes

import "net/http"

type Identity interface {
	CallerID(r *http.Request) string
	// CallerDeviceID returns the device id the caller's session carries, or
	// "" — a session minted by the phone flow carries no device, and that
	// is a legitimate answer, not a missing one.
	CallerDeviceID(r *http.Request) string
}
