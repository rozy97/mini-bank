package usecases

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuccessCase(t *testing.T) {
	assert.Equal(t, 1, 1)
}

func TestFailedCase(t *testing.T) {
	assert.Equal(t, 1, 2)
}
