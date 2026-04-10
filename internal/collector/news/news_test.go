package news

import (
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestNewsCollector_IsRelevant(t *testing.T) {
	collector := &NewsCollector{}

	tests := []struct {
		name     string
		title    string
		desc     string
		expected bool
	}{
		{
			"Relevant construction",
			"New Industrial Park Construction in Richmond",
			"A large warehouse expansion is underway at YVR.",
			true,
		},
		{
			"Relevant infrastructure",
			"New Road Infrastructure Project Awarded",
			"The contract for the highway expansion has been signed.",
			true,
		},
		{
			"Relevant permit",
			"City of Richmond Issues Building Permit for High-Rise",
			"Residential development project moves forward.",
			true,
		},
		{
			"Irrelevant sports",
			"Richmond Hockey Team Wins Game",
			"The local team had a great score in the final match.",
			false,
		},
		{
			"Irrelevant politics",
			"Election Campaign Starts Today",
			"Politicians discuss various topics in the upcoming election.",
			false,
		},
		{
			"Mixed content with negative keyword",
			"Construction project hits snag after election win",
			"The construction of the building is delayed due to campaign changes.",
			false,
		},
		{
			"Generic news",
			"Sunny weather in Vancouver today",
			"Residents enjoy the outdoors at local parks.",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &gofeed.Item{
				Title:       tt.title,
				Description: tt.desc,
			}
			assert.Equal(t, tt.expected, collector.isRelevant(item))
		})
	}
}
