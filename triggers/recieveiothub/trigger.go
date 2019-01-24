package recieveiothub

import (

	//"github.com/TIBCOSoftware/flogo-lib/core/action"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/TIBCOSoftware/flogo-lib/core/trigger"
	"github.com/TIBCOSoftware/flogo-lib/logger"
)

var log = logger.GetLogger("recieveiothub")
var handlerMap = make(map[string]*trigger.Handler)
var connestring string
var t *MyTrigger
var connectionString string

const (
	maxIdleConnections int    = 100
	requestTimeout     int    = 10
	tokenValidSecs     int    = 3600
	apiVersion         string = "2016-11-14"
)

//const apiversion string = "2016-11-14"

type sharedAccessKey string
type sharedAccessKeyName string
type hostName string
type deviceID string

type iotHubHTTPClient struct {
	sharedAccessKeyName sharedAccessKeyName
	sharedAccessKey     sharedAccessKey
	hostName            hostName
	deviceID            deviceID
	client              *http.Client
}

// MyTriggerFactory My Trigger factory
type MyTriggerFactory struct {
	metadata *trigger.Metadata
}

// NewFactory create a new Trigger factory
func NewFactory(md *trigger.Metadata) trigger.Factory {
	return &MyTriggerFactory{metadata: md}
}

// New Creates a new trigger instance for a given id
func (t *MyTriggerFactory) New(config *trigger.Config) trigger.Trigger {
	return &MyTrigger{metadata: t.metadata, config: config}
}

// MyTrigger is a stub for your Trigger implementation
type MyTrigger struct {
	metadata *trigger.Metadata
	config   *trigger.Config
	handlers []*trigger.Handler
}

// Initialize implements trigger.Init.Initialize
func (t *MyTrigger) Initialize(ctx trigger.InitContext) error {

	t.handlers = ctx.GetHandlers()
	return nil
}

// Metadata implements trigger.Trigger.Metadata
func (t *MyTrigger) Metadata() *trigger.Metadata {
	return t.metadata
}

var out struct {
	resp   string
	status string
}

// Start implements trigger.Trigger.Start
func (t *MyTrigger) Start() error {

	//connectionString := "HostName=HomeAutoHub.azure-devices.net;DeviceId=RaspberryPi;SharedAccessKey=iQ9YVrPokpJh3QYpQlYa/lI2Gl5YokI6ltsCo9gRQ5Y="

	client, err := newIotHubHTTPClientFromConnectionString("HostName=HomeAutoHub.azure-devices.net;DeviceId=RaspberryPi;SharedAccessKey=iQ9YVrPokpJh3QYpQlYa/lI2Gl5YokI6ltsCo9gRQ5Y=")
	if err != nil {
		log.Error("Error creating http client from connection string", err)
	}
	out.resp, out.status = client.ReceiveMessage()

	t.metadata.Settings["output"].SetValue(out)

	return nil
}

// Stop implements trigger.Trigger.Start
func (t *MyTrigger) Stop() error {
	// stop the trigger
	return nil
}

func newIotHubHTTPClientFromConnectionString(connectionString string) (*iotHubHTTPClient, error) {
	h, k, kn, d, err := parseConnectionString(connectionString)
	if err != nil {
		return nil, err
	}

	return newIotHubHTTPClient(h, kn, k, d), nil
}
func parseConnectionString(connestring string) (hostName, sharedAccessKey, sharedAccessKeyName, deviceID, error) {
	url, err := url.ParseQuery(connestring)
	if err != nil {
		return "", "", "", "", err
	}

	h := tryGetKeyByName(url, "HostName")
	kn := tryGetKeyByName(url, "SharedAccessKeyName")
	k := tryGetKeyByName(url, "SharedAccessKey")
	d := tryGetKeyByName(url, "DeviceId")

	return hostName(h), sharedAccessKey(k), sharedAccessKeyName(kn), deviceID(d), nil
}

func newIotHubHTTPClient(hostNameStr hostName, sharedAccessKeyNameStr sharedAccessKeyName, sharedAccessKeyStr sharedAccessKey, deviceIDStr deviceID) *iotHubHTTPClient {
	return &iotHubHTTPClient{
		sharedAccessKeyName: sharedAccessKeyName(sharedAccessKeyNameStr),
		sharedAccessKey:     sharedAccessKey(sharedAccessKeyStr),
		hostName:            hostName(hostNameStr),
		deviceID:            deviceID(deviceIDStr),
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: maxIdleConnections,
			},
			Timeout: time.Duration(requestTimeout) * time.Second,
		},
	}
}

func tryGetKeyByName(v url.Values, key string) string {
	if len(v[key]) == 0 {
		return ""
	}

	return strings.Replace(v[key][0], " ", "+", -1)
}

func (c *iotHubHTTPClient) ReceiveMessage() (string, string) {
	url := fmt.Sprintf("%s/devices/%s/messages/deviceBound?api-version=%s", c.hostName, c.deviceID, apiVersion)
	return c.performRequest("GET", url, "")

}

func (c *iotHubHTTPClient) performRequest(method string, uri string, data string) (string, string) {
	token := c.buildSasToken(uri)
	fmt.Printf("%s https://%s\n", method, uri)
	req, _ := http.NewRequest(method, "https://"+uri, bytes.NewBufferString(data))
	fmt.Println(data)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "golang-iot-client")
	req.Header.Set("Authorization", token)

	fmt.Println("Authorization:", token)

	if method == "DELETE" {
		req.Header.Set("If-Match", "*")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	// read the entire reply to ensure connection re-use
	text, _ := ioutil.ReadAll(resp.Body)

	io.Copy(ioutil.Discard, resp.Body)
	defer resp.Body.Close()

	return string(text), resp.Status
}

func (c *iotHubHTTPClient) buildSasToken(uri string) string {
	timestamp := time.Now().Unix() + int64(3600)
	encodedURI := template.URLQueryEscaper(uri)

	toSign := encodedURI + "\n" + strconv.FormatInt(timestamp, 10)

	binKey, _ := base64.StdEncoding.DecodeString(string(c.sharedAccessKey))
	mac := hmac.New(sha256.New, []byte(binKey))
	mac.Write([]byte(toSign))

	encodedSignature := template.URLQueryEscaper(base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	if c.sharedAccessKeyName != "" {
		return fmt.Sprintf("SharedAccessSignature sr=%s&sig=%s&se=%d&skn=%s", encodedURI, encodedSignature, timestamp, c.sharedAccessKeyName)
	}

	return fmt.Sprintf("SharedAccessSignature sr=%s&sig=%s&se=%d", encodedURI, encodedSignature, timestamp)
}