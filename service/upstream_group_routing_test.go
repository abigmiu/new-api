package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

// This table is the contract the auth middleware and channel selection both
// depend on: which reasons reach the key owner with their own message, and
// which collapse into a generic "unavailable" that reveals no internal state.
func TestEvaluateTokenGroupRouting(t *testing.T) {
	tests := []struct {
		name string
		view model.TokenGroupBindingView
		want TokenGroupRoutingDecision
	}{
		{
			name: "active binding routes",
			view: model.TokenGroupBindingView{State: "active", GlobalState: model.UpstreamGroupStateActive},
			want: TokenGroupRoutingActive,
		},
		{
			name: "price change gets its own message",
			view: model.TokenGroupBindingView{State: "price_changed", GlobalState: model.UpstreamGroupStateActive},
			want: TokenGroupRoutingPriceChanged,
		},
		{
			name: "removed upstream group is reported as a removed channel",
			view: model.TokenGroupBindingView{State: "group_unavailable", GlobalState: model.UpstreamGroupStateGroupRemoved},
			want: TokenGroupRoutingChannelRemoved,
		},
		{
			name: "admin disabled group is generic unavailable",
			view: model.TokenGroupBindingView{State: "group_unavailable", GlobalState: model.UpstreamGroupStateAdminDisabled},
			want: TokenGroupRoutingUnavailable,
		},
		{
			name: "provisioning group is generic unavailable",
			view: model.TokenGroupBindingView{State: "group_unavailable", GlobalState: model.UpstreamGroupStateProvisioning},
			want: TokenGroupRoutingUnavailable,
		},
		{
			name: "discovered group is generic unavailable",
			view: model.TokenGroupBindingView{State: "group_unavailable", GlobalState: model.UpstreamGroupStateDiscovered},
			want: TokenGroupRoutingUnavailable,
		},
		{
			name: "unavailable upstream key is generic unavailable",
			view: model.TokenGroupBindingView{State: "group_unavailable", GlobalState: model.UpstreamGroupStateKeyUnavailable},
			want: TokenGroupRoutingUnavailable,
		},
		{
			name: "errored group is generic unavailable",
			view: model.TokenGroupBindingView{State: "group_unavailable", GlobalState: model.UpstreamGroupStateError},
			want: TokenGroupRoutingUnavailable,
		},
		{
			name: "group_unavailable without a global state is generic unavailable",
			view: model.TokenGroupBindingView{State: "group_unavailable"},
			want: TokenGroupRoutingUnavailable,
		},
		{
			name: "user disabled binding is generic unavailable",
			view: model.TokenGroupBindingView{State: "user_disabled", GlobalState: model.UpstreamGroupStateActive},
			want: TokenGroupRoutingUnavailable,
		},
		{
			name: "legacy multi binding is generic unavailable",
			view: model.TokenGroupBindingView{State: model.TokenGroupStateMultipleBindings, GlobalState: model.UpstreamGroupStateActive},
			want: TokenGroupRoutingUnavailable,
		},
		{
			name: "unknown state fails closed",
			view: model.TokenGroupBindingView{State: "something_new", GlobalState: model.UpstreamGroupStateActive},
			want: TokenGroupRoutingUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, EvaluateTokenGroupRouting(test.view))
		})
	}
}
