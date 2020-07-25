package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBaseName(t *testing.T) {
	tests := []struct {
		name   string
		expect string
	}{
		{
			name:   namePrefix + "." + "aaa",
			expect: "aaa",
		},
	}

	for _, test := range tests {
		t.Log(test.name)
		assert.Equal(t, test.expect, extractBaseName(test.name))
	}
}
