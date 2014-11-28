package main

import (
	"encoding/json"
	"fmt"
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
	StartingTokenId       string // Optional. UA Token Id is not the same as device_token.
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

type State struct {
	Status      string `json:"status"`
	DeviceToken string `json:"device_token"`
}

var activeTokensCount float64 = 0
var tokensCount float64 = 0
var downloadedTokensCount float64 = 0

func GetDeviceTokensFromUrbanAirship(pending chan<- UADeviceToken, done chan bool) {
	GetDeviceTokensFromUrbanAirshipWithURL := func(url string, deviceTokenResp *UADeviceTokensResponse) {
		req, _ := http.NewRequest("GET", url, nil)
		req.SetBasicAuth(config.UrbanAirship.AppKey, config.UrbanAirship.MasterSecret)
		req.Header.Add("Content-type", "application/json")
		req.Header.Add("Accept", "application/vnd.urbanairship+json; version=3;")

		client := &http.Client{}
		resp, _ := client.Do(req)
		respBody, _ := ioutil.ReadAll(resp.Body)

		json.Unmarshal(respBody, &deviceTokenResp)
	}

	nextPage := "https://go.urbanairship.com/api/device_tokens/?"

	if config.UrbanAirship.TokensLimitPerRequest > 0 {
		nextPage += "&limit=" + strconv.Itoa(config.UrbanAirship.TokensLimitPerRequest)
	}
	if len(config.UrbanAirship.StartingTokenId) > 0 {
		nextPage += "&start=" + config.UrbanAirship.StartingTokenId
	}

	var allDeviceTokens = []UADeviceToken{}
	for len(nextPage) > 0 {
		var deviceTokens = []UADeviceToken{}

		var deviceTokenResp UADeviceTokensResponse
		GetDeviceTokensFromUrbanAirshipWithURL(nextPage, &deviceTokenResp)

		if activeTokensCount == 0 {
			activeTokensCount = deviceTokenResp.ActiveDeviceTokensCount
		}
		if tokensCount == 0 {
			tokensCount = deviceTokenResp.DeviceTokensCount
		}

		deviceTokens = append(deviceTokens, deviceTokenResp.DeviceTokens...)
		dir := dumpDir + "/urbanairship/" + strconv.Itoa(len(allDeviceTokens))
		os.MkdirAll(dir, 0744)
		txt, _ := json.MarshalIndent(deviceTokenResp, "", "\t")
		ioutil.WriteFile(dir+"/device_tokens.json", txt, 0644)

		if len(deviceTokenResp.NextPage) > 0 {
			nextPage = deviceTokenResp.NextPage
		} else {
			nextPage = ""
		}

		allDeviceTokens = append(allDeviceTokens, deviceTokens...)
		downloadedTokensCount = float64(len(allDeviceTokens))

		for _, deviceToken := range deviceTokens {
			pending <- deviceToken
		}
	}

	txt, _ := json.MarshalIndent(allDeviceTokens, "", "\t")
	ioutil.WriteFile(dumpDir+"/urbanairship.json", txt, 0644)

	pending <- UADeviceToken{}
	done <- true
}

func PostDeviceTokensToPushWoosh(pending chan UADeviceToken, done chan bool, status chan<- State) {
	for deviceToken := range pending {
		if len(deviceToken.DeviceToken) == 0 {
			close(pending)
			break
		}

		if debug {
			fmt.Println("Attempting to register device with token: " + deviceToken.DeviceToken)
		}

		PostDeviceTokenToPushWoosh := func(registerDevice PWRegisterDevice, deviceRegisterResp *PWDeviceRegisterResponse) {
			jsonBody, _ := json.Marshal(registerDevice)

			maxRetries := 10
			for i := 0; i < maxRetries; i++ {
				body := strings.NewReader(string(jsonBody))
				req, _ := http.NewRequest("POST", "https://cp.pushwoosh.com/json/1.3/registerDevice", body)
				req.Header.Add("Content-type", "application/json")
				req.Header.Add("Accept", "application/json")

				client := &http.Client{}

				resp, err := client.Do(req)
				if err != nil {
					fmt.Println("\nHTTP error:", err)
				} else if resp != nil && resp.StatusCode == 200 {
					defer resp.Body.Close()
					respBody, _ := ioutil.ReadAll(resp.Body)
					json.Unmarshal(respBody, &deviceRegisterResp)

					break
				}
				fmt.Println("Retrying HTTP request...")
				time.Sleep(10 * time.Second)
			}

		}

		if deviceToken.Active {
			registerDevice := PWRegisterDevice{
				Request: PWRegisterDeviceRequest{
					Auth:        config.PushWoosh.ApiKey,
					Application: config.PushWoosh.AppCode,
					DeviceType:  config.PushWoosh.DefaultDeviceType,
					Language:    config.PushWoosh.DefaultLanguage,
					Timezone:    config.PushWoosh.DefaultTimezone,
					Hwid:        deviceToken.DeviceToken, // UrbanAirship does not provide UDID. Use the token instead.
					PushToken:   deviceToken.DeviceToken,
				},
			}

			var deviceRegisterResp PWDeviceRegisterResponse
			PostDeviceTokenToPushWoosh(registerDevice, &deviceRegisterResp)
			if deviceRegisterResp.StatusCode != 200 || deviceRegisterResp.StatusMessage != "OK" {
				r, _ := json.MarshalIndent(deviceRegisterResp, "", "\t")
				fmt.Println("\nFailed to register device with token:")
				fmt.Println("\n\t" + registerDevice.Request.PushToken)
				fmt.Println("\nResponse from PushWoosh:\n")
				os.Stdout.Write(r)
				close(done)
			} else {
				if debug {
					r, _ := json.MarshalIndent(deviceRegisterResp, "", "\t")
					os.Stdout.Write(r)
					fmt.Println()
				}
			}
			status <- State{"SENT", deviceToken.DeviceToken}
		} else {
			status <- State{"INACTIVE", deviceToken.DeviceToken}
		}

		if debug {
			fmt.Println("Device registration complete.")
		}
	}
	done <- true
}

func StateMonitor(updateInterval time.Duration) chan<- State {
	updates := make(chan State)
	tokenStatus := make(map[string][]string)
	ticker := time.NewTicker(updateInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Print("\r")
				var downloadProgress float64 = 0
				if downloadedTokensCount > 0 {
					downloadProgress = downloadedTokensCount / tokensCount * 100
				}
				fmt.Printf("%.1f%% imported (%g of %g total tokens)", downloadProgress, downloadedTokensCount, tokensCount)
				var uploadProgress float64 = 0
				if len(tokenStatus["SENT"]) > 0 {
					uploadProgress = float64(len(tokenStatus["SENT"])) / activeTokensCount * 100
				}
				fmt.Print(" --- ")
				fmt.Printf("%.1f%% exported (%d of %g active tokens) ", uploadProgress, len(tokenStatus["SENT"]), activeTokensCount)
			case t := <-updates:
				tokenStatus[t.Status] = append(tokenStatus[t.Status], t.DeviceToken)

				file, _ := os.OpenFile(dumpDir+"/pushwoosh.json", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				defer file.Close()
				jsonStr, _ := json.MarshalIndent(t, "", "\t")
				file.WriteString(string(jsonStr) + ", ")
			}
		}
	}()
	return updates
}

func main() {
	dumpDir = "./dump/" + strconv.FormatInt(time.Now().Unix(), 10)

	pending := make(chan UADeviceToken)
	status := StateMonitor(1 * time.Millisecond)
	done := make(chan bool)

	go GetDeviceTokensFromUrbanAirship(pending, done)

	go PostDeviceTokensToPushWoosh(pending, done, status)
	go PostDeviceTokensToPushWoosh(pending, done, status)
	go PostDeviceTokensToPushWoosh(pending, done, status)

	<-done

	<-done
	<-done
	<-done

	fmt.Println("All done. Bye.")
}
