package cli

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestValidAssetTypes(t *testing.T) {
	assert.True(t, domain.ValidAssetTypes["image"])
	assert.True(t, domain.ValidAssetTypes["audio"])
	assert.True(t, domain.ValidAssetTypes["subtitle"])
	assert.True(t, domain.ValidAssetTypes["scenario"])
	assert.False(t, domain.ValidAssetTypes["video"])
}

func TestValidRatings(t *testing.T) {
	assert.True(t, domain.ValidRatings["good"])
	assert.True(t, domain.ValidRatings["bad"])
	assert.True(t, domain.ValidRatings["neutral"])
	assert.False(t, domain.ValidRatings["excellent"])
}
