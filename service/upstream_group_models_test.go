package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestManagedChannelModelDiff protects the mirroring contract for a managed
// channel: upstream removals must shrink the local model list (otherwise the
// gateway keeps advertising and routing to a model upstream answers with 404),
// and reordering or an unusable upstream response must never rewrite it.
func TestManagedChannelModelDiff(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		upstream    []string
		wantNext    string
		wantAdded   []string
		wantRemoved []string
		wantChanged bool
	}{
		{
			name:        "identical list keeps the stored value",
			current:     "gpt-6-astra,gpt-6.1-sol",
			upstream:    []string{"gpt-6-astra", "gpt-6.1-sol"},
			wantNext:    "gpt-6-astra,gpt-6.1-sol",
			wantChanged: false,
		},
		{
			name:        "reordered and spaced list is not a change",
			current:     "gpt-6-astra, gpt-6.1-sol,,gpt-6-sol",
			upstream:    []string{"gpt-6.1-sol", "gpt-6-sol", "gpt-6-astra"},
			wantNext:    "gpt-6-astra, gpt-6.1-sol,,gpt-6-sol",
			wantChanged: false,
		},
		{
			name:        "retired model is dropped",
			current:     "gpt-5.6-sol,gpt-6-astra,gpt-6-sol,gpt-6.1-sol",
			upstream:    []string{"gpt-6-astra", "gpt-6.1-sol"},
			wantNext:    "gpt-6-astra,gpt-6.1-sol",
			wantRemoved: []string{"gpt-5.6-sol", "gpt-6-sol"},
			wantChanged: true,
		},
		{
			name:        "new model is added",
			current:     "gpt-6-astra",
			upstream:    []string{"gpt-6-astra", "gpt-6.2-sol"},
			wantNext:    "gpt-6-astra,gpt-6.2-sol",
			wantAdded:   []string{"gpt-6.2-sol"},
			wantChanged: true,
		},
		{
			name:        "additions and removals are reported separately",
			current:     "gpt-6-sol,gpt-6-astra",
			upstream:    []string{"gpt-6-astra", "gpt-6.1-sol", "gpt-6.1-sol"},
			wantNext:    "gpt-6-astra,gpt-6.1-sol",
			wantAdded:   []string{"gpt-6.1-sol"},
			wantRemoved: []string{"gpt-6-sol"},
			wantChanged: true,
		},
		{
			// A hiccup upstream (or a listing filtered down to nothing) must not
			// empty a managed channel: every bound key would lose the group.
			name:        "empty upstream listing never clears the channel",
			current:     "gpt-6-astra,gpt-6.1-sol",
			upstream:    nil,
			wantNext:    "gpt-6-astra,gpt-6.1-sol",
			wantChanged: false,
		},
		{
			name:        "blank upstream entries are ignored",
			current:     "gpt-6-astra",
			upstream:    []string{" ", ""},
			wantNext:    "gpt-6-astra",
			wantChanged: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next, added, removed, changed := managedChannelModelDiff(test.current, test.upstream)
			assert.Equal(t, test.wantChanged, changed)
			assert.Equal(t, test.wantNext, next)
			assert.ElementsMatch(t, test.wantAdded, added)
			assert.ElementsMatch(t, test.wantRemoved, removed)
		})
	}
}
