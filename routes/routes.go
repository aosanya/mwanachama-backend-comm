package routes

import (
	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/httpwire"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

type Route = httpwire.Route

type Mount = dispatch.Mount

var Sentinels = map[string]error{
	"ErrConsentRequired":            mwanachamacomm.ErrConsentRequired,
	"ErrConsentWithdrawn":           mwanachamacomm.ErrConsentWithdrawn,
	"ErrChatNotFound":               mwanachamacomm.ErrChatNotFound,
	"ErrChatInvalid":                mwanachamacomm.ErrChatInvalid,
	"ErrChatNotDeclinable":          mwanachamacomm.ErrChatNotDeclinable,
	"ErrChatThreadDeclined":         mwanachamacomm.ErrChatThreadDeclined,
	"ErrChatQuestionRoom":           mwanachamacomm.ErrChatQuestionRoom,
	"ErrSubjectNotFound":            mwanachamacomm.ErrSubjectNotFound,
	"ErrSubjectInvalid":             mwanachamacomm.ErrSubjectInvalid,
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
	"ErrNotificationNotRaisable":    mwanachamacomm.ErrNotificationNotRaisable,
	"ErrInvalidReference":           mwanachamacomm.ErrInvalidReference,
	"ErrConflict":                   mwanachamacomm.ErrConflict,
	"ErrPushSubscriptionInvalid":    mwanachamacomm.ErrPushSubscriptionInvalid,
	"ErrStandingInvalid":            mwanachamacomm.ErrStandingInvalid,
	"ErrStandingNotFound":           mwanachamacomm.ErrStandingNotFound,
	"ErrStandingStopped":            mwanachamacomm.ErrStandingStopped,
}

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
