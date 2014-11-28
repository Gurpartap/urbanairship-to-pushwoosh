package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var debug = false
var dumpDir string

type UrbanAirship struct {
	AppKey       string
	MasterSecret string

	TokensLimitPerRequest int    // Optional. A maximum value of only 10000 is accepted.
	StartingTokenId       string // Optional. Token Id is not the same as device_token.
}

type PushWoosh struct {
	ApiKey  string
	AppCode string

	DefaultDeviceType PWDeviceType
	DefaultLanguage   string
	DefaultTimezone   float64
}

var config = struct {
	UrbanAirship
	PushWoosh
}{
	UrbanAirship{
		"URBANAIRSHIP_APP_KEY",
		"URBANAIRSHIP_MASTER_SECRET",

		1000,
		"",
	},
	PushWoosh{
		"PUSHWOOSH_API_KEY",
		"PUSHWOOSH_APP_CODE",

		PWDeviceType(iOS),
		"",
		0,
	},
}

type UADeviceToken struct {
	Active      bool          `json:"active"`
	Alias       interface{}   `json:"alias"`
	Created     string        `json:"created"`
	DeviceToken string        `json:"device_token"`
	Tags        []interface{} `json:"tags"`
}

type UADeviceTokensResponse struct {
	ActiveDeviceTokensCount float64         `json:"active_device_tokens_count"`
	DeviceTokensCount       float64         `json:"device_tokens_count"`
	DeviceTokens            []UADeviceToken `json:"device_tokens"`
	NextPage                string          `json:"next_page,omitempty"`
}

type PWDeviceType int

const (
	iOS PWDeviceType = 1 + iota
	BB
	Android
	Nokia_ASHA
	Windows_Phone
	// There is no device type 6.
	OS_X = 2 + iota
	Windows_8
	Amazon
	Safari
)

type PWRegisterDevice struct {
	Request PWRegisterDeviceRequest `json:"request"`
}

type PWRegisterDeviceRequest struct {
	Auth        string       `json:"auth"`
	Application string       `json:"application"`
	DeviceType  PWDeviceType `json:"device_type"`
	Hwid        string       `json:"hwid"`
	PushToken   string       `json:"push_token"`
	Language    string       `json:"language,omitempty"`
	Timezone    float64      `json:"timezone,omitempty"`
}

type PWDeviceRegisterResponse struct {
	StatusCode    int         `json:"status_code"`
	StatusMessage string      `json:"status_message"`
	Response      interface{} `json:"response",omitempty`
}

func GetDeviceTokensFromUrbanAirship(pending chan<- *UADeviceToken) {
	if debug {
		println("Entered GetDeviceTokensFromUrbanAirship(tokenChannel chan UADeviceToken)")
	}

	GetDeviceTokensFromUrbanAirshipWithURL := func(url string, deviceTokenResp *UADeviceTokensResponse) {
		req, _ := http.NewRequest("GET", url, nil)
		req.SetBasicAuth(config.UrbanAirship.AppKey, config.UrbanAirship.MasterSecret)
		req.Header.Add("Content-type", "application/json")
		req.Header.Add("Accept", "application/vnd.urbanairship+json; version=3;")

		client := &http.Client{}
		resp, _ := client.Do(req)
		respBody, _ := ioutil.ReadAll(resp.Body)

		json.Unmarshal(respBody, &deviceTokenResp)

		if debug {
			println("Exiting GetDeviceTokensFromUrbanAirshipWithURL(url string, deviceTokenResp *UADeviceTokensResponse)")
		}
	}

	nextPage := "https://go.urbanairship.com/api/device_tokens/?"

	if config.UrbanAirship.TokensLimitPerRequest > 0 {
		nextPage += "&limit=" + strconv.Itoa(config.UrbanAirship.TokensLimitPerRequest)
	}
	if len(config.UrbanAirship.StartingTokenId) > 0 {
		nextPage += "&start=" + config.UrbanAirship.StartingTokenId
	}

	var deviceTokens = []UADeviceToken{}

	for len(nextPage) > 0 {
		var deviceTokenResp UADeviceTokensResponse
		GetDeviceTokensFromUrbanAirshipWithURL(nextPage, &deviceTokenResp)

		deviceTokens = append(deviceTokens, deviceTokenResp.DeviceTokens...)
		dir := dumpDir + "/urbanairship/" + strconv.Itoa(len(deviceTokens))
		os.MkdirAll(dir, 0744)
		txt, _ := json.MarshalIndent(deviceTokenResp, "", "\t")
		ioutil.WriteFile(dir+"/device_tokens.txt", txt, 0644)

		for _, deviceToken := range deviceTokens {
			go func() { pending <- &deviceToken }()
		}

		if len(deviceTokenResp.NextPage) > 0 {
			nextPage = deviceTokenResp.NextPage
		} else {
			nextPage = ""
		}
	}

	txt, _ := json.MarshalIndent(deviceTokens, "", "\t")
	ioutil.WriteFile(dumpDir+"/urbanairship.txt", txt, 0644)

	go func() {
		pending <- &UADeviceToken{
			Active:      false,
			Alias:       nil,
			Created:     "",
			DeviceToken: "",
		}
	}()

	if debug {
		println("Exiting GetDeviceTokensFromUrbanAirship(tokenChannel chan UADeviceToken)")
	}
}

