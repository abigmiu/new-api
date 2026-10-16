package service

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

// TokenGroupRoutingDecision is the single source of truth for how a managed
// group binding routes. Both the auth middleware (which rejects the request up
// front) and channel selection (which is the fallback when a request reaches
// distribution without the middleware check) map a binding through this, so the
// user-visible failure never depends on which layer noticed first.
type TokenGroupRoutingDecision int

const (
	TokenGroupRoutingActive TokenGroupRoutingDecision = iota
	TokenGroupRoutingPriceChanged
	TokenGroupRoutingChannelRemoved
	TokenGroupRoutingUnavailable
)

// EvaluateTokenGroupRouting classifies a binding view.
//
// Only the upstream price change and the removal of the channel itself carry
// their own message: they are the two cases the user can act on or that the
// operator caused. Everything else (upstream key rejected, admin disabled the
// group, still being provisioned, the user disabled it, or a leftover
// multi-binding token) collapses into a single "unavailable" answer, which
// keeps internal state — and with it the operator's channel identity — private.
func EvaluateTokenGroupRouting(view model.TokenGroupBindingView) TokenGroupRoutingDecision {
	switch view.State {
	case "active":
		return TokenGroupRoutingActive
	case "price_changed":
		return TokenGroupRoutingPriceChanged
	case "group_unavailable":
		if view.GlobalState == model.UpstreamGroupStateGroupRemoved {
			return TokenGroupRoutingChannelRemoved
		}
		return TokenGroupRoutingUnavailable
	default:
		// user_disabled and multiple_bindings, plus any state added later: fail
		// closed rather than guessing that the binding is usable.
		return TokenGroupRoutingUnavailable
	}
}

// TokenGroupUnavailableError carries a routing decision out of channel
// selection. Once auth has admitted the request, this is the last layer that
// can explain why a managed key could not be routed; it exists so the retry
// path in the relay controller answers with the same status, error code and
// message the middleware would have used.
type TokenGroupUnavailableError struct {
	Decision TokenGroupRoutingDecision
}

func (e *TokenGroupUnavailableError) Error() string {
	return "token group is unavailable"
}

// ToNewAPIError renders the decision for the key owner. Messages come from
// interpolation-free templates, so no channel identity and no upstream text can
// appear in the response.
func (e *TokenGroupUnavailableError) ToNewAPIError(c *gin.Context) *types.NewAPIError {
	switch e.Decision {
	case TokenGroupRoutingPriceChanged:
		return types.NewErrorWithStatusCode(
			errors.New(i18n.T(c, i18n.MsgTokenGroupPriceChanged)),
			types.ErrorCodeTokenGroupPriceChanged,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
		)
	case TokenGroupRoutingChannelRemoved:
		return types.NewErrorWithStatusCode(
			errors.New(i18n.T(c, i18n.MsgTokenGroupChannelRemoved)),
			types.ErrorCodeTokenGroupRemoved,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
		)
	default:
		return types.NewErrorWithStatusCode(
			errors.New(i18n.T(c, i18n.MsgTokenGroupUnavailable)),
			types.ErrorCodeTokenGroupUnavailable,
			http.StatusServiceUnavailable,
			types.ErrOptionWithSkipRetry(),
		)
	}
}
