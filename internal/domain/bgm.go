package domain

import (
	"fmt"
	"time"
)

// LicenseType represents a BGM license category.
type LicenseType string

const (
	LicenseRoyaltyFree LicenseType = "royalty_free"
	LicenseCCBY        LicenseType = "cc_by"
	LicenseCCBYSA      LicenseType = "cc_by_sa"
	LicenseCCBYNC      LicenseType = "cc_by_nc"
	LicenseCustom      LicenseType = "custom"
)

// ValidLicenseTypes contains all allowed license type values.
var ValidLicenseTypes = map[LicenseType]bool{
	LicenseRoyaltyFree: true,
	LicenseCCBY:        true,
	LicenseCCBYSA:      true,
	LicenseCCBYNC:      true,
	LicenseCustom:      true,
}

// ValidateLicenseType checks if the given license type is valid.
func ValidateLicenseType(lt LicenseType) error {
	if !ValidLicenseTypes[lt] {
		return fmt.Errorf("invalid license type %q: allowed values are royalty_free, cc_by, cc_by_sa, cc_by_nc, custom", lt)
	}
	return nil
}

// BGM represents a background music file with mood tags and license metadata.
type BGM struct {
	ID            string
	Name          string
	FilePath      string
	MoodTags      []string
	DurationMs    int64
	LicenseType   LicenseType
	LicenseSource string
	CreditText    string
	CreatedAt     time.Time
}

// SceneBGMAssignment represents a BGM assigned to a specific scene in a project.
type SceneBGMAssignment struct {
	ProjectID       string
	SceneNum        int
	BGMID           string
	VolumeDB        float64
	FadeInMs        int
	FadeOutMs       int
	DuckingDB       float64
	AutoRecommended bool
	Confirmed       bool
}
