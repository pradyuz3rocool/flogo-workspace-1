package dht11

import (
	"fmt"

	"github.com/TIBCOSoftware/flogo-lib/core/activity"
	"github.com/TIBCOSoftware/flogo-lib/logger"
	"github.com/pakohan/dht"
)

const (
	ivType     = "type"
	ivPin      = "pin"
	ovTemp     = "temp"
	ovHumidity = "humidity"
)

var log = logger.GetLogger("sensor_dht")

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

	deviceType := context.GetInput(ivType).(string)
	gpioPin := context.GetInput(ivPin).(int)

	sensorType := dht.SensorDHT22

	if deviceType == "DHT11" {
		sensorType = dht.SensorDHT11
	}

	humidity, temperature, err := dht.GetSensorData(sensorType, gpioPin)

	if err != nil {
		log.Error(err)
		return false, err
	}

	log.Debugf("DHT Sensor returned [%v] temperature and [%v] humidity", temperature, humidity)

	context.SetOutput(ovTemp, fmt.Sprint(temperature))
	context.SetOutput(ovHumidity, fmt.Sprint(humidity))
	return true, nil
}
