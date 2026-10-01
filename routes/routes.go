package routes

import (
	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/httpwire"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

type Route = httpwire.Route

type Mount = dispatch.Mount

var Sentinels = map[string]error{
	"ErrChatNotFound":               mwanachamacomm.ErrChatNotFound,
	"ErrDMNotFound":                 mwanachamacomm.ErrDMNotFound,
	"ErrDMAlreadyActive":            mwanachamacomm.ErrDMAlreadyActive,
	"ErrDMSelfKick":                 mwanachamacomm.ErrDMSelfKick,
	"ErrDMLastAdmin":                mwanachamacomm.ErrDMLastAdmin,
	"ErrDMNotLastToLeave":           mwanachamacomm.ErrDMNotLastToLeave,
	"ErrDMNotAdmin":                 mwanachamacomm.ErrDMNotAdmin,
	"ErrDMInvalidReaction":          mwanachamacomm.ErrDMInvalidReaction,
	"ErrModerationNotFound":         mwanachamacomm.ErrModerationNotFound,
	"ErrAlreadyDecided":             mwanachamacomm.ErrAlreadyDecided,
	"ErrReviewerIsRemover":          mwanachamacomm.ErrReviewerIsRemover,
	"ErrAlreadyRemoved":             mwanachamacomm.ErrAlreadyRemoved,
	"ErrAddressNotFound":            mwanachamacomm.ErrAddressNotFound,
	"ErrAddressBadSettings":         mwanachamacomm.ErrAddressBadSettings,
	"ErrNotificationNotFound":       mwanachamacomm.ErrNotificationNotFound,
	"ErrNotificationNoSuchCategory": mwanachamacomm.ErrNotificationNoSuchCategory,
	"ErrNotificationCategoryExempt": mwanachamacomm.ErrNotificationCategoryExempt,
	"ErrNotificationCapSpent":       mwanachamacomm.ErrNotificationCapSpent,
	"ErrNotificationInvalid":        mwanachamacomm.ErrNotificationInvalid,
	"ErrNotificationMuted":          mwanachamacomm.ErrNotificationMuted,
	"ErrInvalidReference":           mwanachamacomm.ErrInvalidReference,
	"ErrConflict":                   mwanachamacomm.ErrConflict,
}

// AnonymousActions names what any caller may reach without presenting one.
// comm has nothing of the kind: every address here reads or writes somebody's
// own conversations, addresses or notifications. It is the allowlist the
// table is split on and never the list of what is protected, so an operation
// added to the spec and not named here arrives gated.
var AnonymousActions = []string{}

var Table = dispatch.NewTable(mwanachamacomm.Operations(), Sentinels, AnonymousActions...)

func Build(m *mwanachamacomm.CommManager) ([]Route, error) { return Table.Build(m, Mount{}) }

func BuildFor(m *mwanachamacomm.CommManager, mount Mount) ([]Route, error) {
	return Table.Build(m, mount)
}

func Routes(m *mwanachamacomm.CommManager) []Route { return Table.Routes(m, Mount{}) }

func RoutesFor(m *mwanachamacomm.CommManager, mount Mount) []Route { return Table.Routes(m, mount) }

func Split(m *mwanachamacomm.CommManager, mount Mount) dispatch.Split {
	return Table.Split(m, mount)
}

func PublicRoutes(m *mwanachamacomm.CommManager) []Route {
	return Table.Split(m, Mount{}).Anonymous
}

func OperatorRoutes(m *mwanachamacomm.CommManager, mount Mount) []Route {
	return Table.Split(m, mount).Gated
}

func Shape() ([]Route, error) { return Table.Shape() }
