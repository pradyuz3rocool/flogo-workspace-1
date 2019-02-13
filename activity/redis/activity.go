package redis

import (
	"github.com/TIBCOSoftware/flogo-lib/core/activity"
	"github.com/hoisie/redis"
)

// MyActivity is a stub for your Activity implementation
type MyActivity struct {
	metadata *activity.Metadata
}

// NewActivity creates a new activity
func NewActivity(metadata *activity.Metadata) activity.Activity {
	return &MyActivity{metadata: metadata}
}

// Metadata implements activity.Activity.Metadata
func (a *MyActivity) Metadata() *activity.Metadata {
	return a.metadata
}

// Eval implements activity.Activity.Eval
func (a *MyActivity) Eval(context activity.Context) (done bool, err error) {

	var client redis.Client
	key := context.GetInput("key").(string)
	vals := context.GetInput("value").([]string)

	for _, v := range vals {
		client.Rpush(key, []byte(v))
	}
	dbvals, _ := client.Lrange(key, 0, 100)
	for i, v := range dbvals {
		println(i, ":", string(v))
		context.SetOutput("output[i]", string(v[i]))
	}

	return true, nil
}