func PostDeviceTokensToPushWoosh(pending <-chan *UADeviceToken, status chan<- State, done chan struct{}) {
	if debug {
		println("Entered PostDeviceTokensToPushWoosh(tokenChannel chan UADeviceToken)")
	}

	for deviceToken := range pending {
		if debug {
			println("Attempting to register device...")
		}

		if debug {
			println("Device Token: " + deviceToken.DeviceToken)
		}

		if deviceToken.DeviceToken == "" && deviceToken.Active == false && deviceToken.Created == "" {
			close(done)
		}

		PostDeviceTokenToPushWoosh := func(registerDevice PWRegisterDevice, deviceRegisterResp *PWDeviceRegisterResponse) {
			jsonBody, _ := json.Marshal(registerDevice)
			body := strings.NewReader(string(jsonBody))
			req, _ := http.NewRequest("POST", "https://cp.pushwoosh.com/json/1.3/registerDevice", body)
			req.Header.Add("Content-type", "application/json")
			req.Header.Add("Accept", "application/json")

			client := &http.Client{}
			resp, _ := client.Do(req)
			respBody, _ := ioutil.ReadAll(resp.Body)

			json.Unmarshal(respBody, &deviceRegisterResp)
		}

		if deviceToken.Active {
			registerDevice := PWRegisterDevice{
				Request: PWRegisterDeviceRequest{
					Auth:        config.PushWoosh.ApiKey,
					Application: config.PushWoosh.AppCode,
					DeviceType:  config.PushWoosh.DefaultDeviceType,
					Language:    config.PushWoosh.DefaultLanguage,
					Timezone:    config.PushWoosh.DefaultTimezone,
					Hwid:        deviceToken.DeviceToken, // UrbanAirship does not store a UDID. Use the token instead.
					PushToken:   deviceToken.DeviceToken,
				},
			}

			var deviceRegisterResp PWDeviceRegisterResponse
			PostDeviceTokenToPushWoosh(registerDevice, &deviceRegisterResp)
			if deviceRegisterResp.StatusCode != 200 || deviceRegisterResp.StatusMessage != "OK" {
				r, _ := json.MarshalIndent(deviceRegisterResp, "", "\t")
				println("\nFailed to register device with token:")
				println("\n\t" + registerDevice.Request.PushToken)
				println("\nResponse from PushWoosh:\n")
				os.Stdout.Write(r)
				close(done)
			} else {
				if debug {
					r, _ := json.MarshalIndent(deviceRegisterResp, "", "\t")
					os.Stdout.Write(r)
					println()
				}
			}
			status <- State{"SENT", deviceToken.DeviceToken}
		} else {
			status <- State{"INACTIVE", deviceToken.DeviceToken}
		}

		if debug {
			println("Device registration complete.")
		}
	}

	if debug {
		println("Exiting PostDeviceTokensToPushWoosh(tokenChannel chan UADeviceToken)")
	}
}

type State struct {
	Status      string `json:"status"`
	DeviceToken string `json:"device_token"`
}

func StateMonitor(updateInterval time.Duration, pending chan *UADeviceToken) chan<- State {
	updates := make(chan State)
	tokenStatus := make(map[string][]string)
	ticker := time.NewTicker(updateInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				println("Current state:")
				for k, v := range tokenStatus {
					println(k, ":", len(v))
				}
			case t := <-updates:
				tokenStatus[t.Status] = append(tokenStatus[t.Status], t.DeviceToken)

				// file, _ := os.OpenFile(dumpDir+"/pushwoosh.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				// defer file.Close()
				// jsonStr, _ := json.MarshalIndent(t, "", "\t")
				// file.WriteString(string(jsonStr) + ",\n")
			}
		}
	}()
	return updates
}

func main() {
	if debug {
		println("Entered main()")
	}

	dumpDir = "./dump/" + strconv.FormatInt(time.Now().Unix(), 10)

	pending := make(chan *UADeviceToken)
	done := make(chan struct{})

	status := StateMonitor(5*time.Second, pending)

	go GetDeviceTokensFromUrbanAirship(pending)
	go PostDeviceTokensToPushWoosh(pending, status, done)

	<-done

	println("All done. Bye.")

	if debug {
		println("Exiting main()")
	}
}
