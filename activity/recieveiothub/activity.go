package recieveiothub

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TIBCOSoftware/flogo-lib/core/activity"
	"github.com/TIBCOSoftware/flogo-lib/core/trigger"

	"github.com/TIBCOSoftware/flogo-lib/logger"
)

const (
	maxIdleConnections int    = 100
	requestTimeout     int    = 10
	tokenValidSecs     int    = 3600
	apiVersion         string = "2018-06-30"
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

var log = logger.GetLogger("recieveiothub")
var handlerMap = make(map[string]*trigger.Handler)
var connestring string

var connectionString string

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

var out struct {
	resp   string
	status string
}

// Eval implements activity.Activity.Eval
func (a *MyActivity) Eval(context activity.Context) (done bool, err error) {

	client, err := newIotHubHTTPClientFromConnectionString("HostName=HomeAutoHub.azure-devices.net;DeviceId=RaspberryPi;SharedAccessKey=iQ9YVrPokpJh3QYpQlYa/lI2Gl5YokI6ltsCo9gRQ5Y=")
	if err != nil {
		log.Error("Error creating http client from connection string", err)
	}
	out.resp, out.status = client.ReceiveMessage()
	log.Debug(out.resp)
	context.SetOutput("output", out.resp)

	return true, nil
}

func parseConnectionString(connString string) (hostName, sharedAccessKey, sharedAccessKeyName, deviceID, error) {
	url, err := url.ParseQuery(connString)
	if err != nil {
		return "", "", "", "", err
	}

	h := tryGetKeyByName(url, "HostName")
	kn := tryGetKeyByName(url, "SharedAccessKeyName")
	k := tryGetKeyByName(url, "SharedAccessKey")
	d := tryGetKeyByName(url, "DeviceId")

	return hostName(h), sharedAccessKey(k), sharedAccessKeyName(kn), deviceID(d), nil
}

func tryGetKeyByName(v url.Values, key string) string {
	if len(v[key]) == 0 {
		return ""
	}

	return strings.Replace(v[key][0], " ", "+", -1)
}

// NewIotHubHTTPClient is a constructor of IutHubClient
func newIotHubHTTPClient(hostName hostName, sharedAccessKeyName sharedAccessKeyName, sharedAccessKey sharedAccessKey, deviceID deviceID) *iotHubHTTPClient {
	return &iotHubHTTPClient{
		sharedAccessKeyName: sharedAccessKeyName,
		sharedAccessKey:     sharedAccessKey,
		hostName:            hostName,
		deviceID:            deviceID,
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: maxIdleConnections,
			},
			Timeout: time.Duration(requestTimeout) * time.Second,
		},
	}
}

// NewIotHubHTTPClientFromConnectionString creates new client from connection string
func newIotHubHTTPClientFromConnectionString(connectionString string) (*iotHubHTTPClient, error) {
	h, k, kn, d, err := parseConnectionString(connectionString)
	if err != nil {
		return nil, err
	}

	return newIotHubHTTPClient(h, kn, k, d), nil
}

// IsDevice tell either device id was specified when client created.
// If device id was specified in connection string this will enabled device scoped requests.
func (c *iotHubHTTPClient) IsDevice() bool {
	return c.deviceID != ""
}

// Device API

// SendMessage from a logged in device
func (c *iotHubHTTPClient) ReceiveMessage() (string, string) {
	url := fmt.Sprintf("%s/devices/%s/messages/deviceBound?api-version=%s", c.hostName, c.deviceID, apiVersion)
	return c.performRequest("GET", url, "")
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
		return fmt.Sprintf("SharedAccessSignature sig=%s&se=%d&skn=%s&sr=%s", encodedSignature, timestamp, c.sharedAccessKeyName, encodedURI)
	}

	return fmt.Sprintf("SharedAccessSignature sig=%s&se=%d&sr=%s", encodedSignature, timestamp, encodedURI)
}

func (c *iotHubHTTPClient) performRequest(method string, uri string, data string) (string, string) {
	token := c.buildSasToken(uri)
	//log.("%s https://%s\n", method, uri)
	log.Debug(token)
	req, _ := http.NewRequest(method, "https://"+uri, bytes.NewBufferString(data))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "golang-iot-client")
	req.Header.Set("Authorization", token)

	//log.Println("Authorization:", token)

	if method == "DELETE" {
		req.Header.Set("If-Match", "*")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		log.Error(err)
	}

	// read the entire reply to ensure connection re-use
	text, _ := ioutil.ReadAll(resp.Body)

	io.Copy(ioutil.Discard, resp.Body)
	defer resp.Body.Close()

	return string(text), resp.Status
}
