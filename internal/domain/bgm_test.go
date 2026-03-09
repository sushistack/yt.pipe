package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateLicenseType_Valid(t *testing.T) {
	validTypes := []LicenseType{
		LicenseRoyaltyFree,
		LicenseCCBY,
		LicenseCCBYSA,
		LicenseCCBYNC,
		LicenseCustom,
	}
	for _, lt := range validTypes {
		assert.NoError(t, ValidateLicenseType(lt), "expected %q to be valid", lt)
	}
}

func TestValidateLicenseType_Invalid(t *testing.T) {
	err := ValidateLicenseType("invalid_type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid license type")
}

func TestValidateLicenseType_Empty(t *testing.T) {
	err := ValidateLicenseType("")
	assert.Error(t, err)
}
