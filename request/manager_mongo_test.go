package request_test

import (
	"testing"

	"github.com/ory/fosite/handler/oauth2"
	"github.com/stretchr/testify/assert"

	"github.com/matthewhartstonge/storage/request"
)

func TestRequestMongoManager_ImplementsFositeCoreStorageInterface(t *testing.T) {
	r := &request.MongoManager{}

	var i interface{} = r
	_, ok := i.(oauth2.CoreStorage)
	assert.Equal(t, true, ok, "request.MongoManager does not implement interface oauth2.CoreStorage")
}
