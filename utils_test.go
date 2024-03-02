package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDecodeLpaActivationCode(t *testing.T) {
	var info PullInfo
	var codeNeeded bool
	var err error
	_, _, err = DecodeLpaActivationCode("")
	assert.Error(t, err)
	_, _, err = DecodeLpaActivationCode("LPA:")
	assert.Error(t, err)
	_, _, err = DecodeLpaActivationCode("LPA:1")
	assert.Error(t, err)
	info, _, err = DecodeLpaActivationCode("LPA:1$example.com")
	assert.NoError(t, err)
	assert.Equal(t, "example.com", info.SMDP)
	info, _, err = DecodeLpaActivationCode("LPA:1$example.com$matching-id")
	assert.Equal(t, "example.com", info.SMDP)
	assert.Equal(t, "matching-id", info.MatchID)
	info, codeNeeded, err = DecodeLpaActivationCode("LPA:1$example.com$matching-id$$1")
	assert.Equal(t, "example.com", info.SMDP)
	assert.Equal(t, "matching-id", info.MatchID)
	assert.True(t, codeNeeded, "Confirm Code Required Flag")
}

func TestCompleteActivationCode(t *testing.T) {
	const lpaString = "LPA:1$example.com$matching-id"
	assert.Equal(t, lpaString, CompleteActivationCode("LPA:1$example.com$matching-id"))
	assert.Equal(t, lpaString, CompleteActivationCode("1$example.com$matching-id"))
	assert.Equal(t, lpaString, CompleteActivationCode("$example.com$matching-id"))
	assert.Equal(t, lpaString, CompleteActivationCode("example.com$matching-id"))
}
