package blacklist

import (
	"testing"
	"fmt"
	"github.com/stretchr/testify/assert"
)

var db *blacklistDB

func init() {
	fmt.Println("init")
	db = newBlacklistDB("")
}

func TestNewMemoryBlacklistDB(t *testing.T) {
	db.SetCurrentHeight(100)
	assert.Equal(t, int64(100), db.currentHeight)
}
